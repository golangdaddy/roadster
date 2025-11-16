package game

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/golangdaddy/roadster/car"
	"github.com/golangdaddy/roadster/models"
	carmodel "github.com/golangdaddy/roadster/models/car"
	"github.com/golangdaddy/roadster/road"
	"github.com/hajimehoshi/bitmapfont/v4"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// TrafficCar represents a traffic vehicle on the road
type TrafficCar struct {
	X     float64     // World X position (center of lane)
	Y     float64     // World Y position
	Lane  int         // Lane index (0-based)
	Speed float64     // Speed in pixels per frame
	Color color.Color // Car color
}

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

	// Speed transition tracking
	transitionStartY        float64 // Y position when speed transition started
	transitionStartSpeed    float64 // Speed when transition started
	transitionTargetSpeed   float64 // Target speed for current transition
	transitionSegmentLength float64 // Length of one road segment (600 pixels)
	previousLane            int     // Track previous lane to detect lane changes

	// Traffic cars
	trafficCars []TrafficCar // All traffic cars on the road

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

	rv := &RoadView{
		gameState:               gameState,
		road:                    highway,
		carModel:                carModel,
		carX:                    laneWidth / 2, // Start in center of lane 0 (world X = LaneWidth/2)
		carY:                    0,             // Start at beginning of road (world Y = 0)
		carAngle:                0,             // Facing straight up
		carSpeed:                0,             // Stationary
		cameraX:                 laneWidth / 2, // Camera starts at car position
		cameraY:                 0,
		transitionStartY:        -1, // Sentinel value - no transition active
		transitionStartSpeed:    0,
		transitionTargetSpeed:   0,
		transitionSegmentLength: segmentHeight, // One road segment = 600 pixels
		previousLane:            0,             // Start in lane 0
		trafficCars:             []TrafficCar{},
		onReturnToGarage:        onReturnToGarage,
	}

	return rv
}

// getFurthestCarAheadInLane returns the Y position of the furthest car ahead in the lane
// Returns -1 if no cars exist in the lane
func (rv *RoadView) getFurthestCarAheadInLane(lane int, fromY float64) float64 {
	furthestY := -1.0
	for _, tc := range rv.trafficCars {
		if tc.Lane == lane && tc.Y >= fromY {
			if furthestY < 0 || tc.Y > furthestY {
				furthestY = tc.Y
			}
		}
	}
	return furthestY
}

// getClosestCarBehindInLane returns the Y position of the closest car behind in the lane
// Returns -1 if no cars exist in the lane behind the given Y
func (rv *RoadView) getClosestCarBehindInLane(lane int, fromY float64) float64 {
	closestY := -1.0
	for _, tc := range rv.trafficCars {
		if tc.Lane == lane && tc.Y < fromY {
			if closestY < 0 || tc.Y > closestY {
				closestY = tc.Y
			}
		}
	}
	return closestY
}

// hasCarTooCloseInLane checks if there's a car too close to the given Y position in the lane
func (rv *RoadView) hasCarTooCloseInLane(lane int, checkY float64, minSpacing float64) bool {
	for _, tc := range rv.trafficCars {
		if tc.Lane == lane {
			// Check distance in both directions (cars ahead and behind)
			distance := checkY - tc.Y
			if distance < 0 {
				distance = -distance // Absolute distance
			}
			if distance < minSpacing {
				return true // Too close to existing car
			}
		}
	}
	return false
}

