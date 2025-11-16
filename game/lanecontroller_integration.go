package game

import (
	"image/color"
	"math/rand"

	"github.com/golangdaddy/roadster/lanecontroller"
	"github.com/golangdaddy/roadster/road"
	"github.com/hajimehoshi/ebiten/v2"
)

// updateLaneControllerSprites checks each next road segment and updates sprite loops for lane controllers
func (rv *RoadView) updateLaneControllerSprites() {
	// Get current segment and next segment
	currentSegment := rv.road.GetSegmentAtY(rv.carY)
	if currentSegment == nil {
		return
	}

	// Find next segment (segment that starts after current segment ends)
	var nextSegment *road.RoadSegment
	for i := range rv.road.Segments {
		if rv.road.Segments[i].StartY > currentSegment.EndY {
			nextSegment = &rv.road.Segments[i]
			break
		}
	}

	// Update lane controllers based on current and next segments
	// Each lane controller independently determines its sprite based on segment properties
	for _, lc := range rv.laneControllers {
		// Check if this lane controller is in the current or next segment
		if lc.WorldYStart >= currentSegment.StartY && lc.WorldYEnd <= currentSegment.EndY {
			// Lane controller is in current segment - let it determine its own sprite type
			spriteType := lc.GetSpriteTypeForSegment(currentSegment.HasPetrolStationLane, currentSegment.TileType)
			lc.UpdateSprite(spriteType)
		} else if nextSegment != nil && lc.WorldYStart >= nextSegment.StartY && lc.WorldYEnd <= nextSegment.EndY {
			// Lane controller is in next segment - let it determine its own sprite type
			spriteType := lc.GetSpriteTypeForSegment(nextSegment.HasPetrolStationLane, nextSegment.TileType)
			lc.UpdateSprite(spriteType)
		}
	}
}

// updateLaneControllerTraffic updates all traffic cars in lane controllers
func (rv *RoadView) updateLaneControllerTraffic(baseSpeedLimitMPH, speedPerLaneMPH, pxPerFramePerMPH float64) {
	for _, lc := range rv.laneControllers {
		lc.UpdateTrafficCars(baseSpeedLimitMPH, speedPerLaneMPH, pxPerFramePerMPH, rv.road.LaneWidth)
	}
}

// drawLaneControllers draws all lane controllers
func (rv *RoadView) drawLaneControllers(screen *ebiten.Image, width, height int) {
	if len(rv.laneControllers) == 0 {
		// Fallback: use old road drawing if no lane controllers
		rv.road.Draw(screen, rv.cameraX, rv.cameraY)
		return
	}

	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2

	// Calculate visible world Y range
	worldYStart := rv.cameraY - float64(height)/2 - 100
	worldYEnd := rv.cameraY + float64(height)/2 + 100

	for _, lc := range rv.laneControllers {
		// Check if lane controller is visible
		if !lc.IsVisible(worldYStart, worldYEnd) {
			continue
		}

		// Calculate screen position for this lane
		// World X increases to the right, screen X increases to the right
		// Formula: screenX = screenCenterX - (worldX - cameraX)
		worldX := lc.WorldX
		screenX := screenCenterX - (worldX - rv.cameraX)

		// Calculate screen Y for segment start and end
		// World Y increases upward (forward), screen Y increases downward
		// Formula: screenY = screenCenterY - (worldY - cameraY)
		// StartY is behind (lower world Y), EndY is ahead (higher world Y)
		// On screen: StartY should be at bottom (higher screen Y), EndY should be at top (lower screen Y)
		segmentStartScreenY := screenCenterY - (lc.WorldYStart - rv.cameraY)
		segmentEndScreenY := screenCenterY - (lc.WorldYEnd - rv.cameraY)

		// segmentStartScreenY should be > segmentEndScreenY (start is below end on screen)
		// If not, swap them
		if segmentStartScreenY < segmentEndScreenY {
			segmentStartScreenY, segmentEndScreenY = segmentEndScreenY, segmentStartScreenY
		}

		// Clamp to screen bounds
		drawStartY := segmentStartScreenY
		if drawStartY < 0 {
			drawStartY = 0
		}
		if drawStartY > float64(height) {
			drawStartY = float64(height)
		}
		
		drawEndY := segmentEndScreenY
		if drawEndY < 0 {
			drawEndY = 0
		}
		if drawEndY > float64(height) {
			drawEndY = float64(height)
		}

		// Only draw if there's a valid range (start > end on screen)
		if drawStartY <= drawEndY {
			continue
		}

		// Tile vertically to fill segment height (drawing from bottom to top)
		if lc.SpriteTile != nil {
			tileHeight := float64(lc.SpriteTile.Bounds().Dy())
			// Draw from bottom (drawStartY) upward to top (drawEndY)
			currentY := drawStartY
			for currentY > drawEndY {
				// Draw tile at currentY, but ensure we don't go below drawEndY
				drawY := currentY - tileHeight
				if drawY < drawEndY {
					drawY = drawEndY
				}
				lc.Draw(screen, screenX, drawY, rv.road.LaneWidth)
				currentY = drawY
				if currentY <= drawEndY {
					break
				}
			}
		}
	}
}

