package game

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/golangdaddy/roadster/pkg/models/car"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// MPHPerPixelPerFrame is the conversion factor from pixels per frame to MPH
// At 60 FPS, max speed of 10.4 pixels/frame (was 8)
// Setting max speed to 100 MPH gives us: 10.4 pixels/frame = 100 MPH
// Therefore: 1 pixel/frame = 9.6 MPH (adjusted to make car 30% faster)
const MPHPerPixelPerFrame = 9.6

// Traffic constants
const (
	minTrafficDistance     = 150.0 // Minimum distance between traffic vehicles in pixels
	trafficVariation       = 0.2   // 20% random variation on distance
	trafficSpawnRange      = 3000.0 // Range ahead/behind player to spawn traffic
	trafficSpawnProbability = 0.15 // Chance to spawn a car for a lane/direction (reduced to help hit speed limits)
)

// TrafficCar represents a traffic vehicle
type TrafficCar struct {
	X, Y         float64    // World position
	VelocityY    float64    // Vertical velocity (current speed)
	TargetSpeed  float64    // Desired speed (based on speed limit)
	Lane         int        // Which lane this car is in
	TargetLane   int        // Lane moving towards (if changing)
	LaneProgress float64    // 0.0 to 1.0 for visual transition
	Color        color.RGBA // Car color for variety
	stopAI       chan bool  // Channel to stop the AI behavior goroutine
}

// StartAI starts the behavior goroutine for this traffic car
func (tc *TrafficCar) StartAI(gs *GameplayScreen) {
	tc.stopAI = make(chan bool)
	go func() {
		// Check more frequently (30ms) for smoother reaction
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-tc.stopAI:
				return
			case <-ticker.C:
				tc.updateBehavior(gs)
			}
		}
	}()
}

// StopAI stops the behavior goroutine
func (tc *TrafficCar) StopAI() {
	if tc.stopAI != nil {
		tc.stopAI <- true
		close(tc.stopAI)
		tc.stopAI = nil
	}
}

// updateBehavior runs the AI logic to maintain safe distance
func (tc *TrafficCar) updateBehavior(gs *GameplayScreen) {
	// Safety check: skip AI updates if coordinates are extreme to prevent panics
	if math.Abs(tc.Y) > 100000 || math.Abs(gs.playerCar.Y) > 100000 {
		return
	}

	// Find closest car ahead in same lane
	minDist := 10000.0
	foundCarAhead := false
	speedOfCarAhead := 0.0
	
	rightLaneBlocked := false

	// Check against other traffic
	gs.trafficMutex.RLock()
	for _, other := range gs.traffic {
		if other == tc {
			continue
		}
		if other.Lane == tc.Lane {
			// Check if other car is ahead (Y is smaller)
			if other.Y < tc.Y {
				dist := tc.Y - other.Y
				if dist < minDist {
					minDist = dist
					foundCarAhead = true
					speedOfCarAhead = other.VelocityY
				}
			}
		}
		// Check if right lane is blocked (for lane change)
		// Check if car is in target lane or moving to it
		if other.Lane == tc.Lane+1 || (other.TargetLane == tc.Lane+1 && other.LaneProgress > 0) {
			if math.Abs(tc.Y-other.Y) < minTrafficDistance*2.0 {
				rightLaneBlocked = true
			}
		}
	}
	gs.trafficMutex.RUnlock()

	// Check against player
	// Simple lane check based on X distance
	laneWidth := 80.0
	if math.Abs(gs.playerCar.X - tc.X) < laneWidth/2 {
		// Player is in roughly the same lane
		if gs.playerCar.Y < tc.Y {
			dist := tc.Y - gs.playerCar.Y
			if dist < minDist {
				minDist = dist
				foundCarAhead = true
				speedOfCarAhead = gs.playerCar.VelocityY
			}
		}
	}
	// Check player in right lane
	// Assuming player X logic: lane 0 is around 40?
	// If player X is in right lane range
	// This is tricky without segment/lane math, but we can approximate or skip player check for lane change for now.

	// Adjust speed based on distance
	safeDistance := minTrafficDistance * 1.5
	if foundCarAhead && minDist < safeDistance {
		// Match the car ahead, with extra braking if extremely close
		targetSpeed := speedOfCarAhead
		if minDist < minTrafficDistance {
			targetSpeed = speedOfCarAhead * 0.85
		}
		if minDist < minTrafficDistance*0.5 {
			targetSpeed = speedOfCarAhead * 0.6
		}
		if targetSpeed < 0 {
			targetSpeed = 0
		}
		// Smoothly approach target speed
		tc.VelocityY += (targetSpeed - tc.VelocityY) * 0.5
		if tc.VelocityY < 0 {
			tc.VelocityY = 0
		}
	} else {
		// Quickly restore to lane speed limit
		tc.VelocityY += (tc.TargetSpeed - tc.VelocityY) * 0.4
	}
	
	// Attempt lane change to faster lane (right)
	if !rightLaneBlocked && tc.LaneProgress == 0 && tc.TargetLane == 0 {
		// 1% chance per tick to consider changing
		if rand.Float64() < 0.01 {
			segment := gs.getSegmentAt(tc.Y)
			if tc.Lane+1 < segment.LaneCount {
				tc.TargetLane = tc.Lane + 1
				tc.LaneProgress = 0.01 // Start transition
			}
		}
	}
}

// Car represents the player's car in the game world
type Car struct {
	X, Y             float64 // World position
	Speed            float64
	VelocityX        float64 // Horizontal velocity
	VelocityY        float64 // Vertical velocity (for forward movement)
	SteeringAngle    float64 // Current steering wheel angle (-1 to 1)
	Acceleration     float64 // Acceleration rate
	TurnSpeed        float64 // How fast the car turns
	SteeringResponse float64 // How quickly steering returns to center
	SelectedCar      *car.Car
	Sprite           *ebiten.Image
}

// GameplayScreen represents the main driving gameplay
type GameplayScreen struct {
	roadSegments []RoadSegment
	playerCar    *Car
	traffic      []*TrafficCar // Traffic vehicles
	trafficMutex sync.RWMutex  // Mutex for safe concurrent access to traffic
	scrollSpeed  float64
	roadTextures map[string]*ebiten.Image
	screenWidth  int
	screenHeight int
	cameraX      float64 // Camera X offset to follow car
	onGameEnd    func() // Callback when game ends
	levelData    *LevelData // Store level data for reset
	initialX     float64   // Initial player X position
	initialY     float64   // Initial player Y position
	backgroundPattern *ebiten.Image // Repeating background pattern
	lastSpawnTime int64 // Timestamp of last spawn attempt
	spawnCooldown int64 // Minimum time between spawn attempts (in milliseconds)
}

// NewGameplayScreen creates a new gameplay screen
func NewGameplayScreen(selectedCar *car.Car, levelData *LevelData, onGameEnd func()) *GameplayScreen {
	gs := &GameplayScreen{
		roadSegments: make([]RoadSegment, 0),
		traffic:      make([]*TrafficCar, 0),
		scrollSpeed:  2.0,
		roadTextures: make(map[string]*ebiten.Image),
		screenWidth:  1024,
		screenHeight: 600,
		onGameEnd:    onGameEnd,
		lastSpawnTime: time.Now().UnixMilli(),
		spawnCooldown: 150 + rand.Int63n(100), // 150-250ms random cooldown between spawn attempts
	}

	// Initialize player car
	laneWidth := 80.0
	initialX := laneWidth / 2 // Start in center of starting lane (world X = 40)
	initialY := float64(gs.screenHeight) - 100
	
	gs.playerCar = &Car{
		X:                initialX,
		Y:                initialY,
		Speed:            0,
		VelocityX:        0,
		VelocityY:        0,
		SteeringAngle:    0,
		Acceleration:     0.5,
		TurnSpeed:        3.0,  // Horizontal speed when turning
		SteeringResponse: 0.15, // How fast steering returns to center
		SelectedCar:      selectedCar,
	}

	// Store initial position and level data for reset
	gs.initialX = initialX
	gs.initialY = initialY
	gs.levelData = levelData

	// Load road textures
	gs.loadRoadTextures()

	// Generate repeating background pattern
	// gs.generateBackgroundPattern()

	// Generate road from level data
	gs.generateRoadFromLevel(levelData)

	// Spawn initial traffic
	gs.spawnInitialTraffic()

	return gs
}

