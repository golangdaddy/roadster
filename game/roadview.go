package game

import (
	"fmt"
	"image/color"
	"log"

	"github.com/golangdaddy/roadster/car"
	"github.com/golangdaddy/roadster/models"
	carmodel "github.com/golangdaddy/roadster/models/car"
	"github.com/golangdaddy/roadster/road"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// RoadView represents the main driving view
type RoadView struct {
	gameState *models.GameState
	road      *road.Road
	carModel  *carmodel.Car // Car model with weight, brakes, etc.

	// Car position and state (in world coordinates)
	carX     float64 // X position in world (horizontal - free movement)
	carY     float64 // Y position in world (vertical - distance traveled)
	carAngle float64 // Car angle in degrees (0 = facing up/north)
	carSpeed float64 // Current speed in pixels per frame
	
	// Distance tracking
	totalDistance float64 // Total distance traveled in pixels

	// Camera - fixed above car, doesn't rotate
	cameraX float64 // Camera X position in world space (centered on car)
	cameraY float64 // Camera Y position in world space (follows car)
	
	// Callback for returning to garage
	onReturnToGarage func()
}

// NewRoadView creates a new road view with the selected car
func NewRoadView(gameState *models.GameState, selectedCar *carmodel.Car, onReturnToGarage func()) *RoadView {
	// Load road from level file
	// Each segment is as long as the window height (600 pixels)
	segmentHeight := 600.0
	laneWidth := 80.0

	highway, err := road.LoadRoadFromFile("levels/1.level", segmentHeight, laneWidth)
	if err != nil {
		log.Printf("Failed to load road: %v", err)
		// Create a default road if loading fails
		highway = &road.Road{
			Segments:      []road.RoadSegment{{NumLanes: 3, StartY: 0, EndY: segmentHeight}},
			LaneWidth:     laneWidth,
			SegmentHeight: segmentHeight,
		}
	}

	// Car starts in center of lane 0
	// Lane 0 starts at world X = 0, so center of lane 0 is at X = LaneWidth/2
	
	// Use the selected car, or create a default if none provided
	carModel := selectedCar
	if carModel == nil {
		// Fallback to default car
		carModel = carmodel.NewCar("Default", "Sedan", 2024)
		carModel.Weight = 1500.0
		carModel.Brakes.StoppingPower = 1.0
		carModel.Brakes.Condition = 1.0
		carModel.Brakes.Performance = 1.0
	}

	return &RoadView{
		gameState:        gameState,
		road:             highway,
		carModel:         carModel,
		carX:             laneWidth / 2,   // Start in center of lane 0 (world X = LaneWidth/2)
		carY:             0,   // Start at beginning of road (world Y = 0)
		carAngle:         0,   // Facing straight up
		carSpeed:         0,   // Stationary
		cameraX:          laneWidth / 2,   // Camera starts at car position
		cameraY:          0,
		onReturnToGarage: onReturnToGarage,
	}
}

// Update handles input and updates game state
func (rv *RoadView) Update() error {
	// Check for Escape key to return to garage
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if rv.onReturnToGarage != nil {
			rv.onReturnToGarage()
		}
		return nil
	}
	
	// Handle car movement
	acceleration := 0.15      // Slower acceleration for realism
	turnSpeed := 3.0
	friction := 0.05          // Natural friction/drag (much slower deceleration)
	
	// Speed limit system: Lane 1 (index 0) = 60 mph, each additional lane = +10 mph
	// Current maxSpeed (8.0 px/frame) = 60 mph, so 1 mph = 8.0/60 = 0.133 px/frame
	baseSpeedLimitMPH := 60.0 // Lane 1 speed limit
	speedPerLaneMPH := 10.0   // Additional speed per lane
	pxPerFramePerMPH := 8.0 / 60.0 // Conversion: 60 mph = 8.0 px/frame
	
	// Calculate which lane the car is in (0-indexed, where 0 = Lane 1)
	// Lane 0 starts at X=0, so carX / LaneWidth gives us the lane index
	currentLane := int(rv.carX / rv.road.LaneWidth)
	if currentLane < 0 {
		currentLane = 0
	}
	
	// Get current road segment to know how many lanes are available
	currentSegment := rv.road.GetSegmentAtY(rv.carY)
	if currentSegment == nil {
		// Fallback if no segment found
		currentSegment = &road.RoadSegment{NumLanes: 3}
	}
	
	// Clamp lane to available lanes
	if currentLane >= currentSegment.NumLanes {
		currentLane = currentSegment.NumLanes - 1
	}
	
	// Calculate speed limit for current lane (Lane 1 = 60 mph, Lane 2 = 70 mph, etc.)
	// Lane index 0 = Lane 1 = 60 mph, Lane index 1 = Lane 2 = 70 mph
	speedLimitMPH := baseSpeedLimitMPH + (float64(currentLane) * speedPerLaneMPH)
	speedLimitPxPerFrame := speedLimitMPH * pxPerFramePerMPH
	
	// Debug: log lane info occasionally (every 60 frames = ~1 second at 60fps)
	// This helps verify the system is working
	if int(rv.totalDistance) % 60 == 0 {
		log.Printf("Lane: %d, carX: %.1f, speedLimit: %.1f mph (%.2f px/frame), currentSpeed: %.2f px/frame",
			currentLane+1, rv.carX, speedLimitMPH, speedLimitPxPerFrame, rv.carSpeed)
	}

	// Automatic speed adjustment to match lane speed limit
	// Always apply this when car is within lane bounds, regardless of input
	// Check if car is within the bounds of its current lane
	laneLeftBound := float64(currentLane) * rv.road.LaneWidth
	laneRightBound := float64(currentLane+1) * rv.road.LaneWidth
	
	// Calculate speed difference to target
	speedDifference := speedLimitPxPerFrame - rv.carSpeed
	autoAccelRate := acceleration * 0.02 // Automatic acceleration rate
	autoDecelRate := friction * 0.2      // Automatic deceleration rate
	
	// Apply automatic speed adjustment if within lane bounds
	if rv.carX >= laneLeftBound && rv.carX < laneRightBound {
		if speedDifference > 0.01 {
			// Need to speed up - apply automatic acceleration
			rv.carSpeed += autoAccelRate
			if rv.carSpeed > speedLimitPxPerFrame {
				rv.carSpeed = speedLimitPxPerFrame
			}
		} else if speedDifference < -0.01 {
			// Need to slow down - apply automatic deceleration
			rv.carSpeed -= autoDecelRate
			if rv.carSpeed < speedLimitPxPerFrame {
				rv.carSpeed = speedLimitPxPerFrame
			}
		}
	}
	
	// Manual acceleration forward (user input)
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		rv.carSpeed += acceleration
		// Cap at speed limit for current lane
		if rv.carSpeed > speedLimitPxPerFrame {
			rv.carSpeed = speedLimitPxPerFrame
		}
	}
	
	// Brake (down button) - use car's realistic brake deceleration method
	// This calculates brake force based on car weight and braking efficiency
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		// Get brake coefficient from car model (based on weight and brake efficiency)
		if rv.carSpeed > 0 {
			// Get realistic brake coefficient from car model
			brakeCoefficient := rv.carModel.GetBrakeDeceleration(rv.carSpeed)
			// Apply brake force proportional to current speed
			// new_speed = current_speed - (brake_coefficient * current_speed)
			// This creates exponential decay, which is realistic for braking
			brakeDeceleration := brakeCoefficient * rv.carSpeed
			rv.carSpeed -= brakeDeceleration
			if rv.carSpeed < 0 {
				rv.carSpeed = 0
			}
		}
		// Don't allow reverse - brake only stops forward motion
	}

	// Natural deceleration (friction/drag) - only when no input and not at speed limit
	if !ebiten.IsKeyPressed(ebiten.KeyArrowUp) && !ebiten.IsKeyPressed(ebiten.KeyW) &&
		!ebiten.IsKeyPressed(ebiten.KeyArrowDown) && !ebiten.IsKeyPressed(ebiten.KeyS) {
		// Only apply friction if we're above the speed limit (let automatic adjustment handle it otherwise)
		if rv.carSpeed > speedLimitPxPerFrame {
			rv.carSpeed -= friction
			if rv.carSpeed < speedLimitPxPerFrame {
				rv.carSpeed = speedLimitPxPerFrame
			}
		} else if rv.carSpeed > 0 && (rv.carX < laneLeftBound || rv.carX >= laneRightBound) {
			// Only apply friction if outside lane bounds
			rv.carSpeed -= friction
			if rv.carSpeed < 0 {
				rv.carSpeed = 0
			}
		}
	}

	// Car movement - left/right movement independent of lanes
	// Car moves freely left/right in world coordinates
	horizontalSpeed := turnSpeed
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		rv.carX += horizontalSpeed // Move right (increasing X)
		rv.carAngle = -5 // Tilt left
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		rv.carX -= horizontalSpeed // Move left (decreasing X)
		rv.carAngle = 5 // Tilt right
	} else {
		// No horizontal input - return to straight
		if rv.carAngle > 0 {
			rv.carAngle -= 2.0
			if rv.carAngle < 0 {
				rv.carAngle = 0
			}
		} else if rv.carAngle < 0 {
			rv.carAngle += 2.0
			if rv.carAngle > 0 {
				rv.carAngle = 0
			}
		}
	}

	// Update car Y position (distance traveled upward)
	// Car moves upward, so we increase carY (positive Y is up in world space)
	rv.carY += rv.carSpeed
	rv.totalDistance += rv.carSpeed // Track total distance traveled

	// Update camera to follow car - camera stays fixed above car position
	// Camera doesn't rotate, just follows the car's world position
	// Camera X and Y track the car's world position
	rv.cameraX = rv.carX // Camera X follows car's X position
	rv.cameraY = rv.carY // Camera Y follows car's Y position

	return nil
}

