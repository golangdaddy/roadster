package lanecontroller

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// TrafficCar represents a traffic vehicle in a lane
type TrafficCar struct {
	X            float64     // World X position (center of lane)
	Y            float64     // World Y position
	Lane         int         // Lane index (0-based)
	Speed        float64     // Speed in pixels per frame
	Color        color.Color // Car color
	ID           int64       // Unique identifier for tracking passed status
	FuelLevel    float64     // Current fuel level (0.0 to 1.0)
	FuelCapacity float64     // Fuel tank capacity in liters
}

// LaneController manages a single lane's sprite and cars
type LaneController struct {
	LaneIndex      int           // Lane index (0 = layby, 1+ = normal lanes)
	SpriteTile     *ebiten.Image // Current sprite tile for this lane
	WorldX         float64       // World X position of this lane (center)
	WorldYStart    float64       // World Y where this lane starts
	WorldYEnd      float64       // World Y where this lane ends
	HasLayby       bool          // Whether this is a layby lane
	HasOnRamp      bool          // Whether this lane has an on-ramp sprite
	HasOffRamp     bool          // Whether this lane has an off-ramp sprite
	CurrentSpriteType string     // Current sprite type: "normal", "onramp", "offramp", "layby"
	
	// Traffic cars in this lane
	TrafficCars []TrafficCar
}

// NewLaneController creates a new lane controller
func NewLaneController(laneIndex int, worldX, worldYStart, worldYEnd float64, spriteType string) *LaneController {
	lc := &LaneController{
		LaneIndex:   laneIndex,
		WorldX:      worldX,
		WorldYStart: worldYStart,
		WorldYEnd:   worldYEnd,
		CurrentSpriteType: spriteType,
		TrafficCars: []TrafficCar{},
	}
	
	// Load sprite based on type
	lc.loadSprite(spriteType)
	
	return lc
}

// loadSprite loads the appropriate sprite for this lane
// Note: This does NOT change HasLayby flag - that is set during creation and should not change
func (lc *LaneController) loadSprite(spriteType string) {
	var spritePath string
	switch spriteType {
	case "onramp":
		spritePath = "assets/road/onramp.png"
		lc.HasOnRamp = true
		lc.HasOffRamp = false
		// Don't change HasLayby - it's set during creation
	case "offramp":
		spritePath = "assets/road/offramp.png"
		lc.HasOnRamp = false
		lc.HasOffRamp = true
		// Don't change HasLayby - it's set during creation
	case "layby":
		spritePath = "assets/road/layby.png"
		lc.HasOnRamp = false
		lc.HasOffRamp = false
		// Only set HasLayby if this lane controller was created as a layby lane
		// Don't overwrite it if it's already false (for normal lanes)
		if lc.HasLayby {
			// This is the layby lane controller, keep HasLayby true
		}
	default:
		spritePath = "assets/road/normal.png"
		lc.HasOnRamp = false
		lc.HasOffRamp = false
		// Don't change HasLayby - it's set during creation
	}
	
	img, _, err := ebitenutil.NewImageFromFile(spritePath)
	if err != nil {
		// Sprite not found, will be nil
		fmt.Printf("Warning: Could not load sprite '%s': %v\n", spritePath, err)
		return
	}
	lc.SpriteTile = img
	lc.CurrentSpriteType = spriteType
	fmt.Printf("Loaded sprite '%s' for lane %d (HasLayby=%v): %dx%d\n", spriteType, lc.LaneIndex, lc.HasLayby, img.Bounds().Dx(), img.Bounds().Dy())
}

// UpdateSprite updates the sprite for the next road segment
func (lc *LaneController) UpdateSprite(spriteType string) {
	if lc.CurrentSpriteType != spriteType {
		lc.loadSprite(spriteType)
	}
}