// loadRoadTextures loads the road texture assets
func (gs *GameplayScreen) loadRoadTextures() {
	// Load road textures with correct letter mapping
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/A.png"); err == nil {
		gs.roadTextures["A"] = img
	}
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/B.png"); err == nil {
		gs.roadTextures["B"] = img
	}
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/C.png"); err == nil {
		gs.roadTextures["C"] = img
	}
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/D.png"); err == nil {
		gs.roadTextures["D"] = img
	}
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/E.png"); err == nil {
		gs.roadTextures["E"] = img
	}
	if img, _, err := ebitenutil.NewImageFromFile("assets/road/F.png"); err == nil {
		gs.roadTextures["F"] = img
	}
}

// generateRoadFromLevel creates road segments from level data
func (gs *GameplayScreen) generateRoadFromLevel(levelData *LevelData) {
	segmentHeight := 600.0 // Height of each road segment in world space (600px as specified)

	y := float64(gs.screenHeight) // Start from bottom of screen
	for i, segment := range levelData.Segments {
		// Start with only 1 lane for the first few segments
		laneCount := segment.LaneCount
		roadTypes := segment.RoadTypes
		lanePositions := segment.LanePositions
		
		startLaneIdx := segment.StartLaneIndex
		
		if i < 3 { // First 3 segments are always 1 lane
			laneCount = 1
			// Use only the starting lane's road type and position
			if startLaneIdx < len(roadTypes) {
				roadTypes = []string{roadTypes[startLaneIdx]}
			} else if len(roadTypes) > 0 {
				roadTypes = []string{roadTypes[0]}
			}
			// Preserve the lane position mapping for the starting lane
			if startLaneIdx < len(lanePositions) {
				lanePositions = []int{lanePositions[startLaneIdx]}
			} else if len(lanePositions) > 0 {
				lanePositions = []int{lanePositions[0]}
			}
			startLaneIdx = 0 // Starting lane is at index 0 when there's only 1 lane
		}

		roadSegment := RoadSegment{
			LaneCount:      laneCount,
			RoadTypes:      roadTypes,
			LanePositions:  lanePositions,
			StartLaneIndex: startLaneIdx,
			Y:              y,
		}
		gs.roadSegments = append(gs.roadSegments, roadSegment)
		y -= segmentHeight // Segments go upward
	}
}

// Update handles gameplay logic
func (gs *GameplayScreen) Update() error {
	currentSegment := gs.getCurrentRoadSegment()
	laneWidth := 80.0

	// Check for end of level
	if len(gs.roadSegments) > 0 {
		lastSegment := gs.roadSegments[len(gs.roadSegments)-1]
		// If player has reached the top of the last segment (finished the level)
		if gs.playerCar.Y <= lastSegment.Y {
			// Level completed! Clean up and call the end game callback
			gs.cleanupTraffic()
			if gs.onGameEnd != nil {
				gs.onGameEnd()
			}
			return nil
		}
	}

	// Handle steering input (Left/Right arrow keys)
	maxSteeringAngle := 1.0
	steeringInput := 0.08 // How fast steering angle changes per frame

	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		gs.playerCar.SteeringAngle -= steeringInput
		if gs.playerCar.SteeringAngle < -maxSteeringAngle {
			gs.playerCar.SteeringAngle = -maxSteeringAngle
		}
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		gs.playerCar.SteeringAngle += steeringInput
		if gs.playerCar.SteeringAngle > maxSteeringAngle {
			gs.playerCar.SteeringAngle = maxSteeringAngle
		}
	} else {
		// Return steering to center when no input
		if gs.playerCar.SteeringAngle > 0 {
			gs.playerCar.SteeringAngle -= gs.playerCar.SteeringResponse
			if gs.playerCar.SteeringAngle < 0 {
				gs.playerCar.SteeringAngle = 0
			}
		} else if gs.playerCar.SteeringAngle < 0 {
			gs.playerCar.SteeringAngle += gs.playerCar.SteeringResponse
			if gs.playerCar.SteeringAngle > 0 {
				gs.playerCar.SteeringAngle = 0
			}
		}
	}

	// Calculate current lane and speed limit
	currentLane := gs.getCurrentLane(currentSegment, laneWidth)
	speedLimitMPH := 50.0 + float64(currentLane)*10.0 // Lane 0 = 50, Lane 1 = 60, Lane 2 = 70, etc.
	maxSpeed := speedLimitMPH / MPHPerPixelPerFrame // Convert MPH to pixels/frame
	
	minSpeed := 0.0 // Allow stopping
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		gs.playerCar.VelocityY += gs.playerCar.Acceleration
		if gs.playerCar.VelocityY > maxSpeed {
			gs.playerCar.VelocityY = maxSpeed
		}
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 1.5
		if gs.playerCar.VelocityY < minSpeed {
			gs.playerCar.VelocityY = minSpeed
		}
	} else {
		// Gradually slow down when no input (friction)
		if gs.playerCar.VelocityY > 0 {
			gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 0.2
			if gs.playerCar.VelocityY < 0 {
				gs.playerCar.VelocityY = 0
			}
		}
	}
	
	// Enforce speed limit (in case car was already above limit when changing lanes)
	if gs.playerCar.VelocityY > maxSpeed {
		gs.playerCar.VelocityY = maxSpeed
	}

	// Apply steering to horizontal velocity
	// Steering effectiveness increases with speed
	// Use a reference max speed (100 MPH = 8 pixels/frame) for steering calculation
	referenceMaxSpeed := 100.0 / MPHPerPixelPerFrame // 8.0 pixels/frame
	speedFactor := gs.playerCar.VelocityY / referenceMaxSpeed
	gs.playerCar.VelocityX = gs.playerCar.SteeringAngle * gs.playerCar.TurnSpeed * speedFactor

	// Apply friction to horizontal movement (car naturally slows horizontal drift)
	gs.playerCar.VelocityX *= 0.95

	// Update car position based on velocity
	gs.playerCar.X += gs.playerCar.VelocityX

	// Clamp car position to stay within road bounds
	// Calculate the road boundaries based on current segment
	leftEdge := -float64(currentSegment.StartLaneIndex) * laneWidth
	rightEdge := leftEdge + float64(currentSegment.LaneCount)*laneWidth
	
	if gs.playerCar.X < leftEdge+10 {
		gs.playerCar.X = leftEdge + 10
		gs.playerCar.VelocityX = 0
	}
	if gs.playerCar.X > rightEdge-10 {
		gs.playerCar.X = rightEdge - 10
		gs.playerCar.VelocityX = 0
	}

	// Camera follows car smoothly
	targetCameraX := gs.playerCar.X - float64(gs.screenWidth)/2
	gs.cameraX += (targetCameraX - gs.cameraX) * 0.1

	// Scroll the road (move road downward to create forward movement illusion)
	scrollSpeed := gs.playerCar.VelocityY
	
	// Update car position (car moves upward toward top of screen)
	gs.playerCar.Y -= scrollSpeed
	
	// Update traffic
	gs.updateTraffic(scrollSpeed, currentSegment, laneWidth)

	// Check for collisions with traffic
	if gs.checkCollisions() {
		gs.resetToStart()
	}

	// Remove segments that have scrolled off screen
	// DISABLED: Removing segments causes issues with visibility at the edges.
	// Since we have a finite number of segments pre-generated, we can just keep them all.
	// The draw function handles culling off-screen segments efficiently.
	/*
	for i := 0; i < len(gs.roadSegments); i++ {
		segment := gs.roadSegments[i]
		screenY := segment.Y - gs.playerCar.Y + float64(gs.screenHeight)/2
		
		// Drawing code draws if: -600 <= screenY <= screenHeight
		// Only remove if completely off-screen with buffer
		if (screenY + 600) < -100 || screenY > float64(gs.screenHeight)+100 {
			gs.roadSegments = append(gs.roadSegments[:i], gs.roadSegments[i+1:]...)
			i-- // Adjust index after removal
		} else {
			// Segments are ordered, so once we find one that's still visible, we can stop
			break
		}
	}
	*/

	// All segments are pre-generated from level data, no dynamic addition needed

	return nil
}

// Draw renders the gameplay screen
func (gs *GameplayScreen) Draw(screen *ebiten.Image) {
	// Clear screen with sky color (retro blue) - ensure full coverage
	screen.Fill(color.RGBA{135, 206, 235, 255}) // Sky blue


	// Draw road segments
	gs.drawRoad(screen)

	// Draw traffic (behind player car)
	gs.drawTraffic(screen)

	// Draw player car
	gs.drawCar(screen)

	// Draw UI overlay
	gs.drawUI(screen)
}

// drawRoad renders all road segments
func (gs *GameplayScreen) drawRoad(screen *ebiten.Image) {
	for _, segment := range gs.roadSegments {
		gs.drawRoadSegment(screen, segment)
	}
}