// Draw renders the road view
func (rv *RoadView) Draw(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Draw road (road scrolls in both X and Y as car moves)
	rv.road.Draw(screen, rv.cameraX, rv.cameraY)

	// Draw car - car is always centered on screen (camera follows car)
	carScreenX := float64(width) / 2  // Car always centered horizontally
	carScreenY := float64(height) / 2 // Car always centered vertically
	carColor := color.RGBA{100, 150, 255, 255} // Blue car

	car.RenderCar(screen, carScreenX, carScreenY, rv.carAngle, carColor)

	// Draw control labels
	rv.drawControlLabels(screen, width, height)
	
	// Draw speedometer and distance
	rv.drawSpeedometer(screen, width, height)
	
	// Draw detailed car stats breakdown
	rv.drawCarDetails(screen, width, height)
}

// drawControlLabels draws labels showing which way is forward, backward, left, and right
func (rv *RoadView) drawControlLabels(screen *ebiten.Image, width, height int) {
	face := text.NewGoXFace(bitmapfont.Face)
	labelColor := color.RGBA{200, 200, 200, 200} // Semi-transparent gray

	// Forward (top of screen - car moves upward)
	forwardText := "FORWARD (↑/W)"
	textWidth := text.Advance(forwardText, face)
	textX := float64(width)/2 - textWidth/2
	textY := 30.0
	forwardOp := &text.DrawOptions{}
	forwardOp.GeoM.Translate(textX, textY)
	forwardOp.ColorScale.ScaleWithColor(labelColor)
	text.Draw(screen, forwardText, face, forwardOp)

	// Brake (bottom of screen)
	brakeText := "BRAKE (↓/S)"
	textWidth = text.Advance(brakeText, face)
	textX = float64(width)/2 - textWidth/2
	textY = float64(height) - 30.0
	brakeOp := &text.DrawOptions{}
	brakeOp.GeoM.Translate(textX, textY)
	brakeOp.ColorScale.ScaleWithColor(labelColor)
	text.Draw(screen, brakeText, face, brakeOp)

	// Note: Coordinate system is inverted - negative world X appears on right side of screen
	// A/ArrowLeft increases world X → car appears on LEFT side of screen
	// D/ArrowRight decreases world X → car appears on RIGHT side of screen

	// Left side of screen (A/ArrowLeft moves car here)
	leftText := "LEFT (←/A)"
	textWidth = text.Advance(leftText, face)
	textX = 20.0
	textY = float64(height)/2 - 8
	leftOp := &text.DrawOptions{}
	leftOp.GeoM.Translate(textX, textY)
	leftOp.ColorScale.ScaleWithColor(labelColor)
	text.Draw(screen, leftText, face, leftOp)

	// Right side of screen (D/ArrowRight moves car here)
	rightText := "RIGHT (→/D)"
	textWidth = text.Advance(rightText, face)
	textX = float64(width) - textWidth - 20.0
	textY = float64(height)/2 - 8
	rightOp := &text.DrawOptions{}
	rightOp.GeoM.Translate(textX, textY)
	rightOp.ColorScale.ScaleWithColor(labelColor)
	text.Draw(screen, rightText, face, rightOp)
}