// spawnTrafficForLane spawns a single traffic car for a lane if there's enough space
// direction: "ahead" spawns cars ahead of player, "behind" spawns cars behind player
// Returns true if a car was spawned, false otherwise
func (rv *RoadView) spawnTrafficForLane(segment road.RoadSegment, lane int, direction string) bool {
	// Traffic car colors (variety for visual distinction)
	colors := []color.Color{
		color.RGBA{200, 50, 50, 255},   // Red
		color.RGBA{50, 200, 50, 255},   // Green
		color.RGBA{200, 200, 50, 255},  // Yellow
		color.RGBA{200, 150, 50, 255},  // Orange
		color.RGBA{150, 150, 200, 255}, // Light blue
		color.RGBA{150, 50, 150, 255},  // Purple
	}

	// Minimum spacing: half a screen length (300 pixels)
	height := 600.0
	minSpacing := height / 2.0 // Half screen length

	// Don't spawn in player's immediate view (avoid spawning directly visible)
	visibleMargin := height/2.0 + 50.0 // Half screen + margin

	var minSpawnY, maxSpawnY float64
	var nextSpawnY float64

	if direction == "ahead" {
		// Spawn ahead of player
		spawnAheadDistance := height * 2.0     // Spawn up to 2 screen heights ahead
		minSpawnY = rv.cameraY + visibleMargin // Don't spawn in visible area
		maxSpawnY = rv.cameraY + spawnAheadDistance

		// Clamp spawn range to segment bounds
		if minSpawnY < segment.StartY {
			minSpawnY = segment.StartY
		}
		if maxSpawnY > segment.EndY {
			maxSpawnY = segment.EndY
		}

		if minSpawnY >= maxSpawnY {
			// No valid spawn range
			return false
		}

		// Find the furthest car ahead in this lane within the segment
		furthestCarY := rv.getFurthestCarAheadInLane(lane, segment.StartY)

		if furthestCarY < 0 {
			// No cars in this lane yet - spawn at the start of the spawn range
			nextSpawnY = minSpawnY
		} else {
			// Spawn ahead of the furthest car
			nextSpawnY = furthestCarY + minSpacing

			// Make sure it's within the spawn range
			if nextSpawnY < minSpawnY {
				nextSpawnY = minSpawnY
			}
			if nextSpawnY > maxSpawnY {
				// Too far ahead, don't spawn yet
				return false
			}
		}
	} else {
		// Spawn behind player
		spawnBehindDistance := height * 2.0 // Spawn up to 2 screen heights behind
		minSpawnY = rv.cameraY - spawnBehindDistance
		maxSpawnY = rv.cameraY - visibleMargin // Don't spawn in visible area

		// Clamp spawn range to segment bounds
		if minSpawnY < segment.StartY {
			minSpawnY = segment.StartY
		}
		if maxSpawnY > segment.EndY {
			maxSpawnY = segment.EndY
		}

		if minSpawnY >= maxSpawnY {
			// No valid spawn range
			return false
		}

		// Find the closest car behind in this lane within the segment
		closestCarY := rv.getClosestCarBehindInLane(lane, segment.EndY)

		if closestCarY < 0 {
			// No cars in this lane yet - spawn at the end of the spawn range (furthest behind)
			nextSpawnY = maxSpawnY
		} else {
			// Spawn behind the closest car (further back)
			nextSpawnY = closestCarY - minSpacing

			// Make sure it's within the spawn range
			if nextSpawnY > maxSpawnY {
				nextSpawnY = maxSpawnY
			}
			if nextSpawnY < minSpawnY {
				// Too far behind, don't spawn yet
				return false
			}
		}
	}

	// Double-check: ensure there's no car too close at the spawn position
	if rv.hasCarTooCloseInLane(lane, nextSpawnY, minSpacing) {
		// Too close to existing car, don't spawn
		return false
	}

	// Calculate X position (center of lane) - ensure coordinates are correct
	carX := float64(lane)*rv.road.LaneWidth + rv.road.LaneWidth/2

	// Random color
	colorIndex := rand.Intn(len(colors))

	// Create traffic car with correct coordinates
	trafficCar := TrafficCar{
		X:     carX,
		Y:     nextSpawnY,
		Lane:  lane,
		Speed: 0, // Will be set based on lane speed limit in Update
		Color: colors[colorIndex],
	}

	rv.trafficCars = append(rv.trafficCars, trafficCar)

	return true
}