// drawRoadSegment renders a single road segment
func (gs *GameplayScreen) drawRoadSegment(screen *ebiten.Image, segment RoadSegment) {
	// Calculate screen position (camera is above car)
	screenY := segment.Y - gs.playerCar.Y + float64(gs.screenHeight)/2

	// Safety check: skip if coordinates are extreme to prevent ebiten panics
	if math.Abs(screenY) > 100000 {
		return
	}

	// Skip if completely off screen (using 600px segment height)
	// Draw if any part of the segment is visible on screen
	segmentHeight := 600.0
	// Simple visibility check:
	// If top is below screen bottom (screenY > screenHeight) -> Skip
	// If bottom is above screen top (screenY + segmentHeight < 0) -> Skip
	if screenY > float64(gs.screenHeight) || screenY+segmentHeight < 0 {
		return
	}

	// Calculate road dimensions based on lanes
	laneWidth := 80.0 // Width of each lane in pixels
	
	// Position road so that the starting lane stays in the same world position
	// The starting lane (index segment.StartLaneIndex) should have its center at world X = 40
	// So the left edge of the starting lane is at world X = 0
	// The left edge of lane 0 is at: 0 - (segment.StartLaneIndex * laneWidth)
	roadX := -float64(segment.StartLaneIndex)*laneWidth - gs.cameraX

	// Draw decorative layer (repeating background pattern with trees and water)
	// gs.drawDecorativeLayer(screen, segment, screenY, roadX, laneWidth)

	// Draw each lane with its specific texture
	for laneIdx := 0; laneIdx < segment.LaneCount; laneIdx++ {
		// Get the road type for this lane
		var roadType string
		if laneIdx < len(segment.RoadTypes) {
			roadType = segment.RoadTypes[laneIdx]
		} else {
			roadType = "A" // Default fallback
		}

		// Get the texture for this lane
		texture, exists := gs.roadTextures[roadType]
		if !exists {
			texture = gs.roadTextures["A"] // Default to normal road
		}

		// Calculate lane position (laneX is the left edge of the lane)
		laneX := roadX + float64(laneIdx)*laneWidth

		if texture == nil {
			// Fallback: draw colored rectangle for this lane
			laneImg := ebiten.NewImage(int(laneWidth), 600)
			laneImg.Fill(color.RGBA{64, 64, 64, 255}) // Dark gray road

			// Draw lane marking on the right edge (except for last lane)
			if laneIdx < segment.LaneCount-1 {
				for y := 0; y < 600; y += 20 {
					// Draw dashed lines
					for dashY := 0; dashY < 10 && y+dashY < 600; dashY++ {
						laneImg.Set(int(laneWidth)-2, y+dashY, color.RGBA{255, 255, 255, 255})
						laneImg.Set(int(laneWidth)-1, y+dashY, color.RGBA{255, 255, 255, 255})
					}
				}
			}

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(laneX, screenY)
			screen.DrawImage(laneImg, op)
		} else {
			// Draw the texture for this lane
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(laneWidth/float64(texture.Bounds().Dx()), 600.0/float64(texture.Bounds().Dy()))
			op.GeoM.Translate(laneX, screenY)
			screen.DrawImage(texture, op)
		}
	}
}

// generateBackgroundPattern creates a seamless repeating countryside background
func (gs *GameplayScreen) generateBackgroundPattern() {
	patternHeight := 600 // Match segment height
	patternWidth := gs.screenWidth

	pattern := ebiten.NewImage(patternWidth, patternHeight)

	// Create a seamless countryside scene that tiles well vertically
	// We'll use a gradient from sky blue at top to grass green at bottom

	for y := 0; y < patternHeight; y++ {
		// Sky to grass gradient
		skyRatio := 1.0 - float64(y)/float64(patternHeight)
		grassRatio := float64(y) / float64(patternHeight)

		for x := 0; x < patternWidth; x++ {
			// Base colors
			skyColor := color.RGBA{135, 206, 235, 255}   // Sky blue
			grassColor := color.RGBA{34, 139, 34, 255}    // Grass green

			// Blend sky and grass colors
			r := uint8(float64(skyColor.R)*skyRatio + float64(grassColor.R)*grassRatio)
			g := uint8(float64(skyColor.G)*skyRatio + float64(grassColor.G)*grassRatio)
			b := uint8(float64(skyColor.B)*skyRatio + float64(grassColor.B)*grassRatio)

			baseColor := color.RGBA{r, g, b, 255}

			// Add texture variation based on position
			variation := (x*3 + y*7) % 100
			if variation < 10 {
				// Lighter patches
				baseColor.R = uint8(math.Min(255, float64(baseColor.R)+20))
				baseColor.G = uint8(math.Min(255, float64(baseColor.G)+30))
			} else if variation > 90 {
				// Darker patches
				baseColor.R = uint8(math.Max(0, float64(baseColor.R)-15))
				baseColor.G = uint8(math.Max(0, float64(baseColor.G)-20))
			}

			// Add some rolling hill effect in the middle section
			if y > patternHeight/4 && y < patternHeight*3/4 {
				hillEffect := 8 * math.Sin(float64(x)*0.01 + float64(y)*0.005)
				if hillEffect > 0 {
					// Darker green for hills
					baseColor.R = uint8(math.Max(0, float64(baseColor.R)-hillEffect*3))
					baseColor.G = uint8(math.Max(0, float64(baseColor.G)-hillEffect*2))
				}
			}

			pattern.Set(x, y, baseColor)
		}
	}

	// Add scattered details that repeat in a tileable way
	// Use modulo arithmetic to ensure seamless tiling

	// Wildflowers - positioned to tile seamlessly
	for i := 0; i < 30; i++ {
		// Position flowers using modulo to ensure they appear at same relative positions when tiled
		flowerX := (i * 41) % patternWidth
		// Flowers appear in the lower 2/3 of the pattern
		flowerY := patternHeight/3 + 20 + (i*29)%(patternHeight*2/3 - 40)

		// Flower stem (green, darker than grass)
		stemColor := color.RGBA{0, 80, 0, 255}
		for sy := 0; sy < 6; sy++ {
			stemY := flowerY + sy
			if stemY < patternHeight {
				pattern.Set(flowerX, stemY, stemColor)
			}
		}

		// Flower head (various colors)
		flowerColors := []color.RGBA{
			{255, 255, 0, 255},  // Yellow
			{255, 0, 255, 255},  // Magenta
			{255, 100, 0, 255},  // Orange
			{200, 0, 200, 255},  // Purple
			{255, 150, 0, 255},  // Dark orange
		}
		flowerColor := flowerColors[i%len(flowerColors)]

		// Simple cross-shaped flower
		pattern.Set(flowerX, flowerY-1, flowerColor)   // Top
		pattern.Set(flowerX-1, flowerY, flowerColor)   // Left
		pattern.Set(flowerX, flowerY, flowerColor)     // Center
		pattern.Set(flowerX+1, flowerY, flowerColor)   // Right
		pattern.Set(flowerX, flowerY+1, flowerColor)   // Bottom
	}

	// Small bushes/shrubs
	for i := 0; i < 20; i++ {
		bushX := (i * 53 + 23) % patternWidth
		bushY := patternHeight/2 + 10 + (i*37)%(patternHeight/2 - 30)

		bushColor := color.RGBA{25, 100, 25, 255} // Dark green

		// Draw small bush (2x2)
		for dy := -1; dy <= 0; dy++ {
			for dx := -1; dx <= 0; dx++ {
				bx := bushX + dx
				by := bushY + dy
				if bx >= 0 && bx < patternWidth && by >= 0 && by < patternHeight {
					pattern.Set(bx, by, bushColor)
				}
			}
		}
	}

	// Scattered pebbles/rocks for texture
	for i := 0; i < 50; i++ {
		rockX := (i * 31 + 13) % patternWidth
		rockY := patternHeight/3 + (i*43)%(patternHeight*2/3)

		rockColor := color.RGBA{100, 100, 100, 255}

		// Single pixel rocks, some double pixels
		pattern.Set(rockX, rockY, rockColor)
		if (i%4) == 0 && rockX+1 < patternWidth {
			pattern.Set(rockX+1, rockY, rockColor)
		}
		if (i%7) == 0 && rockY+1 < patternHeight {
			pattern.Set(rockX, rockY+1, rockColor)
		}
	}

	// Add some subtle cloud-like formations in the upper portion
	for i := 0; i < 8; i++ {
		cloudX := (i * 67 + 41) % patternWidth
		cloudY := 30 + (i*59)%100

		cloudColor := color.RGBA{160, 200, 240, 255} // Light blue-white

		// Small cloud puffs
		for dy := -2; dy <= 2; dy++ {
			for dx := -3; dx <= 3; dx++ {
				cx := cloudX + dx
				cy := cloudY + dy
				if cx >= 0 && cx < patternWidth && cy >= 0 && cy < patternHeight/3 {
					// Only draw if it's a "cloud" shape (circular-ish)
					dist := math.Sqrt(float64(dx*dx + dy*dy))
					if dist <= 2.5 {
						pattern.Set(cx, cy, cloudColor)
					}
				}
			}
		}
	}

	gs.backgroundPattern = pattern
}