// drawSpeedometer draws a digital speedometer showing speed, distance, and car stats
func (rv *RoadView) drawSpeedometer(screen *ebiten.Image, width, height int) {
	face := text.NewGoXFace(bitmapfont.Face)
	
	// Speed in pixels per frame, convert to a more readable unit
	// Assuming 60 FPS, speed in pixels/second = speed * 60
	speedPxPerSec := rv.carSpeed * 60.0
	
	// Format speed (show 1 decimal place)
	speedText := fmt.Sprintf("SPEED: %.1f px/s", speedPxPerSec)
	
	// Format distance (show 1 decimal place)
	distanceText := fmt.Sprintf("DISTANCE: %.1f px", rv.totalDistance)
	
	// Car stats
	brakeEfficiency := rv.carModel.Brakes.Condition * rv.carModel.Brakes.Performance * rv.carModel.Brakes.StoppingPower
	carStatsText := fmt.Sprintf("CAR: %s %s | Weight: %.0f kg | Brake Eff: %.1f%%",
		rv.carModel.Make, rv.carModel.Model, rv.carModel.Weight, brakeEfficiency*100)
	
	// Calculate current lane and speed limit for display
	currentLane := int(rv.carX / rv.road.LaneWidth)
	if currentLane < 0 {
		currentLane = 0
	}
	currentSegment := rv.road.GetSegmentAtY(rv.carY)
	if currentSegment == nil {
		currentSegment = &road.RoadSegment{NumLanes: 3}
	}
	if currentLane >= currentSegment.NumLanes {
		currentLane = currentSegment.NumLanes - 1
	}
	baseSpeedLimitMPH := 60.0
	speedPerLaneMPH := 10.0
	speedLimitMPH := baseSpeedLimitMPH + (float64(currentLane) * speedPerLaneMPH)
	speedLimitText := fmt.Sprintf("LANE: %d | LIMIT: %.0f mph", currentLane+1, speedLimitMPH)
	
	// Draw speedometer in top-right corner
	speedColor := color.RGBA{0, 255, 0, 255} // Green for digital display
	speedWidth := text.Advance(speedText, face)
	speedX := float64(width) - speedWidth - 20.0
	speedY := 30.0
	
	speedOp := &text.DrawOptions{}
	speedOp.GeoM.Translate(speedX, speedY)
	speedOp.ColorScale.ScaleWithColor(speedColor)
	text.Draw(screen, speedText, face, speedOp)
	
	// Draw distance below speed
	distanceWidth := text.Advance(distanceText, face)
	distanceX := float64(width) - distanceWidth - 20.0
	distanceY := 60.0
	
	distanceOp := &text.DrawOptions{}
	distanceOp.GeoM.Translate(distanceX, distanceY)
	distanceOp.ColorScale.ScaleWithColor(speedColor)
	text.Draw(screen, distanceText, face, distanceOp)
	
	// Draw car stats below distance
	carStatsWidth := text.Advance(carStatsText, face)
	carStatsX := float64(width) - carStatsWidth - 20.0
	carStatsY := 90.0
	
	carStatsOp := &text.DrawOptions{}
	carStatsOp.GeoM.Translate(carStatsX, carStatsY)
	carStatsOp.ColorScale.ScaleWithColor(speedColor)
	text.Draw(screen, carStatsText, face, carStatsOp)
	
	// Draw speed limit below car stats
	speedLimitWidth := text.Advance(speedLimitText, face)
	speedLimitX := float64(width) - speedLimitWidth - 20.0
	speedLimitY := 120.0
	
	speedLimitOp := &text.DrawOptions{}
	speedLimitOp.GeoM.Translate(speedLimitX, speedLimitY)
	speedLimitOp.ColorScale.ScaleWithColor(speedColor)
	text.Draw(screen, speedLimitText, face, speedLimitOp)
}

