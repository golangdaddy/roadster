package game

import (
	"fmt"
	"image/color"
	"math"

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
	scrollSpeed  float64
	roadTextures map[string]*ebiten.Image
	screenWidth  int
	screenHeight int
	cameraX      float64 // Camera X offset to follow car
	onGameEnd    func() // Callback when game ends
}

// NewGameplayScreen creates a new gameplay screen
func NewGameplayScreen(selectedCar *car.Car, levelData *LevelData, onGameEnd func()) *GameplayScreen {
	gs := &GameplayScreen{
		roadSegments: make([]RoadSegment, 0),
		scrollSpeed:  2.0,
		roadTextures: make(map[string]*ebiten.Image),
		screenWidth:  1024,
		screenHeight: 600,
		onGameEnd:    onGameEnd,
	}

	// Initialize player car
	laneWidth := 80.0
	gs.playerCar = &Car{
		X:                laneWidth / 2, // Start in center of starting lane (world X = 40)
		Y:                float64(gs.screenHeight) - 100,
		Speed:            0,
		VelocityX:        0,
		VelocityY:        0,
		SteeringAngle:    0,
		Acceleration:     0.5,
		TurnSpeed:        3.0,  // Horizontal speed when turning
		SteeringResponse: 0.15, // How fast steering returns to center
		SelectedCar:      selectedCar,
	}

	// Load road textures
	gs.loadRoadTextures()

	// Generate road from level data
	gs.generateRoadFromLevel(levelData)

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

	// Handle acceleration/deceleration (Up/Down arrow keys)
	maxSpeed := 8.0
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
		// Gradually return to default speed
		defaultSpeed := 4.0
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

	// Apply steering to horizontal velocity
	// Steering effectiveness increases with speed
	speedFactor := gs.playerCar.VelocityY / maxSpeed
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

	// Remove segments that have scrolled off the top of screen
	if len(gs.roadSegments) > 0 && gs.roadSegments[0].Y > float64(gs.screenHeight)+100 {
		gs.roadSegments = gs.roadSegments[1:]
	}

	// Add new segments at the bottom if needed (simple infinite road generation)
	if len(gs.roadSegments) < 20 {
		lastY := gs.roadSegments[len(gs.roadSegments)-1].Y
		newSegment := RoadSegment{
			LaneCount:      4,                             // Default to 4 lanes
			RoadTypes:      []string{"A", "A", "A", "A"}, // All lanes type A
			StartLaneIndex: 0,                             // Starting lane at leftmost
			Y:              lastY - 600,                   // Add above current segments (600px segment height)
		}
		// Insert at beginning to maintain order
		gs.roadSegments = append([]RoadSegment{newSegment}, gs.roadSegments...)
	}

	return nil
}

// Draw renders the gameplay screen
func (gs *GameplayScreen) Draw(screen *ebiten.Image) {
	// Clear screen with sky color (retro blue)
	screen.Fill(color.RGBA{135, 206, 235, 255}) // Sky blue

	// Draw road segments
	gs.drawRoad(screen)

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

	// Draw grass background for entire segment width first
	grassImg := ebiten.NewImage(gs.screenWidth, 600)
	grassImg.Fill(color.RGBA{34, 139, 34, 255}) // Forest green grass

	// Add some grass texture variation
	for x := 0; x < gs.screenWidth; x++ {
		for y := 0; y < 600; y += 4 {
			if (x+y)%7 == 0 {
				grassImg.Set(x, y, color.RGBA{50, 160, 50, 255}) // Slightly lighter green
			}
		}
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, screenY)
	screen.DrawImage(grassImg, op)

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

// drawCar renders the player's car
func (gs *GameplayScreen) drawCar(screen *ebiten.Image) {
	carWidth, carHeight := 40, 64

	// Car position on screen (convert world X to screen X with camera offset)
	screenX := gs.playerCar.X - gs.cameraX - float64(carWidth)/2
	screenY := float64(gs.screenHeight) - 150 // Fixed position near bottom

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
	
	// Position in top-left corner
	x := 20.0
	y := 20.0
	width := 180.0
	height := 120.0
	
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
	
	// Draw simple speed gauge bar
	gs.drawSpeedGauge(screen, x+10, y+height-25, width-20, 15, speedMPH)
}

// drawSpeedGauge draws a simple horizontal gauge bar showing speed
func (gs *GameplayScreen) drawSpeedGauge(screen *ebiten.Image, x, y, width, height float64, speedMPH float64) {
	maxSpeed := 100.0 // Maximum speed for gauge (100 MPH)
	speedPercent := math.Min(speedMPH/maxSpeed, 1.0)
	
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
}