// drawTreeToPattern draws a simplified tree to the pattern
func (gs *GameplayScreen) drawTreeToPattern(pattern *ebiten.Image, x, y float64, seed int) {
	treeWidth := 30
	treeHeight := 50
	
	screenX := int(x)
	screenY := int(y)
	
	// Skip if out of bounds
	if screenX < 0 || screenX+treeWidth > pattern.Bounds().Dx() || 
	   screenY < 0 || screenY+treeHeight > pattern.Bounds().Dy() {
		return
	}
	
	// Tree trunk (brown)
	trunkColor := color.RGBA{101, 67, 33, 255}
	trunkWidth := 6
	trunkHeight := 15
	trunkX := screenX + treeWidth/2 - trunkWidth/2
	trunkY := screenY + treeHeight - trunkHeight
	
	for ty := 0; ty < trunkHeight; ty++ {
		for tx := 0; tx < trunkWidth; tx++ {
			if trunkX+tx >= 0 && trunkX+tx < pattern.Bounds().Dx() &&
			   trunkY+ty >= 0 && trunkY+ty < pattern.Bounds().Dy() {
				pattern.Set(trunkX+tx, trunkY+ty, trunkColor)
			}
		}
	}
	
	// Tree foliage (simple circle)
	foliageColors := []color.RGBA{
		{34, 139, 34, 255},
		{0, 128, 0, 255},
		{50, 150, 50, 255},
		{20, 100, 20, 255},
	}
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	foliageColor := foliageColors[positiveSeed%len(foliageColors)]
	
	foliageCenterX := screenX + treeWidth/2
	foliageCenterY := screenY + treeHeight/2
	radius := 15
	
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= radius*radius {
				px := foliageCenterX + dx
				py := foliageCenterY + dy
				if px >= 0 && px < pattern.Bounds().Dx() && py >= 0 && py < pattern.Bounds().Dy() {
					pattern.Set(px, py, foliageColor)
				}
			}
		}
	}
}


// drawDecorativeLayer draws the repeating background pattern with trees positioned relative to road
func (gs *GameplayScreen) drawDecorativeLayer(screen *ebiten.Image, segment RoadSegment, screenY float64, roadX float64, laneWidth float64) {
	if gs.backgroundPattern == nil {
		return
	}
	
	segmentHeight := 600.0
	totalRoadWidth := float64(segment.LaneCount) * laneWidth
	leftGrassStart := roadX
	rightGrassEnd := roadX + totalRoadWidth
	
	// Calculate how many pattern tiles we need to cover the segment
	patternHeight := float64(gs.backgroundPattern.Bounds().Dy())
	numTiles := int(math.Ceil(segmentHeight / patternHeight)) + 1
	
	// Draw repeating pattern tiles (grass and water)
	for i := 0; i < numTiles; i++ {
		tileY := screenY + float64(i)*patternHeight
		
		// Only draw if on screen
		if tileY > float64(gs.screenHeight) || tileY < -patternHeight {
			continue
		}
		
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, tileY)
		screen.DrawImage(gs.backgroundPattern, op)
	}
	
	// Draw trees positioned relative to the road (not at screen edges)
	// Use segment Y as seed for consistent tree placement
	segmentYInt := int(segment.Y)
	if segmentYInt < 0 {
		segmentYInt = -segmentYInt
	}
	segmentSeed := segmentYInt % 1000
	
	// Draw trees on left side of road
	leftTreeX := leftGrassStart - 60.0
	if leftTreeX > -100 && leftTreeX < float64(gs.screenWidth)+100 {
		gs.drawTreesToScreen(screen, leftTreeX, screenY, segmentHeight, segmentSeed)
	}
	
	// Draw trees on right side of road
	rightTreeX := rightGrassEnd + 20.0
	if rightTreeX > -100 && rightTreeX < float64(gs.screenWidth)+100 {
		gs.drawTreesToScreen(screen, rightTreeX, screenY, segmentHeight, segmentSeed+500)
	}
}

// drawTreesToScreen draws trees at a specific X position along the segment
func (gs *GameplayScreen) drawTreesToScreen(screen *ebiten.Image, x float64, y float64, height float64, seed int) {
	treeSpacing := 150.0
	numTrees := int(height / treeSpacing) + 1
	
	// Ensure seed is positive
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	
	for i := 0; i < numTrees; i++ {
		treeY := y + float64(i)*treeSpacing + float64(positiveSeed%50)
		
		// Only draw if on screen
		if treeY > -50 && treeY < float64(gs.screenHeight)+50 {
			treeSeed := positiveSeed + i*17
			gs.drawTreeToScreen(screen, x, treeY, treeSeed)
		}
	}
}

// drawTreeToScreen draws a single tree directly to screen
func (gs *GameplayScreen) drawTreeToScreen(screen *ebiten.Image, x, y float64, seed int) {
	treeWidth := 30
	treeHeight := 50
	
	screenX := int(x)
	screenY := int(y)
	
	// Skip if completely off screen
	if screenX+treeWidth < 0 || screenX > gs.screenWidth || screenY+treeHeight < 0 || screenY > gs.screenHeight {
		return
	}
	
	// Create tree sprite
	treeImg := ebiten.NewImage(treeWidth, treeHeight)
	
	// Tree trunk (brown)
	trunkColor := color.RGBA{101, 67, 33, 255}
	trunkWidth := 6
	trunkHeight := 15
	trunkX := treeWidth/2 - trunkWidth/2
	trunkY := treeHeight - trunkHeight
	
	// Draw trunk
	for ty := 0; ty < trunkHeight; ty++ {
		for tx := 0; tx < trunkWidth; tx++ {
			treeImg.Set(trunkX+tx, trunkY+ty, trunkColor)
		}
	}
	
	// Tree foliage (green, varies by seed)
	foliageColors := []color.RGBA{
		{34, 139, 34, 255},
		{0, 128, 0, 255},
		{50, 150, 50, 255},
		{20, 100, 20, 255},
	}
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	foliageColor := foliageColors[positiveSeed%len(foliageColors)]
	
	foliageCenterX := treeWidth / 2
	foliageCenterY := treeHeight / 2
	radius := 15
	
	// Draw foliage circle
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= radius*radius {
				px := foliageCenterX + dx
				py := foliageCenterY + dy
				if px >= 0 && px < treeWidth && py >= 0 && py < treeHeight {
					treeImg.Set(px, py, foliageColor)
				}
			}
		}
	}
	
	// Draw tree to screen
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(screenX), float64(screenY))
	screen.DrawImage(treeImg, op)
}

// drawTrees draws a row of trees
func (gs *GameplayScreen) drawTrees(screen *ebiten.Image, x float64, y float64, height float64, seed int, leftSide bool) {
	treeSpacing := 120.0
	numTrees := int(height / treeSpacing) + 1
	
	// Ensure seed is positive for modulo operations
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	
	for i := 0; i < numTrees; i++ {
		treeY := y + float64(i)*treeSpacing + float64(positiveSeed%50)
		
		// Only draw if on screen
		if treeY > -50 && treeY < float64(gs.screenHeight)+50 {
			treeSeed := positiveSeed + i*17
			gs.drawTree(screen, x, treeY, treeSeed, leftSide)
		}
	}
}