// drawCarDetails draws a detailed breakdown of car stats on the left side of the screen
func (rv *RoadView) drawCarDetails(screen *ebiten.Image, width, height int) {
	face := text.NewGoXFace(bitmapfont.Face)
	textColor := color.RGBA{0, 255, 0, 255} // Green for digital display
	headerColor := color.RGBA{255, 255, 0, 255} // Yellow for headers
	
	startX := 20.0
	startY := 30.0
	lineHeight := 18.0
	currentY := startY
	
	// Calculate brake force at current speed
	currentBrakeCoefficient := rv.carModel.GetBrakeDeceleration(rv.carSpeed)
	brakeForce := currentBrakeCoefficient * rv.carSpeed * 60.0 // Convert to px/s
	
	// Calculate brake efficiency
	brakeEfficiency := rv.carModel.Brakes.Condition * rv.carModel.Brakes.Performance * rv.carModel.Brakes.StoppingPower
	
	// Calculate overall performance
	overallPerformance := rv.carModel.GetOverallPerformance()
	
	// Calculate weight factor for braking
	baseWeight := 1500.0
	weightFactor := baseWeight / rv.carModel.Weight
	if weightFactor > 1.5 {
		weightFactor = 1.5
	}
	if weightFactor < 0.5 {
		weightFactor = 0.5
	}
	
	// Header
	headerText := fmt.Sprintf("=== %s %s (%d) ===", rv.carModel.Make, rv.carModel.Model, rv.carModel.Year)
	drawTextAt(screen, headerText, startX, currentY, 16, headerColor, face)
	currentY += lineHeight * 1.5
	
	// Physical Attributes
	drawTextAt(screen, "--- PHYSICAL ---", startX, currentY, 14, headerColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Weight: %.0f kg", rv.carModel.Weight), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Length: %.2f m", rv.carModel.Length), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Width: %.2f m", rv.carModel.Width), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Height: %.2f m", rv.carModel.Height), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Seats: %d", rv.carModel.Seats), startX, currentY, 12, textColor, face)
	currentY += lineHeight * 1.5
	
	// Engine Details
	drawTextAt(screen, "--- ENGINE ---", startX, currentY, 14, headerColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Horsepower: %d HP", rv.carModel.Engine.Horsepower), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Torque: %d lb-ft", rv.carModel.Engine.Torque), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Displacement: %.1f L", rv.carModel.Engine.Displacement), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Fuel Efficiency: %.1f MPG", rv.carModel.Engine.FuelEfficiency), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Engine Condition: %.1f%%", rv.carModel.Engine.Condition*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Engine Performance: %.1f%%", rv.carModel.Engine.Performance*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight * 1.5
	
	// Brake Details
	drawTextAt(screen, "--- BRAKES ---", startX, currentY, 14, headerColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Type: %s", rv.carModel.Brakes.Type), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Condition: %.1f%%", rv.carModel.Brakes.Condition*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Performance: %.1f%%", rv.carModel.Brakes.Performance*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Stopping Power: %.2f", rv.carModel.Brakes.StoppingPower), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Brake Efficiency: %.1f%%", brakeEfficiency*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Weight Factor: %.2f", weightFactor), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Brake Coefficient: %.4f", currentBrakeCoefficient), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Brake Force: %.1f px/s", brakeForce), startX, currentY, 12, textColor, face)
	currentY += lineHeight * 1.5
	
	// Performance Summary
	drawTextAt(screen, "--- PERFORMANCE ---", startX, currentY, 14, headerColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Overall: %.1f%%", overallPerformance*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Top Speed: %.0f km/h", rv.carModel.TopSpeed), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("0-100 km/h: %.1f s", rv.carModel.Acceleration), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Handling: %.1f%%", rv.carModel.Handling*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Stability: %.1f%%", rv.carModel.Stability*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Car Condition: %.1f%%", rv.carModel.Condition*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight * 1.5
	
	// Additional Info
	drawTextAt(screen, "--- OTHER ---", startX, currentY, 14, headerColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Transmission: %s", rv.carModel.Transmission), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Drive Type: %s", rv.carModel.DriveType), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Fuel Level: %.1f%%", rv.carModel.FuelLevel*100), startX, currentY, 12, textColor, face)
	currentY += lineHeight
	drawTextAt(screen, fmt.Sprintf("Mileage: %.1f km", rv.carModel.Mileage), startX, currentY, 12, textColor, face)
}

// drawTextAt draws text at a specific position (helper function)
func drawTextAt(screen *ebiten.Image, str string, x, y float64, size float64, clr color.Color, face text.Face) {
	scale := size / 16.0
	scaledHeight := 16.0 * scale
	
	textX := x
	textY := y - scaledHeight/2 + 8
	
	op := &text.DrawOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(textX/scale, textY/scale)
	op.ColorScale.ScaleWithColor(clr)
	
	text.Draw(screen, str, face, op)
}