// drawLaneControllerTraffic draws all traffic cars from lane controllers
func (rv *RoadView) drawLaneControllerTraffic(screen *ebiten.Image, width, height int) {
	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2

	// Draw traffic cars from all lane controllers
	for _, lc := range rv.laneControllers {
		for _, tc := range lc.GetTrafficCars() {
			// Convert world coordinates to screen coordinates
			// World X increases to the right, screen X increases to the right
			// Formula: screenX = screenCenterX - (worldX - cameraX)
			carScreenX := screenCenterX - (tc.X - rv.cameraX)

			// World Y increases upward (forward), screen Y increases downward
			// Formula: screenY = screenCenterY - (worldY - cameraY)
			carScreenY := screenCenterY - (tc.Y - rv.cameraY)

			// Only draw if on screen
			if carScreenX >= -50 && carScreenX <= float64(width)+50 &&
				carScreenY >= -50 && carScreenY <= float64(height)+50 {
				// Draw traffic car (simple rectangle for now)
				carWidth := 30.0
				carHeight := 50.0
				carRect := ebiten.NewImage(int(carWidth), int(carHeight))
				carRect.Fill(tc.Color)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(carScreenX-carWidth/2, carScreenY-carHeight/2)
				screen.DrawImage(carRect, op)
			}
		}
	}
}

// spawnTrafficForLaneController spawns a traffic car in a lane controller
func (rv *RoadView) spawnTrafficForLaneController(lc *lanecontroller.LaneController, worldY float64, direction string) bool {
	// Check if there's enough space in the lane
	spacing := 300.0 // Minimum spacing between cars
	minSpacing := spacing * (0.8 + rand.Float64()*0.4) // Random variation: 80% to 120%

	var closestCarY float64 = -1
	for _, tc := range lc.TrafficCars {
		if direction == "ahead" {
			if tc.Y >= worldY && (closestCarY < 0 || tc.Y < closestCarY) {
				closestCarY = tc.Y
			}
		} else {
			if tc.Y <= worldY && (closestCarY < 0 || tc.Y > closestCarY) {
				closestCarY = tc.Y
			}
		}
	}

	if closestCarY >= 0 {
		distance := closestCarY - worldY
		if direction == "ahead" {
			distance = -distance
		}
		if distance < minSpacing {
			return false // Not enough space
		}
	}

	// Spawn traffic car
	carColors := []color.Color{
		color.RGBA{255, 100, 100, 255}, // Red
		color.RGBA{100, 255, 100, 255}, // Green
		color.RGBA{100, 100, 255, 255}, // Blue
		color.RGBA{255, 255, 100, 255}, // Yellow
		color.RGBA{255, 100, 255, 255}, // Magenta
	}
	carColor := carColors[rand.Intn(len(carColors))]

	initialFuelLevel := 0.5 + rand.Float64()*0.5 // 50% to 100% fuel

	trafficCar := lanecontroller.TrafficCar{
		X:            lc.WorldX,
		Y:            worldY,
		Lane:         lc.LaneIndex,
		Speed:        0, // Will be set by UpdateTrafficCars
		Color:        carColor,
		ID:           rv.nextCarID,
		FuelLevel:    initialFuelLevel,
		FuelCapacity: 50.0, // Default 50 liters for traffic cars
	}
	rv.nextCarID++

	lc.AddTrafficCar(trafficCar)
	return true
}