// spawnTrafficForVisibleSegments spawns traffic cars for visible road segments
// Each lane generates traffic independently and gradually, both ahead and behind the player
// Only spawns a new car if there's at least half a screen length of space
func (rv *RoadView) spawnTrafficForVisibleSegments() {
	height := 600.0               // Window height
	spawnDistance := height * 2.0 // Spawn traffic up to 2 screen heights ahead/behind

	// Calculate spawn ranges for both directions
	worldYAheadStart := rv.cameraY
	worldYAheadEnd := rv.cameraY + spawnDistance
	worldYBehindStart := rv.cameraY - spawnDistance
	worldYBehindEnd := rv.cameraY

	// Check each segment for visibility (both ahead and behind)
	for _, segment := range rv.road.Segments {
		// Check if segment is visible ahead of player
		segmentAheadVisible := !(segment.EndY < worldYAheadStart || segment.StartY > worldYAheadEnd)
		// Check if segment is visible behind player
		segmentBehindVisible := !(segment.EndY < worldYBehindStart || segment.StartY > worldYBehindEnd)

		if !segmentAheadVisible && !segmentBehindVisible {
			continue
		}

		// Handle each lane independently
		// Each lane spawns cars gradually, one at a time, with proper spacing
		for lane := 0; lane < segment.NumLanes; lane++ {
			// Try to spawn one car ahead for this lane (will only spawn if there's enough space)
			if segmentAheadVisible {
				rv.spawnTrafficForLane(segment, lane, "ahead")
			}

			// Try to spawn one car behind for this lane (will only spawn if there's enough space)
			if segmentBehindVisible {
				rv.spawnTrafficForLane(segment, lane, "behind")
			}
		}
	}

	// Clean up traffic cars that are far behind or ahead of the player (to prevent memory issues)
	cleanupDistance := height * 3.5 // Remove cars more than 3.5 screen heights away
	minY := rv.cameraY - cleanupDistance
	maxY := rv.cameraY + cleanupDistance
	var activeTraffic []TrafficCar
	for _, tc := range rv.trafficCars {
		if tc.Y >= minY && tc.Y <= maxY {
			activeTraffic = append(activeTraffic, tc)
		}
	}
	rv.trafficCars = activeTraffic
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
	acceleration := 0.15 // Slower acceleration for realism
	turnSpeed := 3.0
	friction := 0.05 // Natural friction/drag (much slower deceleration)

	// Speed limit system: Lane 1 (index 0) = 60 mph, each additional lane = +10 mph
	// Current maxSpeed (8.0 px/frame) = 60 mph, so 1 mph = 8.0/60 = 0.133 px/frame
	baseSpeedLimitMPH := 60.0      // Lane 1 speed limit
	speedPerLaneMPH := 10.0        // Additional speed per lane
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

	// Check if player is braking (used to pause cruise control)
	isBraking := ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS)

	// Only trigger speed transition when the car actually changes lanes
	// Not just by being in a lane - only when moving from one lane to another
	laneChanged := currentLane != rv.previousLane

	// If braking starts, clear any active transition (player takes control)
	if isBraking && rv.transitionStartY >= 0 {
		rv.transitionStartY = -1 // Clear transition
	}

	if laneChanged {
		// Car has moved to a different lane - start speed transition
		// Only start if not braking (brake pauses cruise control)
		if !isBraking {
			rv.transitionStartY = rv.carY
			rv.transitionStartSpeed = rv.carSpeed
			rv.transitionTargetSpeed = speedLimitPxPerFrame
		}
		// Update previous lane
		rv.previousLane = currentLane
	}

	// Check if we're in an active deceleration transition (needed to block acceleration)
	isDeceleratingTransition := rv.transitionStartY >= 0 && rv.transitionStartSpeed != rv.transitionTargetSpeed &&
		rv.transitionTargetSpeed < rv.transitionStartSpeed

	// Manual acceleration forward (user input)
	// Player controls acceleration - can accelerate up to speed limit
	// BUT: Don't allow acceleration during deceleration transition (it would fight the smooth deceleration)
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		// Only allow acceleration if NOT in a deceleration transition
		if !isDeceleratingTransition {
			rv.carSpeed += acceleration
			// Cap at speed limit for current lane (player can't exceed limit)
			if rv.carSpeed > speedLimitPxPerFrame {
				rv.carSpeed = speedLimitPxPerFrame
			}
		}
	}

	// Brake (down button) - use car's realistic brake deceleration method
	// This calculates brake force based on car weight and braking efficiency
	// Brake ALWAYS works and can slow car below speed limit - player has full control
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
		// Note: Brake can slow car below speed limit - player has full control
	}

	// Natural deceleration (friction/drag) - only when no input
	// Don't apply friction during active cruise control transition (let transition handle it)
	isInTransition := rv.transitionStartY >= 0 && rv.transitionStartSpeed != rv.transitionTargetSpeed

	if !isInTransition {
		if !ebiten.IsKeyPressed(ebiten.KeyArrowUp) && !ebiten.IsKeyPressed(ebiten.KeyW) &&
			!ebiten.IsKeyPressed(ebiten.KeyArrowDown) && !ebiten.IsKeyPressed(ebiten.KeyS) {
			// Apply friction when no input
			if rv.carSpeed > 0 {
				rv.carSpeed -= friction
				if rv.carSpeed < 0 {
					rv.carSpeed = 0
				}
			}
		}
	}

	// Enforce speed limit as maximum (player can't exceed it, but can be below it)
	// BUT: Don't enforce if we're in an active transition (transition handles speed)
	// Brake allows player to slow below speed limit
	isInTransition = rv.transitionStartY >= 0 && rv.transitionStartSpeed != rv.transitionTargetSpeed
	if !isInTransition && rv.carSpeed > speedLimitPxPerFrame {
		rv.carSpeed = speedLimitPxPerFrame
	}

	// Final enforcement of cruise control transition (only when actively transitioning after lane change)
	// This MUST happen after all other speed modifications to ensure smooth transition is never overridden
	// IMPORTANT: Only apply if not braking - braking gives player full control
	if !isBraking && rv.transitionStartY >= 0 && rv.transitionStartSpeed != rv.transitionTargetSpeed {
		// Calculate transition progress
		distanceTraveled := rv.carY - rv.transitionStartY
		transitionProgress := distanceTraveled / rv.transitionSegmentLength
		if transitionProgress < 0.0 {
			transitionProgress = 0.0
		}
		if transitionProgress > 1.0 {
			transitionProgress = 1.0
		}
		targetTransitionSpeed := rv.transitionStartSpeed + (rv.transitionTargetSpeed-rv.transitionStartSpeed)*transitionProgress

		// Enforce transition speed exactly during lane change (cruise control)
		// This ensures smooth tweening for both acceleration and deceleration
		rv.carSpeed = targetTransitionSpeed
	}

	// Car movement - left/right movement independent of lanes
	// Car moves freely left/right in world coordinates
	horizontalSpeed := turnSpeed
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		rv.carX += horizontalSpeed // Move right (increasing X)
		rv.carAngle = -5           // Tilt left
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		rv.carX -= horizontalSpeed // Move left (decreasing X)
		rv.carAngle = 5            // Tilt right
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

	// Spawn traffic for visible segments (dynamic spawning)
	rv.spawnTrafficForVisibleSegments()

	// Update traffic cars
	rv.updateTrafficCars(baseSpeedLimitMPH, speedPerLaneMPH, pxPerFramePerMPH)

	// Check for collisions with traffic cars
	if rv.checkCollisionWithTraffic() {
		// Collision detected - restart the game
		rv.restart()
		return nil
	}

	// Update camera to follow car - camera stays fixed above car position
	// Camera doesn't rotate, just follows the car's world position
	// Camera X and Y track the car's world position
	rv.cameraX = rv.carX // Camera X follows car's X position
	rv.cameraY = rv.carY // Camera Y follows car's Y position

	return nil
}