// GetSpriteTypeForSegment determines what sprite type this lane should use for a given segment
// Returns the sprite type this lane should render based on segment properties
func (lc *LaneController) GetSpriteTypeForSegment(hasPetrolStationLane bool, segmentTileType string) string {
	// Only the layby lane (identified by HasLayby flag) gets layby sprite
	if hasPetrolStationLane && lc.HasLayby {
		return "layby"
	}
	
	// For normal lanes, use the segment's tile type (normal/onramp/offramp)
	// But never use "layby" for normal lanes
	if segmentTileType == "layby" {
		return "normal"
	}
	
	return segmentTileType
}

// Draw draws this lane's sprite at the given screen position
func (lc *LaneController) Draw(screen *ebiten.Image, screenX, screenY float64, laneWidth float64) {
	if lc.SpriteTile == nil {
		return
	}
	
	// Tile horizontally to fill lane width
	tileWidth := float64(lc.SpriteTile.Bounds().Dx())
	currentX := screenX
	for currentX < screenX + laneWidth {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(currentX, screenY)
		screen.DrawImage(lc.SpriteTile, op)
		currentX += tileWidth
	}
}

// IsVisible checks if this lane is visible in the given world Y range
func (lc *LaneController) IsVisible(worldYStart, worldYEnd float64) bool {
	return !(lc.WorldYEnd < worldYStart || lc.WorldYStart > worldYEnd)
}

// AddTrafficCar adds a traffic car to this lane
func (lc *LaneController) AddTrafficCar(car TrafficCar) {
	lc.TrafficCars = append(lc.TrafficCars, car)
}

// UpdateTrafficCars updates all traffic cars in this lane
func (lc *LaneController) UpdateTrafficCars(baseSpeedLimitMPH, speedPerLaneMPH, pxPerFramePerMPH float64, laneWidth float64) {
	var activeTraffic []TrafficCar
	
	for i := range lc.TrafficCars {
		tc := &lc.TrafficCars[i]
		
		// Consume fuel based on distance traveled
		if tc.FuelLevel > 0 && tc.FuelCapacity > 0 {
			const pixelsPerLiter = 60000.0 // 1 liter per 60,000 pixels
			litersConsumed := tc.Speed / pixelsPerLiter
			speedMultiplier := 1.0 + (float64(lc.LaneIndex) * 0.2)
			litersConsumed *= speedMultiplier
			fuelConsumed := litersConsumed / tc.FuelCapacity
			tc.FuelLevel -= fuelConsumed
			if tc.FuelLevel < 0 {
				tc.FuelLevel = 0
			}
		}
		
		// Calculate speed limit for this lane
		speedLimitMPH := baseSpeedLimitMPH + (float64(lc.LaneIndex) * speedPerLaneMPH)
		if lc.HasLayby && lc.LaneIndex == 0 {
			speedLimitMPH = 40.0 // Layby is 40mph
		}
		
		// Traffic cars move 5mph slower than the lane speed limit
		trafficSpeedMPH := speedLimitMPH - 5.0
		if tc.FuelLevel <= 0 {
			trafficSpeedMPH = 0 // Out of fuel - stop
		}
		if trafficSpeedMPH < 0 {
			trafficSpeedMPH = 0
		}
		trafficSpeedPxPerFrame := trafficSpeedMPH * pxPerFramePerMPH
		
		tc.Speed = trafficSpeedPxPerFrame
		
		// Update position
		if tc.FuelLevel > 0 {
			tc.Y += tc.Speed
		}
		
		// Keep car centered in lane
		tc.X = lc.WorldX
		
		// Check if car is still in this lane's Y range
		if tc.Y >= lc.WorldYStart && tc.Y < lc.WorldYEnd {
			activeTraffic = append(activeTraffic, *tc)
		}
	}
	
	lc.TrafficCars = activeTraffic
}

// GetTrafficCars returns all traffic cars in this lane
func (lc *LaneController) GetTrafficCars() []TrafficCar {
	return lc.TrafficCars
}
