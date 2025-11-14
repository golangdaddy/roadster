package game

import (
	"image/color"

	"github.com/golangdaddy/roadster/car"
	"github.com/golangdaddy/roadster/models"
	"github.com/golangdaddy/roadster/road"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// RoadView represents the main driving view
type RoadView struct {
	gameState *models.GameState
	road      *road.Road
	
	// Car position and state (in world coordinates)
	carX      float64 // X position in world (horizontal - lane position)
	carY      float64 // Y position in world (vertical - distance traveled)
	carAngle  float64 // Car angle in degrees (0 = facing up/north)
	carSpeed  float64 // Current speed in pixels per frame
	carLane   int     // Current lane (0-indexed)
	
	// Camera - fixed above car, doesn't rotate
	cameraX float64 // Camera X position in world space (centered on car)
	cameraY float64 // Camera Y position in world space (follows car)
}

// NewRoadView creates a new road view
func NewRoadView(gameState *models.GameState) *RoadView {
	// Create a 4-lane highway
	highway := road.NewRoad(4, 80.0) // 4 lanes, 80 pixels wide each
	
	startLane := 1 // Start in second lane (0-indexed, so lane 2)
	
	// Car starts at world position (0, 0)
	// Lane centers are relative to road center at world X = 0
	startLaneWorldX := float64(startLane)*highway.LaneWidth - highway.RoadWidth/2 + highway.LaneWidth/2
	
	return &RoadView{
		gameState: gameState,
		road:      highway,
		carX:      startLaneWorldX, // Start in center of lane (world X coordinate)
		carY:      0,   // Start at beginning of road (world Y = 0)
		carAngle:  0,   // Facing straight up
		carSpeed:  0,   // Stationary
		carLane:   startLane,
		cameraX:   startLaneWorldX, // Camera starts at car position
		cameraY:   0,
	}
}

// Update handles input and updates game state
func (rv *RoadView) Update() error {
	// Handle car movement
	acceleration := 0.3
	maxSpeed := 8.0
	turnSpeed := 3.0
	
	// Accelerate/decelerate
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		rv.carSpeed += acceleration
		if rv.carSpeed > maxSpeed {
			rv.carSpeed = maxSpeed
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		rv.carSpeed -= acceleration
		if rv.carSpeed < -maxSpeed/2 {
			rv.carSpeed = -maxSpeed / 2
		}
	}
	
	// Natural deceleration
	if !ebiten.IsKeyPressed(ebiten.KeyArrowUp) && !ebiten.IsKeyPressed(ebiten.KeyW) &&
		!ebiten.IsKeyPressed(ebiten.KeyArrowDown) && !ebiten.IsKeyPressed(ebiten.KeyS) {
		if rv.carSpeed > 0 {
			rv.carSpeed -= acceleration * 0.5
			if rv.carSpeed < 0 {
				rv.carSpeed = 0
			}
		} else if rv.carSpeed < 0 {
			rv.carSpeed += acceleration * 0.5
			if rv.carSpeed > 0 {
				rv.carSpeed = 0
			}
		}
	}
	
	// Lane changing - calculate target lane center in world coordinates
	// Road center is at world X = 0, lanes are relative to that
	targetLaneWorldX := float64(rv.carLane)*rv.road.LaneWidth - rv.road.RoadWidth/2 + rv.road.LaneWidth/2
	
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if rv.carLane < rv.road.NumLanes-1 {
			rv.carLane++ // Move to right lane (higher index)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if rv.carLane > 0 {
			rv.carLane-- // Move to left lane (lower index)
		}
	}
	
	// Smoothly move car to target lane (in world coordinates)
	diff := targetLaneWorldX - rv.carX
	if diff > turnSpeed {
		rv.carX += turnSpeed // Moving right (increasing X)
		rv.carAngle = -5 // Tilt left (inverted animation)
	} else if diff < -turnSpeed {
		rv.carX -= turnSpeed // Moving left (decreasing X)
		rv.carAngle = 5 // Tilt right (inverted animation)
	} else {
		rv.carX = targetLaneWorldX
		rv.carAngle = 0 // Straight
	}
	
	// Update car Y position (distance traveled upward)
	// Car moves upward, so we increase carY (positive Y is up in world space)
	rv.carY += rv.carSpeed
	
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
}