// drawTree draws a single tree
func (gs *GameplayScreen) drawTree(screen *ebiten.Image, x, y float64, seed int, leftSide bool) {
	treeWidth := 40
	treeHeight := 60
	
	// Convert to integer screen coordinates
	screenX := int(x)
	screenY := int(y)
	
	// Skip if completely off screen
	if screenX+treeWidth < 0 || screenX > gs.screenWidth || screenY+treeHeight < 0 || screenY > gs.screenHeight {
		return
	}
	
	// Create tree sprite on offscreen image to avoid glitching
	treeImg := ebiten.NewImage(treeWidth, treeHeight)
	
	// Tree trunk (brown)
	trunkColor := color.RGBA{101, 67, 33, 255}
	trunkWidth := 8
	trunkHeight := 20
	trunkX := treeWidth/2 - trunkWidth/2
	trunkY := treeHeight - trunkHeight
	
	// Draw trunk
	for ty := 0; ty < trunkHeight; ty++ {
		for tx := 0; tx < trunkWidth; tx++ {
			treeImg.Set(trunkX+tx, trunkY+ty, trunkColor)
		}
	}
	
	// Tree foliage (green, varies by seed)
	foliageColors := []color.RGBA{
		{34, 139, 34, 255},   // Forest green
		{0, 128, 0, 255},     // Green
		{50, 150, 50, 255},   // Light green
		{20, 100, 20, 255},   // Dark green
	}
	// Ensure seed is positive for array indexing
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	foliageColor := foliageColors[positiveSeed%len(foliageColors)]
	
	// Draw foliage as overlapping circles
	foliageCenterX := treeWidth / 2
	foliageCenterY := treeHeight / 2
	
	// Main foliage circle
	radius := 18
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= radius*radius {
				px := foliageCenterX + dx
				py := foliageCenterY + dy
				if px >= 0 && px < treeWidth && py >= 0 && py < treeHeight {
					treeImg.Set(px, py, foliageColor)
				}
			}
		}
	}
	
	// Smaller circles for depth
	smallRadius := 12
	for _, offset := range []struct{ x, y int }{{-8, -5}, {8, -5}, {0, 8}} {
		for dy := -smallRadius; dy <= smallRadius; dy++ {
			for dx := -smallRadius; dx <= smallRadius; dx++ {
				if dx*dx+dy*dy <= smallRadius*smallRadius {
					px := foliageCenterX + dx + offset.x
					py := foliageCenterY + dy + offset.y
					if px >= 0 && px < treeWidth && py >= 0 && py < treeHeight {
						// Slightly darker for depth
						darkerColor := color.RGBA{
							uint8(math.Max(0, float64(foliageColor.R)-20)),
							uint8(math.Max(0, float64(foliageColor.G)-20)),
							uint8(math.Max(0, float64(foliageColor.B)-20)),
							255,
						}
						treeImg.Set(px, py, darkerColor)
					}
				}
			}
		}
	}
	
	// Draw tree sprite to screen
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(screenX), float64(screenY))
	screen.DrawImage(treeImg, op)
}


// drawCar renders the player's car
func (gs *GameplayScreen) drawCar(screen *ebiten.Image) {
	carWidth, carHeight := 40, 64

	// Car position on screen (convert world X to screen X with camera offset)
	screenX := gs.playerCar.X - gs.cameraX - float64(carWidth)/2
	screenY := float64(gs.screenHeight)/2 - float64(carHeight)/2 // Centered vertically

	// Create improved retro car sprite
	carImg := ebiten.NewImage(carWidth, carHeight)

	// Main car body (red)
	carBody := color.RGBA{220, 20, 20, 255}
	carHighlight := color.RGBA{255, 100, 100, 255}

	// Draw car body
	for y := 10; y < 54; y++ {
		for x := 5; x < 35; x++ {
			carImg.Set(x, y, carBody)
		}
	}

	// Draw roof (slightly darker and smaller)
	roofColor := color.RGBA{180, 15, 15, 255}
	for y := 15; y < 35; y++ {
		for x := 8; x < 32; x++ {
			carImg.Set(x, y, roofColor)
		}
	}

	// Draw windshield (light blue/cyan)
	windshieldColor := color.RGBA{100, 180, 220, 255}
	for y := 16; y < 28; y++ {
		for x := 10; x < 30; x++ {
			if y < 22 || (x > 12 && x < 28) {
				carImg.Set(x, y, windshieldColor)
			}
		}
	}

	// Draw wheels (black circles)
	wheelColor := color.RGBA{40, 40, 40, 255}
	// Front left wheel
	for y := 12; y < 20; y++ {
		for x := 2; x < 8; x++ {
			carImg.Set(x, y, wheelColor)
		}
	}
	// Front right wheel
	for y := 12; y < 20; y++ {
		for x := 32; x < 38; x++ {
			carImg.Set(x, y, wheelColor)
		}
	}
	// Rear left wheel
	for y := 44; y < 52; y++ {
		for x := 2; x < 8; x++ {
			carImg.Set(x, y, wheelColor)
		}
	}
	// Rear right wheel
	for y := 44; y < 52; y++ {
		for x := 32; x < 38; x++ {
			carImg.Set(x, y, wheelColor)
		}
	}

	// Add highlights on top of car
	for y := 12; y < 14; y++ {
		for x := 8; x < 32; x++ {
			carImg.Set(x, y, carHighlight)
		}
	}

	// Draw car shadow/outline (black border)
	borderColor := color.RGBA{0, 0, 0, 255}
	for x := 0; x < carWidth; x++ {
		carImg.Set(x, 10, borderColor)
		carImg.Set(x, 53, borderColor)
	}
	for y := 10; y < 54; y++ {
		carImg.Set(5, y, borderColor)
		carImg.Set(34, y, borderColor)
	}

	// Draw headlights (yellow)
	headlightColor := color.RGBA{255, 255, 100, 255}
	for y := 8; y < 11; y++ {
		for x := 10; x < 14; x++ {
			carImg.Set(x, y, headlightColor)
		}
		for x := 26; x < 30; x++ {
			carImg.Set(x, y, headlightColor)
		}
	}

	// Draw taillights (red)
	taillightColor := color.RGBA{255, 0, 0, 255}
	for y := 53; y < 56; y++ {
		for x := 10; x < 14; x++ {
			carImg.Set(x, y, taillightColor)
		}
		for x := 26; x < 30; x++ {
			carImg.Set(x, y, taillightColor)
		}
	}

	// Apply rotation based on steering angle
	op := &ebiten.DrawImageOptions{}
	
	// Rotate car sprite based on steering angle (subtle rotation)
	rotationAngle := gs.playerCar.SteeringAngle * 0.15 // Max 15 degrees rotation
	op.GeoM.Translate(-float64(carWidth)/2, -float64(carHeight)/2) // Center rotation
	op.GeoM.Rotate(rotationAngle)
	op.GeoM.Translate(float64(carWidth)/2, float64(carHeight)/2)
	
	op.GeoM.Translate(screenX, screenY)
	screen.DrawImage(carImg, op)
	
	// Draw steering wheel indicator in bottom-right corner
	gs.drawSteeringIndicator(screen)
}

// drawSteeringIndicator draws a visual indicator of the steering wheel position
func (gs *GameplayScreen) drawSteeringIndicator(screen *ebiten.Image) {
	// Position in bottom-right corner
	centerX := float64(gs.screenWidth - 80)
	centerY := float64(gs.screenHeight - 80)
	radius := 30.0
	
	// Draw steering wheel circle (gray)
	wheelImg := ebiten.NewImage(70, 70)
	wheelColor := color.RGBA{100, 100, 100, 255}
	
	// Draw circle outline
	for angle := 0.0; angle < 6.28; angle += 0.1 {
		x := 35 + int(radius*math.Cos(angle))
		y := 35 + int(radius*math.Sin(angle))
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				if x+dx >= 0 && x+dx < 70 && y+dy >= 0 && y+dy < 70 {
					wheelImg.Set(x+dx, y+dy, wheelColor)
				}
			}
		}
	}
	
	// Draw center mark
	centerColor := color.RGBA{200, 200, 200, 255}
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			wheelImg.Set(35+dx, 35+dy, centerColor)
		}
	}
	
	// Draw steering indicator line (red when turned, green when centered)
	var indicatorColor color.RGBA
	if gs.playerCar.SteeringAngle > 0.1 || gs.playerCar.SteeringAngle < -0.1 {
		indicatorColor = color.RGBA{255, 50, 50, 255} // Red when steering
	} else {
		indicatorColor = color.RGBA{50, 255, 50, 255} // Green when centered
	}
	
	// Draw line from center at steering angle
	lineAngle := gs.playerCar.SteeringAngle * 1.57 // 90 degrees max rotation
	lineLength := radius - 5
	endX := 35 + int(lineLength*math.Sin(lineAngle))
	endY := 35 - int(lineLength*math.Cos(lineAngle))
	
	// Draw thick line
	for t := 0.0; t <= 1.0; t += 0.02 {
		x := 35 + int(float64(endX-35)*t)
		y := 35 + int(float64(endY-35)*t)
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				if x+dx >= 0 && x+dx < 70 && y+dy >= 0 && y+dy < 70 {
					wheelImg.Set(x+dx, y+dy, indicatorColor)
				}
			}
		}
	}
	
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(centerX-35, centerY-35)
	screen.DrawImage(wheelImg, op)
	
	// Draw text label
	label := fmt.Sprintf("Steering: %.1f", gs.playerCar.SteeringAngle)
	ebitenutil.DebugPrintAt(screen, label, gs.screenWidth-150, gs.screenHeight-25)
}