// checkCollisionWithTraffic checks if the player car collides with any traffic car
// Returns true if collision detected
func (rv *RoadView) checkCollisionWithTraffic() bool {
	// Car dimensions (from car/render.go)
	carWidth := 30.0
	carHeight := 50.0

	// Player car bounding box (centered at carX, carY)
	playerLeft := rv.carX - carWidth/2
	playerRight := rv.carX + carWidth/2
	playerTop := rv.carY - carHeight/2
	playerBottom := rv.carY + carHeight/2

	// Check collision with each traffic car
	for _, tc := range rv.trafficCars {
		// Traffic car bounding box (centered at tc.X, tc.Y)
		trafficLeft := tc.X - carWidth/2
		trafficRight := tc.X + carWidth/2
		trafficTop := tc.Y - carHeight/2
		trafficBottom := tc.Y + carHeight/2

		// Check if bounding boxes overlap
		if playerLeft < trafficRight && playerRight > trafficLeft &&
			playerTop < trafficBottom && playerBottom > trafficTop {
			// Collision detected
			return true
		}
	}

	return false
}

// restart resets the game to initial state
func (rv *RoadView) restart() {
	// Reset car position and state
	laneWidth := rv.road.LaneWidth
	rv.carX = laneWidth / 2 // Center of lane 0
	rv.carY = 0             // Start at beginning of road
	rv.carAngle = 0         // Facing straight up
	rv.carSpeed = 0         // Stationary
	rv.cameraX = laneWidth / 2
	rv.cameraY = 0
	rv.totalDistance = 0

	// Reset speed transition
	rv.transitionStartY = -1
	rv.transitionStartSpeed = 0
	rv.transitionTargetSpeed = 0
	rv.previousLane = 0

	// Clear all traffic cars
	rv.trafficCars = []TrafficCar{}
}

