package game

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"

	"github.com/golangdaddy/roadster/pkg/models/car"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// MPHPerPixelPerFrame is the conversion factor from pixels per frame to MPH
// At 60 FPS, max speed of 8 pixels/frame = 480 pixels/second
// Setting max speed to 100 MPH gives us: 8 pixels/frame = 100 MPH
// Therefore: 1 pixel/frame = 12.5 MPH
const MPHPerPixelPerFrame = 12.5

// Traffic constants
const (
	minTrafficDistance = 300.0 // Minimum distance between traffic vehicles in pixels
	trafficVariation   = 0.2   // 20% random variation on distance
	trafficSpawnRange  = 5000.0 // Range ahead/behind player to spawn traffic (well off-screen)
)

// TrafficCar represents a traffic vehicle
type TrafficCar struct {
	X, Y      float64 // World position
	VelocityY float64 // Vertical velocity (speed)
	Lane      int     // Which lane this car is in
	Color     color.RGBA // Car color for variety
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
	gs.generateBackgroundPattern()

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
		
		startLaneIdx := segment.StartLaneIndex
		
		if i < 3 { // First 3 segments are always 1 lane
			laneCount = 1
			// Use only the starting lane's road type
			if startLaneIdx < len(roadTypes) {
				roadTypes = []string{roadTypes[startLaneIdx]}
			} else if len(roadTypes) > 0 {
				roadTypes = []string{roadTypes[0]}
			}
			startLaneIdx = 0 // Starting lane is at index 0 when there's only 1 lane
		}

		roadSegment := RoadSegment{
			LaneCount:      laneCount,
			RoadTypes:      roadTypes,
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
	
	minSpeed := 2.0
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
		// Gradually return to default speed (but don't exceed speed limit)
		defaultSpeed := 4.0
		if defaultSpeed > maxSpeed {
			defaultSpeed = maxSpeed
		}
		if gs.playerCar.VelocityY < defaultSpeed {
			gs.playerCar.VelocityY += gs.playerCar.Acceleration * 0.5
			if gs.playerCar.VelocityY > defaultSpeed {
				gs.playerCar.VelocityY = defaultSpeed
			}
		} else if gs.playerCar.VelocityY > defaultSpeed {
			gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 0.5
			if gs.playerCar.VelocityY < defaultSpeed {
				gs.playerCar.VelocityY = defaultSpeed
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
	for i := range gs.roadSegments {
		gs.roadSegments[i].Y += scrollSpeed
	}

	// Update car position (car moves upward toward top of screen)
	gs.playerCar.Y -= scrollSpeed

	// Update traffic
	gs.updateTraffic(scrollSpeed, currentSegment, laneWidth)

	// Check for collisions with traffic
	if gs.checkCollisions() {
		gs.resetToStart()
	}

	// Remove segments that have scrolled off screen
	// Use simple screen-based check matching the drawing logic
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

	// All segments are pre-generated from level data, no dynamic addition needed

	return nil
}

// Draw renders the gameplay screen
func (gs *GameplayScreen) Draw(screen *ebiten.Image) {
	// Clear screen with sky color (retro blue)
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

	// Skip if off screen (using 600px segment height)
	if screenY > float64(gs.screenHeight) || screenY < -600 {
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
	gs.drawDecorativeLayer(screen, segment, screenY, roadX, laneWidth)

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

// generateBackgroundPattern creates a repeating background pattern with trees and water
func (gs *GameplayScreen) generateBackgroundPattern() {
	patternHeight := 600 // Match segment height
	patternWidth := gs.screenWidth
	
	pattern := ebiten.NewImage(patternWidth, patternHeight)
	
	// Fill with grass
	pattern.Fill(color.RGBA{34, 139, 34, 255})
	
	// Add grass texture variation
	for x := 0; x < patternWidth; x++ {
		for y := 0; y < patternHeight; y += 4 {
			if (x+y)%7 == 0 {
				pattern.Set(x, y, color.RGBA{50, 160, 50, 255})
			}
		}
	}
	
	// Trees will be drawn dynamically per segment based on road position
	// We'll just generate the grass pattern here
	
	// Add water course in middle section (spans full width)
	waterY := patternHeight / 3
	gs.drawWaterToPattern(pattern, 0.0, float64(patternWidth), float64(waterY))
	
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

// drawWaterToPattern draws water to the pattern
func (gs *GameplayScreen) drawWaterToPattern(pattern *ebiten.Image, leftX, rightX, y float64) {
	waterHeight := 25.0
	waterColor := color.RGBA{30, 144, 255, 255}
	
	for wy := 0; wy < int(waterHeight); wy++ {
		for wx := int(leftX); wx < int(rightX); wx++ {
			if wx >= 0 && wx < pattern.Bounds().Dx() {
				py := int(y) + wy
				if py >= 0 && py < pattern.Bounds().Dy() {
					pattern.Set(wx, py, waterColor)
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

// drawWaterCourse draws a water feature (river or stream)
func (gs *GameplayScreen) drawWaterCourse(screen *ebiten.Image, leftX, rightX, y float64, seed int) {
	waterWidth := rightX - leftX
	waterHeight := 30.0
	
	// Water color (blue, varies slightly)
	waterColors := []color.RGBA{
		{30, 144, 255, 255},  // Dodger blue
		{0, 191, 255, 255},   // Deep sky blue
		{70, 130, 180, 255},  // Steel blue
		{100, 149, 237, 255}, // Cornflower blue
	}
	// Ensure seed is positive for array indexing
	positiveSeed := seed
	if positiveSeed < 0 {
		positiveSeed = -positiveSeed
	}
	baseWaterColor := waterColors[positiveSeed%len(waterColors)]
	
	// Draw water with some variation
	for wy := 0; wy < int(waterHeight); wy++ {
		for wx := 0; wx < int(waterWidth); wx++ {
			screenX := int(leftX) + wx
			screenY := int(y) + wy
			
			if screenX >= 0 && screenX < gs.screenWidth && screenY >= 0 && screenY < gs.screenHeight {
				// Add some variation for water effect
				variation := (wx + wy + seed) % 10
				waterColor := color.RGBA{
					uint8(math.Min(255, float64(baseWaterColor.R)+float64(variation))),
					uint8(math.Min(255, float64(baseWaterColor.G)+float64(variation))),
					uint8(math.Min(255, float64(baseWaterColor.B)+float64(variation))),
					255,
				}
				screen.Set(screenX, screenY, waterColor)
			}
		}
	}
	
	// Draw water edges (slightly darker)
	edgeColor := color.RGBA{
		uint8(math.Max(0, float64(baseWaterColor.R)-30)),
		uint8(math.Max(0, float64(baseWaterColor.G)-30)),
		uint8(math.Max(0, float64(baseWaterColor.B)-30)),
		255,
	}
	
	// Top and bottom edges
	for wx := 0; wx < int(waterWidth); wx++ {
		screenX := int(leftX) + wx
		if screenX >= 0 && screenX < gs.screenWidth {
			// Top edge
			screenY := int(y)
			if screenY >= 0 && screenY < gs.screenHeight {
				screen.Set(screenX, screenY, edgeColor)
			}
			// Bottom edge
			screenY = int(y) + int(waterHeight) - 1
			if screenY >= 0 && screenY < gs.screenHeight {
				screen.Set(screenX, screenY, edgeColor)
			}
		}
	}
	
	// Left and right edges
	for wy := 0; wy < int(waterHeight); wy++ {
		screenY := int(y) + wy
		if screenY >= 0 && screenY < gs.screenHeight {
			// Left edge
			screenX := int(leftX)
			if screenX >= 0 && screenX < gs.screenWidth {
				screen.Set(screenX, screenY, edgeColor)
			}
			// Right edge
			screenX = int(rightX) - 1
			if screenX >= 0 && screenX < gs.screenWidth {
				screen.Set(screenX, screenY, edgeColor)
			}
		}
	}
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

	// Default to first segment if not found
	if len(gs.roadSegments) > 0 {
		return gs.roadSegments[0]
	}

	// Fallback
	return RoadSegment{LaneCount: 1, RoadTypes: []string{"A"}, StartLaneIndex: 0, Y: 0}
}

// getCurrentLane determines which lane the car is currently in
func (gs *GameplayScreen) getCurrentLane(segment RoadSegment, laneWidth float64) int {
	// Calculate the left edge of the road segment
	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	
	// Calculate which lane the car is in based on its X position
	// Lane 0 starts at leftEdge, each lane is laneWidth wide
	relativeX := gs.playerCar.X - leftEdge
	laneIndex := int(relativeX / laneWidth)
	
	// Clamp lane index to valid range
	if laneIndex < 0 {
		laneIndex = 0
	}
	if laneIndex >= segment.LaneCount {
		laneIndex = segment.LaneCount - 1
	}
	
	return laneIndex
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
	
	// Player car's effective world Y for collision (where traffic would be to collide)
	// Player is at screen Y 450, traffic at same position means: tc.Y = gs.playerCar.Y + 150
	playerCollisionY := gs.playerCar.Y + 150.0
	playerYTop := playerCollisionY - collisionHeight/2
	playerYBottom := playerCollisionY + collisionHeight/2
	
	// Check collision with each traffic vehicle
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
	gs.traffic = make([]*TrafficCar, 0)
	
	// Regenerate road from level data
	gs.roadSegments = make([]RoadSegment, 0)
	gs.generateRoadFromLevel(gs.levelData)
	
	// Spawn initial traffic again
	gs.spawnInitialTraffic()
}

// updateTraffic updates traffic positions and spawns new traffic vehicles
func (gs *GameplayScreen) updateTraffic(scrollSpeed float64, currentSegment RoadSegment, laneWidth float64) {
	playerY := gs.playerCar.Y
	playerVelocityY := gs.playerCar.VelocityY
	
	// Update existing traffic positions
	for i := 0; i < len(gs.traffic); i++ {
		tc := gs.traffic[i]
		
		// Traffic moves relative to player: if traffic is slower, it moves up (toward player)
		// If traffic is faster, it moves down (away from player)
		// Since Y decreases upward, we subtract the relative speed
		relativeSpeed := tc.VelocityY - playerVelocityY
		tc.Y -= relativeSpeed
		
		// Remove traffic that's too far off screen (beyond spawn range)
		if tc.Y > playerY+trafficSpawnRange+500 || tc.Y < playerY-trafficSpawnRange-500 {
			// Remove from slice
			gs.traffic = append(gs.traffic[:i], gs.traffic[i+1:]...)
			i--
			continue
		}
	}
	
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
		// Spawn a few vehicles ahead and behind
		for i := 0; i < 3; i++ {
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, true)
			gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, false)
		}
	}
}

// spawnTraffic spawns traffic vehicles ahead and behind the player
func (gs *GameplayScreen) spawnTraffic(segment RoadSegment, laneWidth float64, playerY float64) {
	// Limit how often we check for new traffic (every few frames)
	// For now, check every frame but only spawn if needed
	
	// Check each lane for traffic spawning (skip lane 0)
	for lane := 1; lane < segment.LaneCount; lane++ {
		// Check if we need to spawn traffic ahead (above player)
		gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, true)
		
		// Check if we need to spawn traffic behind (below player)
		gs.spawnTrafficInDirection(segment, laneWidth, playerY, lane, false)
	}
}

// spawnTrafficInDirection spawns traffic in a specific direction (ahead or behind)
func (gs *GameplayScreen) spawnTrafficInDirection(segment RoadSegment, laneWidth float64, playerY float64, lane int, ahead bool) {
	// Find existing traffic in this lane
	var laneTraffic []*TrafficCar
	for _, tc := range gs.traffic {
		// Check if traffic is in this lane (within lane bounds)
		leftEdge := -float64(segment.StartLaneIndex) * laneWidth
		laneLeft := leftEdge + float64(lane)*laneWidth
		laneRight := laneLeft + laneWidth
		
		if tc.X >= laneLeft && tc.X < laneRight {
			laneTraffic = append(laneTraffic, tc)
		}
	}
	
	// Determine spawn range - spawn well off-screen
	// Screen height is 600, so we want to spawn at least 1000px away from player
	var minY, maxY float64
	if ahead {
		// Spawn ahead (above player, lower Y values)
		// Spawn between 1500-5000px ahead of player (well off-screen)
		minY = playerY - trafficSpawnRange
		maxY = playerY - 1500 // Don't spawn too close to player (well off-screen)
	} else {
		// Spawn behind (below player, higher Y values)
		// Spawn between 1500-5000px behind player (well off-screen)
		minY = playerY + 1500 // Don't spawn too close to player (well off-screen)
		maxY = playerY + trafficSpawnRange
	}
	
	// Check if there's already traffic in the spawn range
	hasTrafficInRange := false
	for _, tc := range laneTraffic {
		if tc.Y >= minY && tc.Y <= maxY {
			hasTrafficInRange = true
			break
		}
	}
	
	// If there's already traffic in range, don't spawn more (they'll spawn naturally as traffic moves)
	if hasTrafficInRange {
		return
	}
	
	// No traffic in range, spawn one
	// Calculate spawn position with proper spacing from player (well off-screen)
	var spawnY float64
	if ahead {
		// Spawn ahead of player (well off-screen)
		// Spawn between 1500-3000px ahead
		spawnY = playerY - 1500 - minTrafficDistance*float64(rand.Intn(5)) // Spawn 1500-3000px ahead
		if spawnY < minY {
			spawnY = minY + (maxY-minY)*rand.Float64() // Random position in spawn range
		}
		if spawnY > maxY {
			spawnY = maxY
		}
	} else {
		// Spawn behind player (well off-screen)
		// Spawn between 1500-3000px behind
		spawnY = playerY + 1500 + minTrafficDistance*float64(rand.Intn(5)) // Spawn 1500-3000px behind
		if spawnY > maxY {
			spawnY = maxY - (maxY-minY)*rand.Float64() // Random position in spawn range
		}
		if spawnY < minY {
			spawnY = minY
		}
	}
	
	// Calculate lane center X position
	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	laneCenterX := leftEdge + float64(lane)*laneWidth + laneWidth/2
	
	// Determine traffic speed (slightly slower than speed limit for realism)
	speedLimitMPH := 50.0 + float64(lane)*10.0
	// Traffic drives at 80-95% of speed limit
	trafficSpeedMPH := speedLimitMPH * (0.8 + rand.Float64()*0.15)
	trafficVelocityY := trafficSpeedMPH / MPHPerPixelPerFrame
	
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
		X:         laneCenterX,
		Y:         spawnY,
		VelocityY: trafficVelocityY,
		Lane:      lane,
		Color:     carColor,
	}
	
	gs.traffic = append(gs.traffic, newTraffic)
}

// drawTraffic renders all traffic vehicles
func (gs *GameplayScreen) drawTraffic(screen *ebiten.Image) {
	carWidth, carHeight := 40, 64
	
	for _, tc := range gs.traffic {
		// Calculate screen position relative to player car
		screenY := tc.Y - gs.playerCar.Y + float64(gs.screenHeight)/2
		
		// Only draw if on screen
		if screenY < -100 || screenY > float64(gs.screenHeight)+100 {
			continue
		}
		
		// Calculate screen X position (convert world X to screen X with camera offset)
		screenX := tc.X - gs.cameraX - float64(carWidth)/2
		
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