// getCurrentRoadSegment finds the road segment the car is currently on
func (gs *GameplayScreen) getCurrentRoadSegment() RoadSegment {
	// Find the segment closest to the car's Y position
	// Since Y decreases as we go up, we need to find the segment where carY is between segment.Y and segment.Y-600
	carWorldY := gs.playerCar.Y

	for _, segment := range gs.roadSegments {
		if carWorldY <= segment.Y && carWorldY > segment.Y-600 {
			return segment
		}
	}

	// If no segment found and we have segments, check if player is beyond the last segment
	if len(gs.roadSegments) > 0 {
		lastSegment := gs.roadSegments[len(gs.roadSegments)-1]
		if carWorldY <= lastSegment.Y - 600 {
			// Player is beyond the last segment - use the last segment as reference
			return lastSegment
		}
		// Player is before the first segment - use the first segment
		return gs.roadSegments[0]
	}

	// Fallback
	return RoadSegment{LaneCount: 1, RoadTypes: []string{"A"}, StartLaneIndex: 0, Y: 0}
}

// getSegmentAt finds the road segment at a specific Y position
func (gs *GameplayScreen) getSegmentAt(y float64) RoadSegment {
	for _, segment := range gs.roadSegments {
		if y <= segment.Y && y > segment.Y-600 {
			return segment
		}
	}
	if len(gs.roadSegments) > 0 {
		return gs.roadSegments[0]
	}
	return RoadSegment{LaneCount: 1, RoadTypes: []string{"A"}, StartLaneIndex: 0, Y: 0}
}

// getCurrentLane determines which lane the car is currently in
// Returns the character position in the level file (position 0 = lane 0, even if it's X)
func (gs *GameplayScreen) getCurrentLane(segment RoadSegment, laneWidth float64) int {
	// Calculate the left edge of the road segment
	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	
	// Calculate which rendered lane the car is in based on its X position
	// Lane 0 starts at leftEdge, each lane is laneWidth wide
	relativeX := gs.playerCar.X - leftEdge
	renderedLaneIndex := int(relativeX / laneWidth)
	
	// Clamp rendered lane index to valid range
	if renderedLaneIndex < 0 {
		renderedLaneIndex = 0
	}
	if renderedLaneIndex >= segment.LaneCount {
		renderedLaneIndex = segment.LaneCount - 1
	}
	
	// Map rendered lane index to character position in level file
	// This ensures position 0 in the level file is always lane 0, even if it's 'X'
	if renderedLaneIndex < len(segment.LanePositions) {
		return segment.LanePositions[renderedLaneIndex]
	}
	
	// Fallback: if no mapping exists, return the rendered index (shouldn't happen)
	return renderedLaneIndex
}

// checkCollisions checks if the player car collides with any traffic vehicles
func (gs *GameplayScreen) checkCollisions() bool {
	// Use smaller collision boxes than the actual car size to allow maneuvering between cars
	// Actual car size is 40x64, but we'll use smaller collision boxes
	collisionWidth := 30.0  // Smaller than 40px car width
	collisionHeight := 50.0 // Smaller than 64px car height
	
	// Player car is drawn at fixed screen position (screenHeight - 150 = 450)
	// Traffic cars are drawn at: screenY = tc.Y - gs.playerCar.Y + screenHeight/2
	// For collision at same screen Y: 450 = tc.Y - gs.playerCar.Y + 300
	// Therefore: tc.Y = gs.playerCar.Y + 150
	// We need to check if traffic Y is within collisionHeight of this position
	
	// Player car world X position (using smaller collision box)
	playerLeft := gs.playerCar.X - collisionWidth/2
	playerRight := gs.playerCar.X + collisionWidth/2
	
	// Player car's effective world Y for collision
	// Both player and traffic use the same world coordinate system
	playerCollisionY := gs.playerCar.Y
	playerYTop := playerCollisionY - collisionHeight/2
	playerYBottom := playerCollisionY + collisionHeight/2
	
	// Check collision with each traffic vehicle
	gs.trafficMutex.RLock()
	defer gs.trafficMutex.RUnlock()
	for _, tc := range gs.traffic {
		// Traffic car world X position (using smaller collision box)
		trafficLeft := tc.X - collisionWidth/2
		trafficRight := tc.X + collisionWidth/2
		
		// Traffic car world Y bounding box (using smaller collision box)
		trafficYTop := tc.Y - collisionHeight/2
		trafficYBottom := tc.Y + collisionHeight/2
		
		// Check X overlap
		if playerLeft < trafficRight && playerRight > trafficLeft {
			// Check Y overlap (traffic bounding box overlaps with player collision range)
			if trafficYTop < playerYBottom && trafficYBottom > playerYTop {
				return true // Collision detected
			}
		}
	}
	
	return false
}

// getSegmentAtY finds the road segment at a given world Y position
func (gs *GameplayScreen) getSegmentAtY(y float64) RoadSegment {
	// Segments are ordered by decreasing Y
	for _, segment := range gs.roadSegments {
		if y <= segment.Y && y > segment.Y - 600 {
			return segment
		}
	}
	// Fallback
	if len(gs.roadSegments) > 0 {
		return gs.roadSegments[0]
	}
	return RoadSegment{LaneCount: 1, RoadTypes: []string{"A"}, StartLaneIndex: 0, Y: 0}
}

// resetToStart resets the player to the start of the level
func (gs *GameplayScreen) resetToStart() {
	// Reset player position
	gs.playerCar.X = gs.initialX
	gs.playerCar.Y = gs.initialY
	gs.playerCar.VelocityX = 0
	gs.playerCar.VelocityY = 0
	gs.playerCar.SteeringAngle = 0
	gs.cameraX = 0

	// Clear all traffic
	gs.cleanupTraffic()

	// Regenerate road from level data
	gs.roadSegments = make([]RoadSegment, 0)
	gs.generateRoadFromLevel(gs.levelData)

	// Spawn initial traffic again
	gs.spawnInitialTraffic()
}

// cleanupTraffic stops all AI goroutines and clears traffic
func (gs *GameplayScreen) cleanupTraffic() {
	for _, tc := range gs.traffic {
		tc.StopAI()
	}
	gs.traffic = make([]*TrafficCar, 0)
}

// updateTraffic updates traffic positions and spawns new traffic vehicles
func (gs *GameplayScreen) updateTraffic(scrollSpeed float64, currentSegment RoadSegment, laneWidth float64) {
	playerY := gs.playerCar.Y
	
	// Update existing traffic positions
	gs.trafficMutex.Lock()
	for i := 0; i < len(gs.traffic); i++ {
		tc := gs.traffic[i]
		
		// Traffic moves relative to player: if traffic is slower, it moves up (toward player)
		// If traffic is faster, it moves down (away from player)
		// Since Y decreases upward, we subtract the absolute speed to move up
		tc.Y -= tc.VelocityY
		
		// Remove traffic that's too far off screen (beyond spawn range)
		if tc.Y > playerY+trafficSpawnRange+500 || tc.Y < playerY-trafficSpawnRange-500 {
			// Stop AI behavior
			tc.StopAI()
			// Remove from slice
			gs.traffic = append(gs.traffic[:i], gs.traffic[i+1:]...)
			i--
			continue
		}
	}

	// Enforce ordering within each lane so cars never overtake
	laneCars := make(map[int][]*TrafficCar)
	for _, tc := range gs.traffic {
		laneCars[tc.Lane] = append(laneCars[tc.Lane], tc)
	}

	for _, cars := range laneCars {
		sort.Slice(cars, func(i, j int) bool {
			return cars[i].Y < cars[j].Y // smaller Y is further ahead
		})

		for i := 1; i < len(cars); i++ {
			ahead := cars[i-1]
			behind := cars[i]

			minAllowedY := ahead.Y + minTrafficDistance
			if behind.Y < minAllowedY {
				behind.Y = minAllowedY
			}

			// Ensure the car behind is not faster than the car ahead
			if behind.VelocityY > ahead.VelocityY {
				behind.VelocityY = ahead.VelocityY
			}
		}
	}
	gs.trafficMutex.Unlock()
	
	// Spawn new traffic vehicles
	gs.spawnTraffic(currentSegment, laneWidth, playerY)
}