// updateTrafficCars updates all traffic cars to move at 5mph less than their lane speed limits
// Removes cars when their lane disappears instead of collapsing them into lower lanes
func (rv *RoadView) updateTrafficCars(baseSpeedLimitMPH, speedPerLaneMPH, pxPerFramePerMPH float64) {
	var activeTraffic []TrafficCar

	for i := range rv.trafficCars {
		tc := rv.trafficCars[i]

		// Get the road segment this traffic car is in
		segment := rv.road.GetSegmentAtY(tc.Y)
		if segment == nil {
			// No segment found, remove this car
			continue
		}

		// If the car's lane no longer exists, remove it instead of moving it
		if tc.Lane >= segment.NumLanes {
			// Lane disappeared, remove this car
			continue
		}
		if tc.Lane < 0 {
			// Invalid lane, remove this car
			continue
		}

		// Calculate speed limit for this lane
		speedLimitMPH := baseSpeedLimitMPH + (float64(tc.Lane) * speedPerLaneMPH)

		// Traffic cars move 5mph slower than the lane speed limit (more challenging)
		trafficSpeedMPH := speedLimitMPH - 5.0
		// Ensure speed doesn't go below 0
		if trafficSpeedMPH < 0 {
			trafficSpeedMPH = 0
		}
		trafficSpeedPxPerFrame := trafficSpeedMPH * pxPerFramePerMPH

		// Set traffic car speed
		tc.Speed = trafficSpeedPxPerFrame

		// Update traffic car position (moves upward like player car)
		tc.Y += tc.Speed

		// Keep traffic car centered in its lane
		tc.X = float64(tc.Lane)*rv.road.LaneWidth + rv.road.LaneWidth/2

		// Add to active traffic list
		activeTraffic = append(activeTraffic, tc)
	}

	// Replace traffic cars list with only active ones
	rv.trafficCars = activeTraffic
}

// Draw renders the road view
func (rv *RoadView) Draw(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Draw countryside background first (grass)
	rv.drawCountrysideBackground(screen, width, height)

	// Draw road (road scrolls in both X and Y as car moves)
	rv.road.Draw(screen, rv.cameraX, rv.cameraY)

	// Draw countryside elements (trees, water) on top of road but below traffic
	rv.drawCountrysideElements(screen, width, height)

	// Draw traffic cars
	rv.drawTrafficCars(screen, width, height)

	// Draw car - car is always centered on screen (camera follows car)
	carScreenX := float64(width) / 2           // Car always centered horizontally
	carScreenY := float64(height) / 2          // Car always centered vertically
	carColor := color.RGBA{100, 150, 255, 255} // Blue car

	car.RenderCar(screen, carScreenX, carScreenY, rv.carAngle, carColor)

	// Draw control labels
	rv.drawControlLabels(screen, width, height)

	// Draw speedometer and distance
	rv.drawSpeedometer(screen, width, height)

	// Draw detailed car stats breakdown
	rv.drawCarDetails(screen, width, height)
}

// drawTrafficCars renders all traffic cars on screen
func (rv *RoadView) drawTrafficCars(screen *ebiten.Image, width, height int) {
	// Convert world coordinates to screen coordinates
	// Match the coordinate system used by the road drawing:
	// - X: screenX = screenCenterX - (worldX - cameraX)
	// - Y: screenY = screenCenterY - (worldY - cameraY) [inverted because world Y increases upward, screen Y increases downward]
	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2

	for _, tc := range rv.trafficCars {
		// Convert world coordinates to screen coordinates (matching road coordinate system)
		screenX := screenCenterX - (tc.X - rv.cameraX)
		screenY := screenCenterY - (tc.Y - rv.cameraY)

		// Only draw if traffic car is visible on screen (with some margin)
		margin := 100.0
		if screenX >= -margin && screenX <= float64(width)+margin &&
			screenY >= -margin && screenY <= float64(height)+margin {
			// Render traffic car (facing straight up, no angle)
			car.RenderCar(screen, screenX, screenY, 0, tc.Color)
		}
	}
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
	textColor := color.RGBA{0, 255, 0, 255}     // Green for digital display
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

// drawCountrysideBackground draws a simple grass background - optimized for performance
func (rv *RoadView) drawCountrysideBackground(screen *ebiten.Image, width, height int) {
	// Simple grass green color - just fill the screen
	grassColor := color.RGBA{60, 179, 113, 255} // Medium sea green - proper grass green
	screen.Fill(grassColor)
}

// hashFloat generates a deterministic pseudo-random float from a seed
// Uses a more stable hash function to prevent glitching
func hashFloat(seed int64) float64 {
	// More stable hash function using multiple rounds
	hash := uint64(seed)
	// Multiple rounds for better distribution
	for i := 0; i < 3; i++ {
		hash ^= hash >> 33
		hash *= 0xff51afd7ed558ccd
		hash ^= hash >> 33
		hash *= 0xc4ceb9fe1a85ec53
		hash ^= hash >> 33
	}
	// Convert to float in range [0, 1) using only positive bits
	return float64(hash&0x7FFFFFFF) / float64(0x7FFFFFFF)
}

// hashInt generates a deterministic pseudo-random int from a seed
func hashInt(seed int64, max int) int {
	if max <= 0 {
		return 0
	}
	return int(hashFloat(seed) * float64(max))
}

// drawTree draws a simple tree at the given screen coordinates - optimized
func drawTree(screen *ebiten.Image, x, y float64, treeType int64) {
	// Tree type determines size variation
	baseSize := 1.0 + hashFloat(treeType)*0.3 // 1.0 to 1.3x size (reduced variation for performance)

	// Trunk (brown rectangle)
	trunkColor := color.RGBA{101, 67, 33, 255} // Brown
	trunkWidth := 8.0 * baseSize
	trunkHeight := 20.0 * baseSize

	// Create trunk image
	trunkImg := ebiten.NewImage(int(trunkWidth), int(trunkHeight))
	trunkImg.Fill(trunkColor)
	trunkOp := &ebiten.DrawImageOptions{}
	trunkOp.GeoM.Translate(x-trunkWidth/2, y-trunkHeight)
	screen.DrawImage(trunkImg, trunkOp)

	// Simplified foliage - single circle for performance
	foliageSize := 25.0 * baseSize
	foliageColor := color.RGBA{0, 120, 0, 255} // Dark green

	foliageImg := ebiten.NewImage(int(foliageSize), int(foliageSize))
	foliageImg.Fill(foliageColor)
	foliageOp := &ebiten.DrawImageOptions{}
	foliageOp.GeoM.Translate(x-foliageSize/2, y-trunkHeight-foliageSize/2)
	screen.DrawImage(foliageImg, foliageOp)
}

// drawField draws a simple crop field - optimized
func drawField(screen *ebiten.Image, x, y, width, height float64, fieldType int64) {
	// Field base color (dirt/soil with green tint for crops)
	fieldColor := color.RGBA{100, 120, 60, 255} // Brown-green mix for crops

	// Draw field base (simplified - no individual rows for performance)
	fieldImg := ebiten.NewImage(int(width), int(height))
	fieldImg.Fill(fieldColor)
	fieldOp := &ebiten.DrawImageOptions{}
	fieldOp.GeoM.Translate(x-width/2, y-height/2)
	screen.DrawImage(fieldImg, fieldOp)
}

// drawWater draws a simple water feature - optimized
func drawWater(screen *ebiten.Image, x, y, size float64, waterSeed int64) {
	// Water base color (deeper blue)
	waterColor := color.RGBA{0, 80, 150, 255}

	// Draw main water body (simplified for performance)
	waterSize := size * (0.9 + hashFloat(waterSeed)*0.2) // Vary size
	waterImg := ebiten.NewImage(int(waterSize), int(waterSize))
	waterImg.Fill(waterColor)
	waterOp := &ebiten.DrawImageOptions{}
	waterOp.GeoM.Translate(x-waterSize/2, y-waterSize/2)
	screen.DrawImage(waterImg, waterOp)
}

// drawCountrysideElements draws a rich countryside with trees, water, fields, and hills
// Uses quantized grid positions to prevent glitching
func (rv *RoadView) drawCountrysideElements(screen *ebiten.Image, width, height int) {
	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2

	// Calculate visible world Y range with extra margin
	worldYStart := rv.cameraY - float64(height)/2 - 300
	worldYEnd := rv.cameraY + float64(height)/2 + 300

	// Grid spacing for scenery elements
	gridSpacing := 120.0

	// Quantize world Y to grid positions to ensure consistent generation
	gridStartY := math.Floor(worldYStart/gridSpacing) * gridSpacing
	gridEndY := math.Ceil(worldYEnd/gridSpacing) * gridSpacing

	// Draw scenery elements in a quantized grid pattern
	for gridY := gridStartY; gridY <= gridEndY; gridY += gridSpacing {
		// Use quantized grid Y as seed for completely deterministic generation
		seedY := int64(gridY * 100.0)

		// Draw fields on left side (crop fields)
		fieldSeedLeft := seedY + 500000
		fieldChance := hashFloat(fieldSeedLeft)
		if fieldChance < 0.15 { // 15% chance of field
			fieldOffsetX := hashFloat(fieldSeedLeft+1)*80.0 - 40.0
			fieldOffsetY := hashFloat(fieldSeedLeft+2) * gridSpacing * 0.6
			fieldX := -200.0 + fieldOffsetX
			fieldY := gridY + fieldOffsetY

			// Convert to screen coordinates
			screenX := screenCenterX - (fieldX - rv.cameraX)
			screenY := screenCenterY - (fieldY - rv.cameraY)

			// Draw field if visible
			if screenX >= -150 && screenX <= float64(width)+150 &&
				screenY >= -150 && screenY <= float64(height)+150 {
				fieldWidth := 100.0 + hashFloat(fieldSeedLeft+3)*50.0
				fieldHeight := 80.0 + hashFloat(fieldSeedLeft+4)*40.0
				drawField(screen, screenX, screenY, fieldWidth, fieldHeight, fieldSeedLeft)
			}
		}

		// Draw fields on right side
		fieldSeedRight := seedY + 600000
		fieldChanceRight := hashFloat(fieldSeedRight)
		if fieldChanceRight < 0.15 {
			roadWidth := float64(rv.road.GetNumLanesAtY(gridY)) * rv.road.LaneWidth
			fieldOffsetX := hashFloat(fieldSeedRight+1)*80.0 - 40.0
			fieldOffsetY := hashFloat(fieldSeedRight+2) * gridSpacing * 0.6
			fieldX := roadWidth + 200.0 + fieldOffsetX
			fieldY := gridY + fieldOffsetY

			// Convert to screen coordinates
			screenX := screenCenterX - (fieldX - rv.cameraX)
			screenY := screenCenterY - (fieldY - rv.cameraY)

			// Draw field if visible
			if screenX >= -150 && screenX <= float64(width)+150 &&
				screenY >= -150 && screenY <= float64(height)+150 {
				fieldWidth := 100.0 + hashFloat(fieldSeedRight+3)*50.0
				fieldHeight := 80.0 + hashFloat(fieldSeedRight+4)*40.0
				drawField(screen, screenX, screenY, fieldWidth, fieldHeight, fieldSeedRight)
			}
		}

		// Check for water on left side
		waterSeedLeft := seedY + 1000000
		waterChance := hashFloat(waterSeedLeft)
		if waterChance < 0.10 { // 10% chance of water
			waterOffsetX := hashFloat(waterSeedLeft+1)*50.0 - 25.0
			waterOffsetY := hashFloat(waterSeedLeft+2) * gridSpacing * 0.7
			waterX := -180.0 + waterOffsetX
			waterY := gridY + waterOffsetY

			// Convert to screen coordinates
			screenX := screenCenterX - (waterX - rv.cameraX)
			screenY := screenCenterY - (waterY - rv.cameraY)

			// Draw water if visible
			if screenX >= -120 && screenX <= float64(width)+120 &&
				screenY >= -120 && screenY <= float64(height)+120 {
				waterSize := 70.0 + hashFloat(waterSeedLeft+3)*40.0
				drawWater(screen, screenX, screenY, waterSize, waterSeedLeft)
			}
		}

		// Check for water on right side
		waterSeedRight := seedY + 2000000
		waterChanceRight := hashFloat(waterSeedRight)
		if waterChanceRight < 0.10 {
			roadWidth := float64(rv.road.GetNumLanesAtY(gridY)) * rv.road.LaneWidth
			waterOffsetX := hashFloat(waterSeedRight+1)*50.0 - 25.0
			waterOffsetY := hashFloat(waterSeedRight+2) * gridSpacing * 0.7
			waterX := roadWidth + 180.0 + waterOffsetX
			waterY := gridY + waterOffsetY

			// Convert to screen coordinates
			screenX := screenCenterX - (waterX - rv.cameraX)
			screenY := screenCenterY - (waterY - rv.cameraY)

			// Draw water if visible
			if screenX >= -120 && screenX <= float64(width)+120 &&
				screenY >= -120 && screenY <= float64(height)+120 {
				waterSize := 70.0 + hashFloat(waterSeedRight+3)*40.0
				drawWater(screen, screenX, screenY, waterSize, waterSeedRight)
			}
		}

		// Draw trees on left side - reduced density for performance
		for treeIdx := 0; treeIdx < 2; treeIdx++ {
			treeSeedLeft := seedY + int64(3000000+treeIdx*100000)
			treeChance := hashFloat(treeSeedLeft)
			if treeChance < 0.25 { // 25% chance per tree slot (reduced from 30%)
				treeOffsetX := hashFloat(treeSeedLeft+10)*80.0 - 40.0
				treeOffsetY := hashFloat(treeSeedLeft+20) * gridSpacing * 0.9
				treeX := -140.0 + treeOffsetX
				treeY := gridY + treeOffsetY

				// Convert to screen coordinates
				screenX := screenCenterX - (treeX - rv.cameraX)
				screenY := screenCenterY - (treeY - rv.cameraY)

				// Draw tree if visible
				if screenX >= -60 && screenX <= float64(width)+60 &&
					screenY >= -60 && screenY <= float64(height)+60 {
					drawTree(screen, screenX, screenY, treeSeedLeft)
				}
			}
		}

		// Draw trees on right side
		for treeIdx := 0; treeIdx < 2; treeIdx++ {
			treeSeedRight := seedY + int64(4000000+treeIdx*100000)
			treeChance := hashFloat(treeSeedRight)
			if treeChance < 0.25 {
				roadWidth := float64(rv.road.GetNumLanesAtY(gridY)) * rv.road.LaneWidth
				treeOffsetX := hashFloat(treeSeedRight+10)*80.0 - 40.0
				treeOffsetY := hashFloat(treeSeedRight+20) * gridSpacing * 0.9
				treeX := roadWidth + 140.0 + treeOffsetX
				treeY := gridY + treeOffsetY

				// Convert to screen coordinates
				screenX := screenCenterX - (treeX - rv.cameraX)
				screenY := screenCenterY - (treeY - rv.cameraY)

				// Draw tree if visible
				if screenX >= -60 && screenX <= float64(width)+60 &&
					screenY >= -60 && screenY <= float64(height)+60 {
					drawTree(screen, screenX, screenY, treeSeedRight)
				}
			}
		}
	}
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