// spawnInitialTraffic spawns initial traffic when the game starts
func (gs *GameplayScreen) spawnInitialTraffic() {
	if len(gs.roadSegments) == 0 {
		return
	}
	
	segment := gs.roadSegments[0]
	laneWidth := 80.0
	playerY := gs.playerCar.Y
	
	// Spawn traffic in each lane (skip lane 0)
	for lane := 1; lane < segment.LaneCount; lane++ {
		// Spawn at most one vehicle ahead and behind with probability to keep density low
		if rand.Float64() < trafficSpawnProbability {
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, true)
		}
		if rand.Float64() < trafficSpawnProbability {
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, false)
		}
	}
}

// spawnTraffic spawns traffic vehicles ahead and behind the player
func (gs *GameplayScreen) spawnTraffic(segment RoadSegment, laneWidth float64, playerY float64) {
	// Check cooldown before attempting to spawn
	currentTime := time.Now().UnixMilli()
	if currentTime - gs.lastSpawnTime < gs.spawnCooldown {
		return
	}
	gs.lastSpawnTime = currentTime

	// Only spawn if we have lanes available (multi-lane roads)
	if segment.LaneCount < 2 {
		return
	}

	// Consistent spawning: try each lane in sequence
	for lane := 1; lane < segment.LaneCount; lane++ {
		// Consistent probability for each lane
		baseProbability := trafficSpawnProbability

		// Always try to spawn ahead first (more visible)
		if rand.Float64() < baseProbability {
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, true)
		}

		// Lower chance to spawn behind
		if rand.Float64() < baseProbability * 0.4 {
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, false)
		}
	}
}

// spawnTrafficInDirection spawns traffic in a specific direction (ahead or behind)
func (gs *GameplayScreen) spawnTrafficInDirection(segment RoadSegment, laneWidth float64, playerY float64, lane int, ahead bool) {
	// Find existing traffic in this lane
	var laneTraffic []*TrafficCar
	
	gs.trafficMutex.RLock()
	for _, tc := range gs.traffic {
		// Check if traffic is in this lane (within lane bounds)
		leftEdge := -float64(segment.StartLaneIndex) * laneWidth
		laneLeft := leftEdge + float64(lane)*laneWidth
		laneRight := laneLeft + laneWidth
		
		if tc.X >= laneLeft && tc.X < laneRight {
			laneTraffic = append(laneTraffic, tc)
		}
	}
	gs.trafficMutex.RUnlock()
	
	// Determine spawn range - spawn well off-screen
	// Screen height is 600, so we want to spawn at least 1000px away from player
	var minY, maxY float64
	if ahead {
		// Spawn ahead (above player, lower Y values)
		// Spawn between 3000px and 800px ahead
		minY = playerY - trafficSpawnRange
		maxY = playerY - 800
	} else {
		// Spawn behind (below player, higher Y values)
		// Spawn between 800px and 3000px behind
		minY = playerY + 800
		maxY = playerY + trafficSpawnRange
	}
	
	// Generate a candidate spawn position uniformly in range
	spawnY := minY + rand.Float64()*(maxY-minY)
	
	// Check if the candidate position is safe (maintaining minimum distance)
	isSafe := true
	for _, tc := range laneTraffic {
		if math.Abs(tc.Y - spawnY) < minTrafficDistance {
			isSafe = false
			break
		}
	}
	
	// If not safe, don't spawn
	if !isSafe {
		return
	}
	
	// Calculate lane center X position
	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	laneCenterX := leftEdge + float64(lane)*laneWidth + laneWidth/2
	
	// Map rendered lane index to character position in level file
	// Position 0 in level file is always lane 0, even if it's 'X'
	lanePosition := lane
	if lane < len(segment.LanePositions) {
		lanePosition = segment.LanePositions[lane]
	}
	
	// Determine traffic speed (exact lane speed limit)
	// Lane 0=50, Lane 1=65, Lane 2=80, etc. (15mph steps for visibility)
	// Use character position for speed calculation
	speedLimitMPH := 50.0 + float64(lanePosition)*15.0
	trafficVelocityY := speedLimitMPH / MPHPerPixelPerFrame
	
	// Random car colors for variety
	colors := []color.RGBA{
		{100, 150, 200, 255}, // Blue
		{200, 150, 100, 255}, // Brown
		{150, 150, 150, 255}, // Gray
		{50, 150, 50, 255},   // Green
		{200, 200, 50, 255},  // Yellow
		{200, 100, 200, 255}, // Purple
	}
	carColor := colors[rand.Intn(len(colors))]
	
	// Create new traffic car
	newTraffic := &TrafficCar{
		X:           laneCenterX,
		Y:           spawnY,
		VelocityY:   trafficVelocityY,
		TargetSpeed: trafficVelocityY,
		Lane:        lane,
		Color:       carColor,
	}
	
	// Start AI behavior
	newTraffic.StartAI(gs)
	
	gs.trafficMutex.Lock()
	gs.traffic = append(gs.traffic, newTraffic)
	gs.trafficMutex.Unlock()
}

// drawTraffic renders all traffic vehicles
func (gs *GameplayScreen) drawTraffic(screen *ebiten.Image) {
	carWidth, carHeight := 40, 64
	
	gs.trafficMutex.RLock()
	defer gs.trafficMutex.RUnlock()
	
	for _, tc := range gs.traffic {
		// Calculate screen position relative to player car
		// Center the traffic car vertically to match player car center logic
		screenY := tc.Y - gs.playerCar.Y + float64(gs.screenHeight)/2 - float64(carHeight)/2
		
		// Only draw if on screen
		if screenY < -100 || screenY > float64(gs.screenHeight)+100 {
			continue
		}
		
		// Calculate screen X position (convert world X to screen X with camera offset)
		screenX := tc.X - gs.cameraX - float64(carWidth)/2

		// Skip if too far off-screen horizontally to prevent ebiten panics
		if screenX < -200 || screenX > float64(gs.screenWidth)+200 {
			continue
		}
		
		// Create traffic car sprite (similar to player car but with different color)
		carImg := ebiten.NewImage(carWidth, carHeight)
		
		// Use the traffic car's color
		carBody := tc.Color
		carHighlight := color.RGBA{
			uint8(math.Min(255, float64(carBody.R)+30)),
			uint8(math.Min(255, float64(carBody.G)+30)),
			uint8(math.Min(255, float64(carBody.B)+30)),
			255,
		}
		
		// Draw car body
		for y := 10; y < 54; y++ {
			for x := 5; x < 35; x++ {
				carImg.Set(x, y, carBody)
			}
		}
		
		// Draw roof (slightly darker)
		roofColor := color.RGBA{
			uint8(math.Max(0, float64(carBody.R)-40)),
			uint8(math.Max(0, float64(carBody.G)-40)),
			uint8(math.Max(0, float64(carBody.B)-40)),
			255,
		}
		for y := 15; y < 35; y++ {
			for x := 8; x < 32; x++ {
				carImg.Set(x, y, roofColor)
			}
		}
		
		// Draw windshield (light blue/cyan)
		windshieldColor := color.RGBA{100, 180, 220, 255}
		for y := 16; y < 28; y++ {
			for x := 10; x < 30; x++ {
				if y < 22 || (x > 12 && x < 28) {
					carImg.Set(x, y, windshieldColor)
				}
			}
		}
		
		// Draw wheels (black)
		wheelColor := color.RGBA{40, 40, 40, 255}
		// Front left wheel
		for y := 12; y < 20; y++ {
			for x := 2; x < 8; x++ {
				carImg.Set(x, y, wheelColor)
			}
		}
		// Front right wheel
		for y := 12; y < 20; y++ {
			for x := 32; x < 38; x++ {
				carImg.Set(x, y, wheelColor)
			}
		}
		// Rear left wheel
		for y := 44; y < 52; y++ {
			for x := 2; x < 8; x++ {
				carImg.Set(x, y, wheelColor)
			}
		}
		// Rear right wheel
		for y := 44; y < 52; y++ {
			for x := 32; x < 38; x++ {
				carImg.Set(x, y, wheelColor)
			}
		}
		
		// Add highlights
		for y := 12; y < 14; y++ {
			for x := 8; x < 32; x++ {
				carImg.Set(x, y, carHighlight)
			}
		}
		
		// Draw car shadow/outline (black border)
		borderColor := color.RGBA{0, 0, 0, 255}
		for x := 0; x < carWidth; x++ {
			carImg.Set(x, 10, borderColor)
			carImg.Set(x, 53, borderColor)
		}
		for y := 10; y < 54; y++ {
			carImg.Set(5, y, borderColor)
			carImg.Set(34, y, borderColor)
		}
		
		// Draw headlights (yellow)
		headlightColor := color.RGBA{255, 255, 100, 255}
		for y := 8; y < 11; y++ {
			for x := 10; x < 14; x++ {
				carImg.Set(x, y, headlightColor)
			}
			for x := 26; x < 30; x++ {
				carImg.Set(x, y, headlightColor)
			}
		}
		
		// Draw taillights (red)
		taillightColor := color.RGBA{255, 0, 0, 255}
		for y := 53; y < 56; y++ {
			for x := 10; x < 14; x++ {
				carImg.Set(x, y, taillightColor)
			}
			for x := 26; x < 30; x++ {
				carImg.Set(x, y, taillightColor)
			}
		}
		
		// Draw the traffic car
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(screenX, screenY)
		screen.DrawImage(carImg, op)
		
		// Debug: Draw speed
		speedMPH := tc.VelocityY * MPHPerPixelPerFrame
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%.0f", speedMPH), int(screenX), int(screenY)-15)
	}
}

// drawUI renders the game UI overlay
func (gs *GameplayScreen) drawUI(screen *ebiten.Image) {
	// Draw speedometer
	gs.drawSpeedometer(screen)

	// Draw mini-map or progress indicator
	progressText := "PROGRESS: 0%"

	// Draw fuel indicator
	fuelPercent := gs.playerCar.SelectedCar.FuelLevel / gs.playerCar.SelectedCar.FuelCapacity * 100
	fuelText := fmt.Sprintf("FUEL: %.0f%%", fuelPercent)

	// For now, just draw simple text overlays
	// We'll implement proper text rendering later
	_ = progressText
	_ = fuelText
}

// drawSpeedometer draws a speedometer displaying current speed in MPH
func (gs *GameplayScreen) drawSpeedometer(screen *ebiten.Image) {
	// Calculate speed in MPH from VelocityY (pixels per frame)
	speedMPH := gs.playerCar.VelocityY * MPHPerPixelPerFrame
	
	// Get current lane and speed limit
	currentSegment := gs.getCurrentRoadSegment()
	laneWidth := 80.0
	currentLane := gs.getCurrentLane(currentSegment, laneWidth)
	speedLimitMPH := 50.0 + float64(currentLane)*10.0
	
	// Position in top-left corner
	x := 20.0
	y := 20.0
	width := 180.0
	height := 140.0 // Increased height to fit speed limit display
	
	// Draw speedometer background (semi-transparent dark box)
	bgImg := ebiten.NewImage(int(width), int(height))
	bgColor := color.RGBA{20, 20, 30, 200} // Dark with transparency
	bgImg.Fill(bgColor)
	
	// Draw border
	borderColor := color.RGBA{100, 100, 120, 255}
	borderWidth := 2
	w, h := int(width), int(height)
	
	// Top and bottom borders
	for i := 0; i < w; i++ {
		for j := 0; j < borderWidth; j++ {
			bgImg.Set(i, j, borderColor)
			bgImg.Set(i, h-1-j, borderColor)
		}
	}
	// Left and right borders
	for i := 0; i < h; i++ {
		for j := 0; j < borderWidth; j++ {
			bgImg.Set(j, i, borderColor)
			bgImg.Set(w-1-j, i, borderColor)
		}
	}
	
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(bgImg, op)
	
	// Draw speed value (large number)
	face := text.NewGoXFace(bitmapfont.Face)
	speedText := fmt.Sprintf("%.0f", speedMPH)
	
	// Calculate text size and position
	textScale := 3.0
	textWidth := text.Advance(speedText, face) * textScale
	textX := x + width/2 - textWidth/2
	textY := y + 50.0
	
	textOp := &text.DrawOptions{}
	textOp.GeoM.Scale(textScale, textScale)
	textOp.GeoM.Translate(textX/textScale, textY/textScale)
	
	// Color based on speed (green for normal, yellow for fast, red for very fast)
	var speedColor color.RGBA
	if speedMPH < 50 {
		speedColor = color.RGBA{100, 255, 100, 255} // Green
	} else if speedMPH < 80 {
		speedColor = color.RGBA{255, 255, 100, 255} // Yellow
	} else {
		speedColor = color.RGBA{255, 100, 100, 255} // Red
	}
	textOp.ColorScale.ScaleWithColor(speedColor)
	text.Draw(screen, speedText, face, textOp)
	
	// Draw "MPH" label below speed
	labelText := "MPH"
	labelScale := 1.5
	labelWidth := text.Advance(labelText, face) * labelScale
	labelX := x + width/2 - labelWidth/2
	labelY := y + 85.0
	
	labelOp := &text.DrawOptions{}
	labelOp.GeoM.Scale(labelScale, labelScale)
	labelOp.GeoM.Translate(labelX/labelScale, labelY/labelScale)
	labelOp.ColorScale.ScaleWithColor(color.RGBA{200, 200, 200, 255})
	text.Draw(screen, labelText, face, labelOp)
	
	// Draw speed limit indicator
	limitText := fmt.Sprintf("LIMIT: %.0f MPH", speedLimitMPH)
	limitScale := 1.0
	limitWidth := text.Advance(limitText, face) * limitScale
	limitX := x + width/2 - limitWidth/2
	limitY := y + 105.0
	
	limitOp := &text.DrawOptions{}
	limitOp.GeoM.Scale(limitScale, limitScale)
	limitOp.GeoM.Translate(limitX/limitScale, limitY/limitScale)
	
	// Color limit text: yellow if at/over limit, white if under
	if speedMPH >= speedLimitMPH {
		limitOp.ColorScale.ScaleWithColor(color.RGBA{255, 200, 50, 255}) // Yellow warning
	} else {
		limitOp.ColorScale.ScaleWithColor(color.RGBA{200, 200, 200, 255}) // White
	}
	text.Draw(screen, limitText, face, limitOp)
	
	// Draw simple speed gauge bar
	gs.drawSpeedGauge(screen, x+10, y+height-25, width-20, 15, speedMPH, speedLimitMPH)
}

// drawSpeedGauge draws a simple horizontal gauge bar showing speed
func (gs *GameplayScreen) drawSpeedGauge(screen *ebiten.Image, x, y, width, height float64, speedMPH float64, speedLimitMPH float64) {
	maxSpeed := 100.0 // Maximum speed for gauge (100 MPH)
	speedPercent := math.Min(speedMPH/maxSpeed, 1.0)
	limitPercent := math.Min(speedLimitMPH/maxSpeed, 1.0)
	
	// Draw background bar (dark gray) with border
	bgBar := ebiten.NewImage(int(width), int(height))
	bgBar.Fill(color.RGBA{40, 40, 40, 255})
	
	// Draw border around gauge
	borderColor := color.RGBA{150, 150, 150, 255}
	w, h := int(width), int(height)
	for i := 0; i < w; i++ {
		bgBar.Set(i, 0, borderColor)
		bgBar.Set(i, h-1, borderColor)
	}
	for i := 0; i < h; i++ {
		bgBar.Set(0, i, borderColor)
		bgBar.Set(w-1, i, borderColor)
	}
	
	// Draw background bar first
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(bgBar, op)
	
	// Draw filled portion based on speed on top of background
	filledWidth := int(width * speedPercent)
	if filledWidth > 0 {
		filledBar := ebiten.NewImage(filledWidth, int(height))
		
		// Color gradient: green -> yellow -> red
		var barColor color.RGBA
		if speedPercent < 0.5 {
			// Green to yellow
			ratio := speedPercent / 0.5
			barColor = color.RGBA{
				uint8(100 + ratio*155),
				uint8(255),
				uint8(100),
				255,
			}
		} else {
			// Yellow to red
			ratio := (speedPercent - 0.5) / 0.5
			barColor = color.RGBA{
				uint8(255),
				uint8(255 - ratio*155),
				uint8(100 - ratio*100),
				255,
			}
		}
		filledBar.Fill(barColor)
		
		filledOp := &ebiten.DrawImageOptions{}
		filledOp.GeoM.Translate(x, y)
		screen.DrawImage(filledBar, filledOp)
	}
	
	// Draw speed limit marker line
	if limitPercent > 0 && limitPercent <= 1.0 {
		limitXPos := x + width*limitPercent
		limitLine := ebiten.NewImage(2, int(height))
		limitLine.Fill(color.RGBA{255, 255, 255, 255}) // White line
		
		limitOp := &ebiten.DrawImageOptions{}
		limitOp.GeoM.Translate(limitXPos-1, y)
		screen.DrawImage(limitLine, limitOp)
	}
}
