package game

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/golangdaddy/roadster/pkg/models/car"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	trafficSpawnRange      = 1600.0 // Range ahead/behind player to spawn traffic (decreased from 6000)
	trafficSpawnProbability = 0.105 // Chance to spawn a car for a lane/direction (30% reduction from 0.15)
)

// TrafficCar represents a traffic vehicle
type TrafficCar struct {
	X, Y         float64    // World position
	VelocityY    float64    // Vertical velocity (current speed)
	TargetSpeed  float64    // Desired speed (based on speed limit)
	Acceleration float64    // Acceleration rate
	Deceleration float64    // Deceleration/Braking rate
	Lane         int        // Which lane this car is in
	TargetLane   int        // Lane moving towards (if changing)
	LaneProgress float64    // 0.0 to 1.0 for visual transition
	Color        color.RGBA // Car color for variety
	LastLaneChangeTime int64 // Timestamp of last lane change
	Passed       bool // Whether the player has passed this car
}

// PlayerPed represents the human character when on foot
type PlayerPed struct {
	X, Y      float64
	Speed     float64
	Sprite    *ebiten.Image
}

// Update runs the AI logic to maintain safe distance
func (tc *TrafficCar) Update(gs *GameplayScreen) {
	// Safety check: skip AI updates if coordinates are extreme to prevent panics
	if math.Abs(tc.Y) > 100000 || math.Abs(gs.playerCar.Y) > 100000 {
		return
	}

	// Check for pedestrian
	if gs.onFoot && gs.playerPed != nil {
		dist := math.Hypot(tc.X-gs.playerPed.X, tc.Y-gs.playerPed.Y)
		if dist < 200 {
			tc.TargetSpeed = 0
			return
		}
	}

	// Find closest car ahead in same lane AND check for faster cars behind
	minDist := 10000.0
	foundCarAhead := false
	speedOfCarAhead := 0.0
	
	minDistBehind := 10000.0
	foundCarBehind := false
	speedOfCarBehind := 0.0
	
	rightLaneBlocked := false
	leftLaneBlocked := false

	// Get segment info early for logic
	tcSegment := gs.getSegmentAt(tc.Y)

	// Anti-deadlock: If speed is very low for too long, force a resolution
	// If car is basically stopped (Velocity < 1.0)
	if tc.VelocityY < 1.0 {
		// If stuck for more than 3 seconds (assuming 60fps, simple counter approach needed or timestamp)
		// Simplified approach: if stopped and blocked, try desperate maneuvers
		
		// If blocked ahead, try to force a lane change even if risky
		if foundCarAhead && minDist < minTrafficDistance {
			// Try ANY lane
			if tc.Lane+1 < tcSegment.LaneCount && !rightLaneBlocked {
				tc.TargetLane = tc.Lane + 1
				tc.LaneProgress = 0.01
				return
			}
			if tc.Lane > 1 && !leftLaneBlocked {
				tc.TargetLane = tc.Lane - 1
				tc.LaneProgress = 0.01
				return
			}
			
			// If completely stuck (blocked ahead and sides), gradually despawn if off-screen or far behind player
			// Or just ghost through if really stuck?
			// Let's just aggressively reduce collision box for movement if stuck
		}
	}

	// Check against other traffic - cars are now aware of ALL nearby cars
	for _, other := range gs.traffic {
		if other == tc {
			continue
		}

		// Check cars in same lane
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
			// Check if other car is behind (Y is larger)
			if other.Y > tc.Y {
				dist := other.Y - tc.Y
				if dist < minDistBehind {
					minDistBehind = dist
					foundCarBehind = true
					speedOfCarBehind = other.VelocityY
				}
			}
		}

		// Check if right lane is blocked (for lane change)
		// Include cars transitioning to/from the target lane
		if other.Lane == tc.Lane+1 || (other.TargetLane == tc.Lane+1 && other.LaneProgress > 0.3) {
			if math.Abs(tc.Y-other.Y) < minTrafficDistance*1.5 {
				rightLaneBlocked = true
			}
		}
		// Check if left lane is blocked
		if other.Lane == tc.Lane-1 || (other.TargetLane == tc.Lane-1 && other.LaneProgress > 0.3) {
			if math.Abs(tc.Y-other.Y) < minTrafficDistance*1.5 {
				leftLaneBlocked = true
			}
		}

		// Also check cars in adjacent lanes that might affect our decision
		// This makes cars more aware of their surroundings
		if other.Lane == tc.Lane+1 || other.Lane == tc.Lane-1 {
			// If a car in adjacent lane is very close, be more cautious about lane changes
			if math.Abs(tc.Y-other.Y) < minTrafficDistance*0.8 {
				if other.Lane == tc.Lane+1 {
					rightLaneBlocked = true
				}
				if other.Lane == tc.Lane-1 {
					leftLaneBlocked = true
				}
			}
		}
	}

	// Check against player
	laneWidth := 80.0
	
	// Check if player blocks adjacent lanes for lane changing
	// Calculate player's lane index relative to traffic's current segment
	// tcSegment is already calculated above
	segLeftEdge := -float64(tcSegment.StartLaneIndex) * laneWidth
	playerLane := int((gs.playerCar.X - segLeftEdge) / laneWidth)
	
	// Check if player is close enough to block a lane change
	if math.Abs(tc.Y - gs.playerCar.Y) < minTrafficDistance*2.0 {
		if playerLane == tc.Lane + 1 {
			rightLaneBlocked = true
		}
		if playerLane == tc.Lane - 1 {
			leftLaneBlocked = true
		}
	}

	// Simple lane check based on X distance
	if math.Abs(gs.playerCar.X - tc.X) < laneWidth/2 {
		// Player is in roughly the same lane
		if gs.playerCar.Y < tc.Y {
			// Player ahead of this traffic car
			dist := tc.Y - gs.playerCar.Y
			if dist < minDist {
				minDist = dist
				foundCarAhead = true
				speedOfCarAhead = gs.playerCar.VelocityY
			}
		} else {
			// Player behind this traffic car
			dist := gs.playerCar.Y - tc.Y
			if dist < minDistBehind {
				minDistBehind = dist
				foundCarBehind = true
				speedOfCarBehind = gs.playerCar.VelocityY
			}
		}
	}
	// Check player in right lane
	// Assuming player X logic: lane 0 is around 40?
	// If player X is in right lane range
	// This is tricky without segment/lane math, but we can approximate or skip player check for lane change for now.

	// Adjust speed based on distance
	// Determine base target speed based on lane (restore to limit if no car ahead)
	lanePosition := tc.Lane
	if tc.Lane < len(tcSegment.LanePositions) {
		lanePosition = tcSegment.LanePositions[tc.Lane]
	}
	speedLimitMPH := 50.0 + float64(lanePosition)*10.0
	baseTargetSpeed := speedLimitMPH / MPHPerPixelPerFrame

	// Default to base target speed
	tc.TargetSpeed = baseTargetSpeed

	// Initialize move over flag
	shouldMoveOver := false

	safeDistance := minTrafficDistance * 1.5
	if foundCarAhead && minDist < safeDistance {
		// Match the car ahead
		tc.TargetSpeed = speedOfCarAhead
		
		// If the car ahead is moving VERY slowly or we are too close, brake harder
		if minDist < minTrafficDistance {
			tc.TargetSpeed = speedOfCarAhead * 0.85
		}
		if minDist < minTrafficDistance*0.5 {
			tc.TargetSpeed = speedOfCarAhead * 0.6
		}
		
		// AGGRESSIVE OVERTAKING: If stuck behind a slower car, increase urge to change lanes
		// Especially if we are in a fast lane
		if tc.Lane > 1 && speedOfCarAhead < baseTargetSpeed * 0.8 {
			// Force a lane change attempt (ignore random chance)
			shouldMoveOver = true
		}
	}

	// VIGILANT: If a faster car is approaching from behind, slow down slightly to help them pass
	if foundCarBehind && minDistBehind < 300 && speedOfCarBehind > tc.VelocityY * 1.2 {
		// Reduce speed by 10% to facilitate overtaking
		tc.TargetSpeed = tc.VelocityY * 0.9
	}

	// Apply Physics (harmonised with player AI)
	if math.Abs(tc.VelocityY - tc.TargetSpeed) < 0.1 {
		tc.VelocityY = tc.TargetSpeed
	} else if tc.VelocityY < tc.TargetSpeed {
		// Accelerate
		// BOOST acceleration if significantly under target speed to reach it faster
		acceleration := tc.Acceleration
		if tc.TargetSpeed - tc.VelocityY > 2.0 { // More than ~20mph difference
			acceleration *= 2.0 // Double acceleration to catch up
		}
		
		tc.VelocityY += acceleration
		if tc.VelocityY > tc.TargetSpeed {
			tc.VelocityY = tc.TargetSpeed
		}
	} else if tc.VelocityY > tc.TargetSpeed {
		// Decelerate/Brake
		
		// SAFETY CHECK: Don't brake if we are changing lanes to a faster lane
		// This prevents cars from slowing down right as they enter a fast lane, causing collisions
		isChangingToFasterLane := tc.LaneProgress > 0 && tc.TargetLane > tc.Lane
		
		if !isChangingToFasterLane {
			// Use Deceleration rate, boost if we need to brake hard (target is much lower)
			brakeForce := tc.Deceleration
			if tc.TargetSpeed < tc.VelocityY * 0.5 {
				brakeForce *= 2.0 // Emergency braking
			}
			
			tc.VelocityY -= brakeForce
			if tc.VelocityY < tc.TargetSpeed {
				tc.VelocityY = tc.TargetSpeed
			}
		}
	}
	
	// Ensure non-negative speed
	if tc.VelocityY < 0 {
		tc.VelocityY = 0
	}
	
	// VIGILANT LANE CHANGE: Move out of the way for faster cars approaching from behind
	// shouldMoveOver is already initialized above
	if foundCarBehind && minDistBehind < 400 {
		// A car is approaching from behind
		// Check if it's significantly faster (more than 20% faster)
		if speedOfCarBehind > tc.VelocityY * 1.2 {
			shouldMoveOver = true
		}
	}

	// PRIORITY: Cars driving 20mph+ under lane speed limit should move over
	currentSpeedMPH := tc.VelocityY * MPHPerPixelPerFrame
	laneSpeedLimitMPH := 50.0 + float64(lanePosition)*10.0
	// Use the ACTUAL lane speed limit for comparison, not the "-5" target
	shouldMoveOverSlow := false
	if currentSpeedMPH < (laneSpeedLimitMPH - 20.0) {
		shouldMoveOverSlow = true
	}

	// Attempt lane change
	if tc.LaneProgress == 0 && tc.TargetLane == 0 {
		// Cooldown check (10 seconds)
		now := time.Now().UnixMilli()
		if now-tc.LastLaneChangeTime < 10000 {
			return
		}

		segment := gs.getSegmentAt(tc.Y)

		// HIGHEST PRIORITY: Move over if driving 20mph+ under speed limit
		// But never move to lane 0 - lane 1 is the minimum for traffic
		if shouldMoveOverSlow && tc.Lane > 1 {
			// Move to a slower lane (left) - this is mandatory for slow drivers
			canLeft := !leftLaneBlocked
			if canLeft {
				tc.TargetLane = tc.Lane - 1
				tc.LaneProgress = 0.01 // Start transition
				return
			}
		}

		// Priority: Move over for faster cars OR if we want to overtake
		// But never move to lane 0 - lane 1 is the minimum for traffic
		if shouldMoveOver {
			// If we are the slow one blocking, move left
			// If we are stuck behind a slow one, move right (overtake)
			
			// Overtake logic (Move Right)
			if foundCarAhead && tc.Lane+1 < segment.LaneCount {
				canRight := !rightLaneBlocked
				if canRight {
					tc.TargetLane = tc.Lane + 1
					tc.LaneProgress = 0.01
					return
				}
			}
			
			// Move over logic (Move Left) - only if we aren't trying to overtake
			if !foundCarAhead && tc.Lane > 1 {
				canLeft := !leftLaneBlocked
				if canLeft {
					tc.TargetLane = tc.Lane - 1
					tc.LaneProgress = 0.01 // Start transition
					return
				}
			}
		}

		// LIFECYCLE: Keep Right / Move to Slower Lane
		// After 20 seconds (cooldown), try to move to a slower lane (Left, Lane-1)
		// Only allow moving to faster lanes (Right) if stuck or evading
		
		// Always try to move left (slower) if possible and safe
		if tc.Lane > 1 {
			// Check if the slower lane (Left, Lane-1) is clear
			canLeft := !leftLaneBlocked
			
			// If clear, take it!
			if canLeft {
				// 5% chance per frame to actually initiate the move (makes it feel natural but persistent)
				if rand.Float64() < 0.05 {
					tc.TargetLane = tc.Lane - 1
					tc.LaneProgress = 0.01
					return
				}
			}
		}

		// Overtaking (Right/Faster Lane) - ONLY if stuck behind a slow car
		if foundCarAhead && tc.Lane+1 < segment.LaneCount {
			// Only overtake if the car ahead is significantly slower
			if speedOfCarAhead < tc.TargetSpeed * 0.9 {
				canRight := !rightLaneBlocked
				if canRight {
					// 2% chance to overtake (reluctant to move to fast lane)
					if rand.Float64() < 0.02 {
						tc.TargetLane = tc.Lane + 1
						tc.LaneProgress = 0.01
						return
					}
				}
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

type PetrolStation struct {
	X, Y float64
	Lane int
}

// GameplayScreen represents the main driving gameplay
type GameplayScreen struct {
	roadSegments []RoadSegment
	petrolStations []PetrolStation
	playerCar    *Car
	traffic      []*TrafficCar // Traffic vehicles
	trafficMutex sync.RWMutex  // Mutex for safe concurrent access to traffic
	scrollSpeed  float64
	roadTextures map[string]*ebiten.Image
	screenWidth  int
	screenHeight int
	cameraX      float64 // Camera X offset to follow car
	cameraY      float64 // Camera Y offset to follow target
	onGameEnd    func() // Callback when game ends
	levelData    *LevelData // Store level data for reset
	initialX     float64   // Initial player X position
	initialY     float64   // Initial player Y position
	backgroundPattern *ebiten.Image // Repeating background pattern
	lastSpawnTime int64 // Timestamp of last spawn attempt
	spawnCooldown int64 // Minimum time between spawn attempts (in milliseconds)
	DistanceTravelled float64 // Total miles travelled
	TotalCarsPassed   int     // Total number of cars passed
	Level             int     // Current player level
	LevelThreshold    int     // Total cars needed to reach next level
	PrevLevelThreshold int    // Total cars needed to reach current level (for progress bar)
	paused            bool
	onFoot            bool
	playerPed         *PlayerPed
	autoDrive         bool  // Auto-pilot mode
	autoDriveLane     int   // Target lane for auto-pilot
	lastAutoDriveLaneChange int64 // Timestamp of last auto-drive lane change
}

// NewGameplayScreen creates a new gameplay screen
func NewGameplayScreen(selectedCar *car.Car, levelData *LevelData, onGameEnd func()) *GameplayScreen {
	gs := &GameplayScreen{
		roadSegments: make([]RoadSegment, 0),
		petrolStations: make([]PetrolStation, 0),
		traffic:      make([]*TrafficCar, 0),
		scrollSpeed:  2.0,
		roadTextures: make(map[string]*ebiten.Image),
		screenWidth:  1024,
		screenHeight: 600,
		onGameEnd:    onGameEnd,
		lastSpawnTime: time.Now().UnixMilli(),
		spawnCooldown: 215 + rand.Int63n(143), // 215-358ms random cooldown (30% reduction in spawn frequency)
		DistanceTravelled: 0,
		TotalCarsPassed:   0,
		Level:             1,
		LevelThreshold:    172, // Start with 172 cars to level up (matches config)
		PrevLevelThreshold: 0,
	}

	// Initialize player car
	laneWidth := 80.0
	initialX := laneWidth / 2 // Start in center of starting lane (world X = 40)
	initialY := float64(gs.screenHeight) - 100
	
	gs.cameraX = initialX - float64(gs.screenWidth)/2
	gs.cameraY = initialY - float64(gs.screenHeight)/2
	
	gs.playerCar = &Car{
		X:                initialX,
		Y:                initialY,
		Speed:            0,
		VelocityX:        0,
		VelocityY:        0,
		SteeringAngle:    0,
		Acceleration:     0.05, // Reduced for more gradual speed transitions
		TurnSpeed:        6.0,  // Higher target speed to compensate for inertia
		SteeringResponse: 0.05, // Smoother steering return
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
		
		// Check for Petrol Station (Road Type F)
		laneWidth := 80.0
		for laneIdx, rt := range roadTypes {
			if rt == "F" {
				leftEdge := -float64(startLaneIdx) * laneWidth
				laneX := leftEdge + float64(laneIdx)*laneWidth + laneWidth/2
				
				station := PetrolStation{
					X:    laneX - 100,
					Y:    y - segmentHeight/2,
					Lane: laneIdx,
				}
				gs.petrolStations = append(gs.petrolStations, station)
			}
		}

		gs.roadSegments = append(gs.roadSegments, roadSegment)
		y -= segmentHeight // Segments go upward
	}
}

// isLaneClear checks if a lane is safe to enter
func (gs *GameplayScreen) isLaneClear(laneIdx int, segment RoadSegment, laneWidth float64) bool {
	// Check bounds
	if laneIdx < 0 || laneIdx >= segment.LaneCount {
		return false
	}

	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	laneCenterX := leftEdge + float64(laneIdx)*laneWidth + laneWidth/2

	gs.trafficMutex.RLock()
	defer gs.trafficMutex.RUnlock()

	for _, tc := range gs.traffic {
		// Check lateral overlap (simplified)
		if math.Abs(tc.X-laneCenterX) < laneWidth/2 {
			// Check longitudinal distance - very lenient for lane 0 (right lane domination)
			dist := tc.Y - gs.playerCar.Y // Positive = ahead, negative = behind

			// Rightmost lane gets special treatment - be more aggressive
			rightmostLane := segment.LaneCount - 1
			if laneIdx == rightmostLane {
				// Only need minimal clearance in rightmost lane
				if dist > -50 && dist < 100 { // 100px ahead, 50px behind
					return false
				}
			} else {
				// Other lanes need more clearance
				if dist > -100 && dist < 200 { // 200px ahead, 100px behind
					return false
				}
			}
		}
	}
	return true
}

// updateAutoPilot controls the car autonomously
func (gs *GameplayScreen) updateAutoPilot(currentSegment RoadSegment, segmentIdx int, laneWidth float64, maxSpeed float64) {
	laneChanged := false

	// 1. Determine target lane position (and interpolated bounds)
	startLeftEdge := -float64(currentSegment.StartLaneIndex) * laneWidth
	// Default bounds (full segment width)
	boundRight := startLeftEdge + float64(currentSegment.LaneCount)*laneWidth
	boundLeft := startLeftEdge

	// Interpolate bounds to match physics (Check availability of space)
	if segmentIdx < len(gs.roadSegments)-1 {
		nextSegment := gs.roadSegments[segmentIdx+1]
		nextLeft := -float64(nextSegment.StartLaneIndex) * laneWidth
		nextRight := nextLeft + float64(nextSegment.LaneCount)*laneWidth

		progress := (currentSegment.Y - gs.playerCar.Y) / 600.0
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}

		boundRight = boundRight + (nextRight-boundRight)*progress
		boundLeft = boundLeft + (nextLeft-boundLeft)*progress
	}

	targetLaneX := startLeftEdge + float64(gs.autoDriveLane)*laneWidth + laneWidth/2

	// 2. Check for obstacles in current target lane
	collisionRisk := false
	minDist := 800.0 // Look ahead distance for awareness
	closeObstacleDist := 400.0 // Distance that actually requires speed reduction

	gs.trafficMutex.RLock()
	for _, tc := range gs.traffic {
		// Check if traffic is in our intended lane
		if math.Abs(tc.X-targetLaneX) < laneWidth/2 {
			// Check if ahead (PlayerY > TrafficY)
			if tc.Y < gs.playerCar.Y && tc.Y > gs.playerCar.Y-minDist {
				dist := gs.playerCar.Y - tc.Y
				if dist < minDist {
					minDist = dist
				}
				// Only consider it a collision risk if it's close enough to affect speed
				if dist < closeObstacleDist {
					collisionRisk = true
				}
			}
		}
	}
	gs.trafficMutex.RUnlock()

	// Helper to check physical availability of lane
	checkAvailability := func(laneIdx int) bool {
		laneCenterX := startLeftEdge + float64(laneIdx)*laneWidth + laneWidth/2
		// Strict margin: center must be inside bounds by half lane width
		if laneCenterX > boundRight-40 {
			return false
		}
		if laneCenterX < boundLeft+40 {
			return false
		}
		return true
	}

	// 2.5 Emergency Wall Avoidance (Prioritize over traffic)
	if !checkAvailability(gs.autoDriveLane) {
		laneCenterX := startLeftEdge + float64(gs.autoDriveLane)*laneWidth + laneWidth/2
		if laneCenterX > boundRight-40 && gs.autoDriveLane > 0 {
			gs.autoDriveLane--
			laneChanged = true
		} else if laneCenterX < boundLeft+40 && gs.autoDriveLane < currentSegment.LaneCount-1 {
			gs.autoDriveLane++
			laneChanged = true
		}
	}

	// 3. Lane Change Logic - DOMINATE THE RIGHT LANE (Rightmost lane)
	// Only change if we aren't already changing (aligned with lane)
	if math.Abs(gs.playerCar.X-targetLaneX) < 20 {
		// Check minimum lane hold time (2 seconds)
		now := time.Now().UnixMilli()
		canChangeLanes := (now - gs.lastAutoDriveLaneChange) >= 2000

		// Primary goal: Stay in rightmost lane (fast lane) at all costs
		rightmostLane := currentSegment.LaneCount - 1

		// If we're NOT in the rightmost lane, aggressively try to get back there
		if canChangeLanes && gs.autoDriveLane < rightmostLane {
			if checkAvailability(rightmostLane) && gs.isLaneClear(rightmostLane, currentSegment, laneWidth) {
				gs.autoDriveLane = rightmostLane
				laneChanged = true
				gs.lastAutoDriveLaneChange = now
			}
		}

		// Emergency evasion ONLY if in immediate danger and can't stay in rightmost lane
		// Emergency overrides the 2-second rule
		if !laneChanged && collisionRisk && minDist < 200 {
			// Move left (to a slower lane) as emergency measure
			if gs.autoDriveLane == rightmostLane && gs.autoDriveLane > 0 &&
			   checkAvailability(gs.autoDriveLane-1) && gs.isLaneClear(gs.autoDriveLane-1, currentSegment, laneWidth) {
				gs.autoDriveLane--
				laneChanged = true
				gs.lastAutoDriveLaneChange = now
			}
		}

		// If we're in a slow lane and no longer blocked, return to rightmost lane (respecting cooldown)
		if !laneChanged && canChangeLanes && gs.autoDriveLane < rightmostLane && !collisionRisk {
			if checkAvailability(rightmostLane) && gs.isLaneClear(rightmostLane, currentSegment, laneWidth) {
				gs.autoDriveLane = rightmostLane
				laneChanged = true
				gs.lastAutoDriveLaneChange = now
			}
		}
	}

	// 4. Speed Control - Maintain maximum speed when road is clear
	targetSpeed := maxSpeed

	// Only slow down if there's a CLOSE obstacle (not just any traffic ahead)
	if collisionRisk && minDist < 300 && !laneChanged {
		if minDist < 150 {
			targetSpeed = 0 // Brake hard for very close obstacles
		} else if minDist < 300 {
			targetSpeed = maxSpeed * 0.8 // Gentle slow down for closer obstacles
		}
	}

	// If no collision risk at all, ensure we're at max speed
	if !collisionRisk {
		targetSpeed = maxSpeed
	}

	if math.Abs(gs.playerCar.VelocityY-targetSpeed) < 0.1 {
		gs.playerCar.VelocityY = targetSpeed
	} else if gs.playerCar.VelocityY < targetSpeed {
		gs.playerCar.VelocityY += gs.playerCar.Acceleration
	} else if gs.playerCar.VelocityY > targetSpeed {
		gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 2.0
	}

	// 5. Steering
	// Re-calculate target in case lane changed
	targetLaneX = startLeftEdge + float64(gs.autoDriveLane)*laneWidth + laneWidth/2
	errorX := targetLaneX - gs.playerCar.X

	// P-Controller for steering
	kp := 0.03
	steer := errorX * kp
	// Clamp
	if steer > 1.0 {
		steer = 1.0
	}
	if steer < -1.0 {
		steer = -1.0
	}

	gs.playerCar.SteeringAngle = steer
}

// Update handles gameplay logic
func (gs *GameplayScreen) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		gs.paused = !gs.paused
		return nil
	}

	if gs.paused {
		return gs.updatePauseMenu()
	}

	currentSegment, segmentIdx := gs.getCurrentRoadSegment()
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

	// Handle inputs
	if gs.onFoot {
		gs.updatePed()
		// Stop the car
		gs.playerCar.VelocityY *= 0.9
		if gs.playerCar.VelocityY < 0.01 {
			gs.playerCar.VelocityY = 0
		}
		gs.playerCar.VelocityX *= 0.9
	} else {
		// Check for car exit
		// Check for car exit
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if math.Abs(gs.playerCar.VelocityY) < 0.5 {
				gs.exitCar()
			}
		}

		// Calculate current lane and speed limit
		currentLane := gs.getCurrentLane(currentSegment, laneWidth)
		speedLimitMPH := 50.0 + float64(currentLane)*10.0
		maxSpeed := speedLimitMPH / MPHPerPixelPerFrame

		// Toggle Auto Drive
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			gs.autoDrive = !gs.autoDrive
			if gs.autoDrive {
				// DOMINATE THE RIGHT LANE: Always start in the rightmost (fastest) lane
				gs.autoDriveLane = currentSegment.LaneCount - 1
				// Initialize lane change timer
				gs.lastAutoDriveLaneChange = time.Now().UnixMilli()
			}
		}

		if gs.autoDrive {
			gs.updateAutoPilot(currentSegment, segmentIdx, laneWidth, maxSpeed)
		} else {
			// Handle steering input (Left/Right arrow keys)
			maxSteeringAngle := 1.0
			steeringInput := 0.08

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

			minSpeed := 0.0
			if ebiten.IsKeyPressed(ebiten.KeyArrowUp) && gs.playerCar.SelectedCar.FuelLevel > 0 {
				if math.Abs(gs.playerCar.VelocityY-maxSpeed) < gs.playerCar.Acceleration {
					gs.playerCar.VelocityY = maxSpeed
				} else if gs.playerCar.VelocityY < maxSpeed {
					gs.playerCar.VelocityY += gs.playerCar.Acceleration
					if gs.playerCar.VelocityY > maxSpeed {
						gs.playerCar.VelocityY = maxSpeed
					}
				}
			} else if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
				gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 3.0
				if gs.playerCar.VelocityY < minSpeed {
					gs.playerCar.VelocityY = minSpeed
				}
			} else {
				if gs.playerCar.VelocityY > 0 {
					gs.playerCar.VelocityY -= gs.playerCar.Acceleration * 0.1
					if gs.playerCar.VelocityY < 0 {
						gs.playerCar.VelocityY = 0
					}
				}
			}
		}

		if gs.playerCar.VelocityY > maxSpeed {
			if gs.playerCar.VelocityY-maxSpeed < 0.05 {
				gs.playerCar.VelocityY = maxSpeed
			} else {
				gs.playerCar.VelocityY -= 0.02
				if gs.playerCar.VelocityY < maxSpeed {
					gs.playerCar.VelocityY = maxSpeed
				}
			}
		}

		referenceMaxSpeed := 100.0 / MPHPerPixelPerFrame
		speedFactor := gs.playerCar.VelocityY / referenceMaxSpeed
		
		// Calculate target lateral velocity based on steering angle
		targetVelocityX := gs.playerCar.SteeringAngle * gs.playerCar.TurnSpeed * speedFactor
		
		// Apply "grip" or inertia: Interpolate current VelocityX towards target
		// Lower grip factor = more drift/slide (0.0 = ice, 1.0 = instant turn)
		gripFactor := 0.2 
		gs.playerCar.VelocityX += (targetVelocityX - gs.playerCar.VelocityX) * gripFactor
	}

	// Update car position based on velocity
	gs.playerCar.X += gs.playerCar.VelocityX

	// Clamp car position to stay within road bounds
	// Calculate the road boundaries based on current segment with interpolation for ramps
	leftEdge := -float64(currentSegment.StartLaneIndex) * laneWidth
	rightEdge := leftEdge + float64(currentSegment.LaneCount)*laneWidth
	
	// Interpolate with NEXT segment to match visual ramp ahead
	if segmentIdx < len(gs.roadSegments)-1 {
		nextSegment := gs.roadSegments[segmentIdx+1]
		
		// Calculate next segment bounds
		nextLeftEdge := -float64(nextSegment.StartLaneIndex) * laneWidth
		nextRightEdge := nextLeftEdge + float64(nextSegment.LaneCount)*laneWidth
		
		// Calculate progress in current segment (0.0 at bottom/start, 1.0 at top/end)
		progress := (currentSegment.Y - gs.playerCar.Y) / 600.0
		if progress < 0 { progress = 0 }
		if progress > 1 { progress = 1 }
		
		leftEdge = leftEdge + (nextLeftEdge - leftEdge) * progress
		rightEdge = rightEdge + (nextRightEdge - rightEdge) * progress
	}

	// Check for nearby petrol stations to expand bounds (ALLOW ENTRY)
	for _, station := range gs.petrolStations {
		// Check vertical proximity (within 250px)
		if math.Abs(gs.playerCar.Y-station.Y) < 250 {
			// Expand left edge to include station area
			stationBound := station.X - 60
			if stationBound < leftEdge {
				leftEdge = stationBound
			}
		}
	}

	if gs.playerCar.X < leftEdge+10 {
		gs.playerCar.X = leftEdge + 10
		gs.playerCar.VelocityX = 0
	}
	if gs.playerCar.X > rightEdge-10 {
		gs.playerCar.X = rightEdge - 10
		gs.playerCar.VelocityX = 0
	}

	// Camera follows car perfectly on X axis to keep it centered
	targetX := gs.playerCar.X
	if gs.onFoot && gs.playerPed != nil {
		targetX = gs.playerPed.X
	}
	gs.cameraX = targetX - float64(gs.screenWidth)/2
	
	targetY := gs.playerCar.Y
	if gs.onFoot && gs.playerPed != nil {
		targetY = gs.playerPed.Y
	}
	// Offset Y by -125 to show more road ahead (moves camera up relative to car, so car appears lower)
	targetCameraY := targetY - float64(gs.screenHeight)/2 - 125
	gs.cameraY += (targetCameraY - gs.cameraY) * 0.1

	// Update distance travelled and fuel
	// MPH = Miles per Hour. At 60 FPS: Miles per Frame = MPH / 216000
	currentSpeedMPH := gs.playerCar.VelocityY * MPHPerPixelPerFrame
	gs.DistanceTravelled += currentSpeedMPH / 216000.0
	
	// Consume fuel based on speed
	// Base burn + speed factor (Tuned for ~5 mins driving)
	fuelBurn := 0.0002 + gs.playerCar.VelocityY * 0.0003
	if gs.playerCar.SelectedCar.FuelLevel > 0 {
		gs.playerCar.SelectedCar.FuelLevel -= fuelBurn
		if gs.playerCar.SelectedCar.FuelLevel < 0 {
			gs.playerCar.SelectedCar.FuelLevel = 0
		}
	}

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

	// Check Petrol Stations
	if math.Abs(gs.playerCar.VelocityY) < 0.5 { // Stopped or very slow
		for _, station := range gs.petrolStations {
			dist := math.Hypot(gs.playerCar.X-station.X, gs.playerCar.Y-station.Y)
			if dist < 80 {
				if gs.playerCar.SelectedCar.FuelLevel < gs.playerCar.SelectedCar.FuelCapacity {
					gs.playerCar.SelectedCar.FuelLevel += 0.5
					if gs.playerCar.SelectedCar.FuelLevel > gs.playerCar.SelectedCar.FuelCapacity {
						gs.playerCar.SelectedCar.FuelLevel = gs.playerCar.SelectedCar.FuelCapacity
					}
				}
			}
		}
	}

	return nil
}

// Draw renders the gameplay screen
func (gs *GameplayScreen) Draw(screen *ebiten.Image) {
	// Clear screen with sky color (retro blue) - ensure full coverage
	screen.Fill(color.RGBA{135, 206, 235, 255}) // Sky blue

	gs.drawBackground(screen)

	// Draw road segments
	gs.drawPetrolStationTarmac(screen)
	gs.drawRoad(screen)
	gs.drawPetrolStations(screen)

	// Draw traffic (behind player car)
	gs.drawTraffic(screen)

	// Draw player car
	gs.drawCar(screen)

	if gs.onFoot && gs.playerPed != nil {
		gs.drawPed(screen)
	}

	// Draw UI overlay
	gs.drawUI(screen)
	
	// Draw pause menu on top
	if gs.paused {
		gs.drawPauseMenu(screen)
	}
}

// drawBackground renders the base grass layer
func (gs *GameplayScreen) drawBackground(screen *ebiten.Image) {
	if gs.backgroundPattern == nil {
		return
	}

	w, h := gs.backgroundPattern.Size()

	// Calculate offset based on camera position to make it move with the world
	// Modulo width/height to keep it repeating seamlessly
	offsetX := -int(gs.cameraX) % w
	offsetY := -int(gs.cameraY) % h

	// Tile vertically and horizontally covering the screen
	// Start slightly off-screen to ensure smooth scrolling
	for y := offsetY - h; y < gs.screenHeight; y += h {
		for x := offsetX - w; x < gs.screenWidth; x += w {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			screen.DrawImage(gs.backgroundPattern, op)
		}
	}
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
	screenY := segment.Y - gs.cameraY

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

// generateBackgroundPattern creates a detailed repeating grass background
func (gs *GameplayScreen) generateBackgroundPattern() {
	patternSize := 128
	pattern := ebiten.NewImage(patternSize, patternSize)

	// Base color
	baseR, baseG, baseB := 34, 139, 34
	pattern.Fill(color.RGBA{uint8(baseR), uint8(baseG), uint8(baseB), 255})

	// Add patches
	for i := 0; i < 20; i++ {
		cx := rand.Intn(patternSize)
		cy := rand.Intn(patternSize)
		radius := rand.Intn(30) + 10
		
		// Shade variation
		shade := rand.Intn(40) - 20
		
		r := uint8(math.Min(255, math.Max(0, float64(baseR + shade))))
		g := uint8(math.Min(255, math.Max(0, float64(baseG + shade))))
		b := uint8(math.Min(255, math.Max(0, float64(baseB + shade))))
		col := color.RGBA{r, g, b, 255}

		// Draw circular patch (wrapping)
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy <= radius*radius {
					px := (cx + dx + patternSize) % patternSize
					py := (cy + dy + patternSize) % patternSize
					pattern.Set(px, py, col)
				}
			}
		}
	}

	// Add blades (noise)
	for i := 0; i < 1000; i++ {
		x := rand.Intn(patternSize)
		y := rand.Intn(patternSize)
		
		if rand.Float64() < 0.5 {
			// Dark
			pattern.Set(x, y, color.RGBA{20, 100, 20, 255})
		} else {
			// Light
			pattern.Set(x, y, color.RGBA{60, 180, 60, 255})
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
	screenY := gs.playerCar.Y - gs.cameraY - float64(carHeight)/2

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

// getCurrentRoadSegment finds the road segment the car is currently on and its index
func (gs *GameplayScreen) getCurrentRoadSegment() (RoadSegment, int) {
	// Find the segment closest to the car's Y position
	// Since Y decreases as we go up, we need to find the segment where carY is between segment.Y and segment.Y-600
	carWorldY := gs.playerCar.Y

	for i, segment := range gs.roadSegments {
		if carWorldY <= segment.Y && carWorldY > segment.Y-600 {
			return segment, i
		}
	}

	// If no segment found and we have segments, check if player is beyond the last segment
	if len(gs.roadSegments) > 0 {
		lastIdx := len(gs.roadSegments) - 1
		lastSegment := gs.roadSegments[lastIdx]
		if carWorldY <= lastSegment.Y - 600 {
			// Player is beyond the last segment - use the last segment as reference
			return lastSegment, lastIdx
		}
		// Player is before the first segment - use the first segment
		return gs.roadSegments[0], 0
	}

	// Fallback
	return RoadSegment{LaneCount: 1, RoadTypes: []string{"A"}, StartLaneIndex: 0, Y: 0}, -1
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
	gs.traffic = make([]*TrafficCar, 0)
}

// updateTraffic updates traffic positions and spawns new traffic vehicles
func (gs *GameplayScreen) updateTraffic(scrollSpeed float64, currentSegment RoadSegment, laneWidth float64) {
	playerY := gs.playerCar.Y

	// Update existing traffic positions
	gs.trafficMutex.Lock()

	// First pass: Run AI logic for all cars so they become aware of each other
	for i := 0; i < len(gs.traffic); i++ {
		tc := gs.traffic[i]
		tc.Update(gs)
	}

	// Second pass: Apply movement with collision prevention
	for i := 0; i < len(gs.traffic); i++ {
		tc := gs.traffic[i]

		// Calculate desired new position
		desiredY := tc.Y - tc.VelocityY

		// Check for collisions with other cars in same lane or transitioning lanes
		canMove := true
		for j := 0; j < len(gs.traffic); j++ {
			if i == j {
				continue
			}

			other := gs.traffic[j]
			// Check if other car is in same lane or transitioning to our lane
			inSameLane := other.Lane == tc.Lane
			transitioningToOurLane := (other.TargetLane == tc.Lane && other.LaneProgress > 0.5) ||
				(tc.TargetLane == other.Lane && tc.LaneProgress > 0.5)

			if inSameLane || transitioningToOurLane {
				// Check if moving would cause collision
				// Use a smaller collision box for movement to prevent "magnetic repulsion"
				// effectively allowing closer following before hard stop
				collisionDist := minTrafficDistance * 0.8 // Allow slightly closer than spawn distance
				
				// If deadlocked (moving very slowly), allow even closer proximity to wiggle out
				if tc.VelocityY < 1.0 {
					collisionDist = minTrafficDistance * 0.5
				}

				if math.Abs(desiredY - other.Y) < collisionDist {
					canMove = false
					break
				}
			}
		}

		// Only move if no collision would occur
		if canMove {
			tc.Y = desiredY
		} else {
			// If we can't move forward, slow down significantly to prevent pile-ups
			// More aggressive braking to prevent visual overlap
			tc.VelocityY *= 0.5 // Was 0.7 - brake harder
			// Apply some movement but much slower
			tc.Y -= tc.VelocityY * 0.2 // Was 0.3
		}

		// Check if player has passed this car (overtaken)
		// Player Y < Traffic Y means Player is AHEAD (further up the road)
		if !tc.Passed && gs.playerCar.Y < tc.Y {
			tc.Passed = true
			gs.TotalCarsPassed++

			// Level Up Logic
			if gs.TotalCarsPassed >= gs.LevelThreshold {
				gs.Level++
				gs.PrevLevelThreshold = gs.LevelThreshold
				gs.LevelThreshold = int(float64(gs.LevelThreshold) * 1.5)
			}
		}

		// Handle lane changing
		if tc.LaneProgress > 0 {
			// Increment progress
			tc.LaneProgress += 0.01 // Speed of lane change (slower/more gentle)
			if tc.LaneProgress > 1.0 {
				tc.LaneProgress = 1.0
			}

			// PRE-ACCELERATION: If moving to a faster lane (Target > Lane), update target speed early
			// This prevents slowing down during the merge and causing collisions
			if tc.TargetLane > tc.Lane {
				speedLimitMPH := 50.0 + float64(tc.TargetLane)*10.0
				newTargetSpeed := (speedLimitMPH - 5.0) / MPHPerPixelPerFrame
				if tc.TargetSpeed < newTargetSpeed {
					tc.TargetSpeed = newTargetSpeed
				}
			}

			// Get traffic segment for accurate lane positioning
			tcSegment := gs.getSegmentAt(tc.Y)

			// Calculate positions
			leftEdge := -float64(tcSegment.StartLaneIndex) * laneWidth
			startX := leftEdge + float64(tc.Lane)*laneWidth + laneWidth/2
			endX := leftEdge + float64(tc.TargetLane)*laneWidth + laneWidth/2

			// Lerp X
			tc.X = startX + (endX - startX) * tc.LaneProgress

			// Complete transition
			if tc.LaneProgress >= 1.0 {
				tc.Lane = tc.TargetLane
				tc.TargetLane = 0
				tc.LaneProgress = 0
				tc.LastLaneChangeTime = time.Now().UnixMilli()

				// Safety check: Never allow traffic in lane 0
				if tc.Lane < 1 {
					tc.Lane = 1
				}

				// Update TargetSpeed for new lane (standard update)
				speedLimitMPH := 50.0 + float64(tc.Lane)*10.0
				tc.TargetSpeed = (speedLimitMPH - 5.0) / MPHPerPixelPerFrame
			}
		}

		// Remove traffic that's too far off screen (beyond spawn range)
		if tc.Y > playerY+trafficSpawnRange+500 || tc.Y < playerY-trafficSpawnRange-500 {
			// Remove from slice
			gs.traffic = append(gs.traffic[:i], gs.traffic[i+1:]...)
			i--
			continue
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
		// Check if traffic is logically in this lane or moving into it
		// This matches the updateTraffic collision logic to prevent spawning on top of cars
		if tc.Lane == lane || tc.TargetLane == lane {
			laneTraffic = append(laneTraffic, tc)
		}
	}
	gs.trafficMutex.RUnlock()
	
	// Determine spawn range - spawn well off-screen
	// Screen height is 600, so we want to spawn at least 1000px away from player
	var minY, maxY float64
	if ahead {
		// Spawn ahead (above player, lower Y values)
		// Spawn between 1600px and 800px ahead (adjusted for reduced range)
		minY = playerY - trafficSpawnRange
		maxY = playerY - 800
	} else {
		// Spawn behind (below player, higher Y values)
		// Spawn between 800px and 1600px behind
		minY = playerY + 800
		maxY = playerY + trafficSpawnRange
	}
	
	// Generate a candidate spawn position uniformly in range
	spawnY := minY + rand.Float64()*(maxY-minY)
	
	// DENSITY CHECK: Increase minimum distance for faster lanes to prevent overcrowding
	// Lane 1 (60mph) -> 150px
	// Lane 2 (70mph) -> 250px
	// Lane 3+ (80mph+) -> 350px
	minSpawnDist := minTrafficDistance
	if lane > 1 {
		minSpawnDist += 100.0
	}
	if lane > 2 {
		minSpawnDist += 100.0
	}

	// Check if the candidate position is safe (maintaining density-aware distance)
	isSafe := true
	for _, tc := range laneTraffic {
		if math.Abs(tc.Y - spawnY) < minSpawnDist {
			isSafe = false
			break
		}
	}
	
	// Additional cluster check: ensure we don't spawn too many cars in a small area across all lanes
	if isSafe {
		gs.trafficMutex.RLock()
		carsInProximity := 0
		for _, tc := range gs.traffic {
			if math.Abs(tc.Y - spawnY) < 300.0 {
				carsInProximity++
			}
		}
		gs.trafficMutex.RUnlock()
		
		// If there are already 2 or more cars nearby (in any lane), don't spawn another one
		// This prevents "walls" of traffic
		if carsInProximity >= 2 {
			isSafe = false
		}
	}
	
	// If not safe, don't spawn
	if !isSafe {
		return
	}
	
	// Calculate lane center X position
	leftEdge := -float64(segment.StartLaneIndex) * laneWidth
	laneCenterX := leftEdge + float64(lane)*laneWidth + laneWidth/2
	
	// Check restriction: Only one car spawned ahead in player's lane
	playerLane := gs.getCurrentLane(segment, laneWidth)
	if ahead && lane == playerLane {
		carsAheadInPlayerLane := 0
		for _, tc := range laneTraffic {
			if tc.Y < playerY { // Ahead
				carsAheadInPlayerLane++
			}
		}
		if carsAheadInPlayerLane >= 1 {
			return
		}
	}
	
	// Determine traffic speed (player speed limit minus 5mph)
	// Lane 0=50, Lane 1=60, Lane 2=70, etc. (match player steps)
	speedLimitMPH := 50.0 + float64(lane)*10.0
	// Target speed MUST be 5mph below the limit
	targetSpeedMPH := speedLimitMPH - 5.0
	
	trafficVelocityY := targetSpeedMPH / MPHPerPixelPerFrame
	
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
	
	// Safety check: Never spawn traffic in lane 0 (reserved for player)
	if lane == 0 {
		return
	}

	// Create new traffic car
	newTraffic := &TrafficCar{
		X:           laneCenterX,
		Y:           spawnY,
		VelocityY:   trafficVelocityY,
		TargetSpeed: trafficVelocityY,
		Acceleration: 0.05, // Same as player
		Deceleration: 0.1,  // Better braking
		Lane:        lane,
		Color:       carColor,
		Passed:      !ahead, // If spawned behind, it's already passed
		LastLaneChangeTime: time.Now().UnixMilli(), // Initialize with spawn time
	}
	
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
		screenY := tc.Y - gs.cameraY - float64(carHeight)/2
		
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
	// Top left: Speedometer
	gs.drawSpeedometer(screen)

	// Top right: Stats
	// Draw background box for stats
	statsWidth := 180.0
	statsHeight := 220.0 // Increased height for extra spacing
	x := float64(gs.screenWidth) - statsWidth - 20.0
	y := 20.0
	
	bgImg := ebiten.NewImage(int(statsWidth), int(statsHeight))
	bgImg.Fill(color.RGBA{20, 20, 30, 200})
	
	// Draw border
	borderColor := color.RGBA{100, 100, 120, 255}
	w, h := int(statsWidth), int(statsHeight)
	for i := 0; i < w; i++ {
		bgImg.Set(i, 0, borderColor)
		bgImg.Set(i, h-1, borderColor)
	}
	for i := 0; i < h; i++ {
		bgImg.Set(0, i, borderColor)
		bgImg.Set(w-1, i, borderColor)
	}
	
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(bgImg, op)

	// Padding inside box
	x += 15.0
	y += 15.0

	face := text.NewGoXFace(bitmapfont.Face)
	
	// Draw Miles
	milesText := fmt.Sprintf("MILES: %.1f", gs.DistanceTravelled)
	textOp := &text.DrawOptions{}
	textOp.GeoM.Translate(x, y)
	textOp.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, milesText, face, textOp)
	
	// Draw Status Bars (Fuel, Food, Sleep)
	barWidth := 150.0
	barHeight := 12.0
	spacing := 40.0 // Increased spacing (was 25.0)
	
	// Fuel
	fuelPercent := gs.playerCar.SelectedCar.FuelLevel / gs.playerCar.SelectedCar.FuelCapacity
	gs.drawStatusBar(screen, x, y+spacing, barWidth, barHeight, fuelPercent, "FUEL", color.RGBA{255, 165, 0, 255}) // Orange
	
	// Food
	foodPercent := gs.playerCar.SelectedCar.FoodLevel / gs.playerCar.SelectedCar.FoodCapacity
	gs.drawStatusBar(screen, x, y+spacing*2, barWidth, barHeight, foodPercent, "FOOD", color.RGBA{0, 255, 0, 255}) // Green
	
	// Sleep
	sleepPercent := gs.playerCar.SelectedCar.SleepLevel / gs.playerCar.SelectedCar.SleepCapacity
	gs.drawStatusBar(screen, x, y+spacing*3, barWidth, barHeight, sleepPercent, "SLEEP", color.RGBA{50, 150, 255, 255}) // Blue
	
	// Level Progress Bar
	levelProgress := float64(gs.TotalCarsPassed - gs.PrevLevelThreshold) / float64(gs.LevelThreshold - gs.PrevLevelThreshold)
	if levelProgress < 0 { levelProgress = 0 }
	if levelProgress > 1 { levelProgress = 1 }
	
	levelLabel := fmt.Sprintf("LEVEL %d", gs.Level)
	gs.drawStatusBar(screen, x, y+spacing*4, barWidth, barHeight, levelProgress, levelLabel, color.RGBA{255, 215, 0, 255}) // Gold
	// DEBUG: Traffic Counter
	gs.trafficMutex.RLock()
	totalCars := len(gs.traffic)
	carsAhead := 0
	carsBehind := 0
	playerY := gs.playerCar.Y
	for _, tc := range gs.traffic {
		if tc.Y < playerY {
			carsAhead++
		} else {
			carsBehind++
		}
	}
	gs.trafficMutex.RUnlock()

	debugText := fmt.Sprintf("CARS: %d (AHEAD: %d, BEHIND: %d)", totalCars, carsAhead, carsBehind)
	debugOp := &text.DrawOptions{}
	debugOp.GeoM.Translate(20, float64(gs.screenHeight)-30)
	debugOp.ColorScale.ScaleWithColor(color.RGBA{200, 200, 200, 255})
	text.Draw(screen, debugText, face, debugOp)
}

func (gs *GameplayScreen) drawPetrolStationTarmac(screen *ebiten.Image) {
	for _, station := range gs.petrolStations {
		// Draw large tarmac area
		w, h := 200, 500
		tarmacImg := ebiten.NewImage(w, h)
		tarmacImg.Fill(color.RGBA{105, 105, 105, 255}) // DimGray

		worldX := station.X - 80
		worldY := station.Y - float64(h)/2

		screenX := worldX - gs.cameraX
		screenY := worldY - gs.cameraY

		if screenY < -float64(h) || screenY > float64(gs.screenHeight) {
			continue
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(screenX, screenY)
		screen.DrawImage(tarmacImg, op)
	}
}

func (gs *GameplayScreen) drawPetrolStations(screen *ebiten.Image) {
	face := text.NewGoXFace(bitmapfont.Face)
	for _, station := range gs.petrolStations {
		// Calculate screen pos
		screenX := station.X - gs.cameraX - 20
		screenY := station.Y - gs.cameraY - 20

		// Only draw if visible
		if screenY < -50 || screenY > float64(gs.screenHeight)+50 {
			continue
		}

		// Draw Pump
		pumpImg := ebiten.NewImage(40, 40)
		pumpImg.Fill(color.RGBA{255, 50, 50, 255}) // Red box
		// Draw "P"
		for i := 10; i < 30; i++ {
			pumpImg.Set(i, 10, color.White)
			pumpImg.Set(i, 20, color.White)
		}
		for i := 10; i < 30; i++ {
			pumpImg.Set(10, i, color.White)
		}
		for i := 10; i < 20; i++ {
			pumpImg.Set(30, i, color.White)
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(screenX, screenY)
		screen.DrawImage(pumpImg, op)

		// Draw "FUEL" text
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(screenX, screenY-15)
		textOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, "FUEL", face, textOp)
	}
}

// drawSpeedometer draws a speedometer displaying current speed in MPH
func (gs *GameplayScreen) drawSpeedometer(screen *ebiten.Image) {
	// Calculate speed in MPH from VelocityY (pixels per frame)
	speedMPH := gs.playerCar.VelocityY * MPHPerPixelPerFrame
	
	// Get current lane and speed limit
	currentSegment, _ := gs.getCurrentRoadSegment()
	laneWidth := 80.0
	currentLane := gs.getCurrentLane(currentSegment, laneWidth)
	speedLimitMPH := 50.0 + float64(currentLane)*10.0
	
	// Position in top-left corner
	x := 20.0
	y := 20.0
	width := 180.0
	height := 160.0 // Increased height for spacing (was 140.0)
	
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
	textY := y + 40.0 // Moved up slightly (was 50)
	
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
	labelY := y + 75.0 // Moved up (was 85)
	
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
	limitY := y + 100.0 // Moved up slightly (was 105)
	
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
	gs.drawSpeedGauge(screen, x+10, y+height-30, width-20, 15, speedMPH, speedLimitMPH)
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

// drawStatusBar draws a labeled status bar with percentage fill
func (gs *GameplayScreen) drawStatusBar(screen *ebiten.Image, x, y, width, height float64, percent float64, label string, barColor color.RGBA) {
	// Draw label
	face := text.NewGoXFace(bitmapfont.Face)
	labelOp := &text.DrawOptions{}
	labelOp.GeoM.Translate(x, y-14) // Above bar (Moved up for clearance)
	labelOp.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, label, face, labelOp)

	// Draw background (dark gray)
	bgBar := ebiten.NewImage(int(width), int(height))
	bgBar.Fill(color.RGBA{40, 40, 40, 200})
	
	// Draw border
	borderColor := color.RGBA{100, 100, 100, 255}
	w, h := int(width), int(height)
	for i := 0; i < w; i++ {
		bgBar.Set(i, 0, borderColor)
		bgBar.Set(i, h-1, borderColor)
	}
	for i := 0; i < h; i++ {
		bgBar.Set(0, i, borderColor)
		bgBar.Set(w-1, i, borderColor)
	}
	
	bgOp := &ebiten.DrawImageOptions{}
	bgOp.GeoM.Translate(x, y)
	screen.DrawImage(bgBar, bgOp)
	
	// Draw filled portion
	if percent > 1.0 { percent = 1.0 }
	if percent < 0.0 { percent = 0.0 }
	
	filledWidth := int(width * percent)
	if filledWidth > 0 {
		filledBar := ebiten.NewImage(filledWidth, int(height))
		filledBar.Fill(barColor)
		
		// Warning color (red) if low
		if percent < 0.2 {
			filledBar.Fill(color.RGBA{255, 50, 50, 255})
		}
		
		fillOp := &ebiten.DrawImageOptions{}
		fillOp.GeoM.Translate(x, y)
		screen.DrawImage(filledBar, fillOp)
	}
}

// createPedSprite generates a simple pedestrian sprite
func (gs *GameplayScreen) createPedSprite() *ebiten.Image {
	img := ebiten.NewImage(16, 16)
	img.Fill(color.RGBA{255, 200, 150, 255}) // Skin
	// Add some clothes (Blue shirt)
	for y := 6; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{0, 0, 200, 255})
		}
	}
	return img
}

func (gs *GameplayScreen) exitCar() {
	gs.onFoot = true
	gs.playerPed = &PlayerPed{
		X:      gs.playerCar.X - 40, // Spawn to the left
		Y:      gs.playerCar.Y,
		Speed:  3.0,
		Sprite: gs.createPedSprite(),
	}
	gs.playerCar.VelocityX = 0
	gs.playerCar.VelocityY = 0
}

func (gs *GameplayScreen) updatePed() {
	// Movement (8 directions)
	dx, dy := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		dx = -1
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		dx = 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		dy = -1
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		dy = 1
	}

	// Run (Shift)
	speed := gs.playerPed.Speed
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		speed *= 2.0
	}

	// Normalize diagonal
	if dx != 0 && dy != 0 {
		factor := speed / math.Sqrt(2)
		gs.playerPed.X += dx * factor
		gs.playerPed.Y += dy * factor
	} else {
		gs.playerPed.X += dx * speed
		gs.playerPed.Y += dy * speed
	}

	// Interaction with Traffic
	gs.trafficMutex.Lock()
	defer gs.trafficMutex.Unlock()

	for i, tc := range gs.traffic {
		dist := math.Hypot(tc.X-gs.playerPed.X, tc.Y-gs.playerPed.Y)

		// Stop car if close
		if dist < 150 {
			tc.TargetSpeed = 0
			tc.VelocityY *= 0.9 // Brake
		}

		// Steal Car
		if dist < 50 && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			// Take over traffic car
			gs.playerCar.X = tc.X
			gs.playerCar.Y = tc.Y
			gs.playerCar.VelocityX = 0
			gs.playerCar.VelocityY = 0
			// TODO: Change color/sprite of player car?

			// Remove traffic car
			gs.traffic = append(gs.traffic[:i], gs.traffic[i+1:]...)

			// Enter car
			gs.onFoot = false
			gs.playerPed = nil
			return
		}
	}

	// Re-enter own car
	distToOwn := math.Hypot(gs.playerCar.X-gs.playerPed.X, gs.playerCar.Y-gs.playerPed.Y)
	if distToOwn < 50 && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		gs.onFoot = false
		gs.playerPed = nil
	}
}

func (gs *GameplayScreen) drawPed(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	// Position relative to camera/screen
	screenX := gs.playerPed.X - gs.cameraX - 8
	screenY := gs.playerPed.Y - gs.cameraY - 8

	op.GeoM.Translate(screenX, screenY)
	screen.DrawImage(gs.playerPed.Sprite, op)
}
func (gs *GameplayScreen) updatePauseMenu() error {
	// Mouse interaction
	mx, my := ebiten.CursorPosition()
	centerX := gs.screenWidth / 2
	centerY := gs.screenHeight / 2
	
	// Button dimensions
	btnW, btnH := 200, 50
	
	// Resume Button (Center Y + 50)
	if mx >= centerX-btnW/2 && mx <= centerX+btnW/2 &&
	   my >= centerY+50-btnH/2 && my <= centerY+50+btnH/2 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			gs.paused = false
		}
	}
	
	// Exit Button (Center Y + 110)
	if mx >= centerX-btnW/2 && mx <= centerX+btnW/2 &&
	   my >= centerY+110-btnH/2 && my <= centerY+110+btnH/2 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Exit to title
			if gs.onGameEnd != nil {
				gs.onGameEnd()
			}
		}
	}
	
	return nil
}

// drawPauseMenu renders the pause menu overlay
func (gs *GameplayScreen) drawPauseMenu(screen *ebiten.Image) {
	// Dark overlay
	overlay := ebiten.NewImage(gs.screenWidth, gs.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 200})
	screen.DrawImage(overlay, nil)
	
	centerX := float64(gs.screenWidth) / 2
	centerY := float64(gs.screenHeight) / 2
	
	face := text.NewGoXFace(bitmapfont.Face)
	
	// 1. Retro Player Avatar (Top)
	avatarScale := 6.0
	avatarSize := 8.0 * avatarScale
	avatarX := centerX - avatarSize/2
	avatarY := centerY - 180 // Move up slightly
	
	// Draw Avatar Background
	avatarImg := ebiten.NewImage(8, 8)
	// Skin
	avatarImg.Fill(color.RGBA{255, 200, 150, 255})
	// Hair (Top 2 rows + sideburns)
	for x := 0; x < 8; x++ {
		avatarImg.Set(x, 0, color.RGBA{60, 40, 20, 255})
		avatarImg.Set(x, 1, color.RGBA{60, 40, 20, 255})
	}
	avatarImg.Set(0, 2, color.RGBA{60, 40, 20, 255})
	avatarImg.Set(7, 2, color.RGBA{60, 40, 20, 255})
	
	// Sunglasses (Cool driver look)
	for x := 1; x < 7; x++ {
		if x != 3 && x != 4 { // Lenses
			avatarImg.Set(x, 3, color.RGBA{0, 0, 0, 255})
		} else { // Bridge
			avatarImg.Set(x, 3, color.RGBA{50, 50, 50, 255})
		}
	}
	
	// Smile
	avatarImg.Set(2, 6, color.RGBA{150, 50, 50, 255})
	avatarImg.Set(3, 6, color.RGBA{150, 50, 50, 255})
	avatarImg.Set(4, 6, color.RGBA{150, 50, 50, 255})
	avatarImg.Set(5, 6, color.RGBA{150, 50, 50, 255})
	avatarImg.Set(1, 5, color.RGBA{150, 50, 50, 255})
	avatarImg.Set(6, 5, color.RGBA{150, 50, 50, 255})
	
	// Draw Border
	borderImg := ebiten.NewImage(int(avatarSize)+8, int(avatarSize)+8)
	borderImg.Fill(color.White)
	borderOp := &ebiten.DrawImageOptions{}
	borderOp.GeoM.Translate(avatarX-4, avatarY-4)
	screen.DrawImage(borderImg, borderOp)
	
	// Draw Avatar
	avatarOp := &ebiten.DrawImageOptions{}
	avatarOp.GeoM.Scale(avatarScale, avatarScale)
	avatarOp.GeoM.Translate(avatarX, avatarY)
	screen.DrawImage(avatarImg, avatarOp)

	// PAUSED text
	pausedText := "PAUSED"
	textW := text.Advance(pausedText, face) * 3
	op := &text.DrawOptions{}
	op.GeoM.Scale(3, 3)
	op.GeoM.Translate(centerX - textW/2, avatarY + avatarSize + 20)
	op.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, pausedText, face, op)
	
	// Stats
	statsY := avatarY + avatarSize + 70
	milesText := fmt.Sprintf("TOTAL MILES: %.1f", gs.DistanceTravelled)
	carsText := fmt.Sprintf("CARS PASSED: %d", gs.TotalCarsPassed)
	
	statsScale := 1.5
	
	// Miles
	mW := text.Advance(milesText, face) * statsScale
	mOp := &text.DrawOptions{}
	mOp.GeoM.Scale(statsScale, statsScale)
	mOp.GeoM.Translate(centerX - mW/2, statsY)
	mOp.ColorScale.ScaleWithColor(color.RGBA{100, 255, 255, 255})
	text.Draw(screen, milesText, face, mOp)
	
	// Cars
	cW := text.Advance(carsText, face) * statsScale
	cOp := &text.DrawOptions{}
	cOp.GeoM.Scale(statsScale, statsScale)
	cOp.GeoM.Translate(centerX - cW/2, statsY + 30)
	cOp.ColorScale.ScaleWithColor(color.RGBA{100, 255, 100, 255})
	text.Draw(screen, carsText, face, cOp)
	
	// Level
	levelText := fmt.Sprintf("LEVEL: %d", gs.Level)
	lW := text.Advance(levelText, face) * statsScale
	lOp := &text.DrawOptions{}
	lOp.GeoM.Scale(statsScale, statsScale)
	lOp.GeoM.Translate(centerX - lW/2, statsY + 60)
	lOp.ColorScale.ScaleWithColor(color.RGBA{255, 215, 0, 255})
	text.Draw(screen, levelText, face, lOp)
	
	// Buttons
	// Helper to draw button
	drawButton := func(label string, y float64) {
		btnW, btnH := 200.0, 50.0
		btnX := centerX - btnW/2
		btnY := y - btnH/2
		
		// Button background
		btnImg := ebiten.NewImage(int(btnW), int(btnH))
		btnImg.Fill(color.RGBA{50, 50, 50, 255})
		
		// Check hover
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= btnX && float64(mx) <= btnX+btnW &&
		   float64(my) >= btnY && float64(my) <= btnY+btnH {
			btnImg.Fill(color.RGBA{100, 100, 100, 255}) // Highlight
		}
		
		btnOp := &ebiten.DrawImageOptions{}
		btnOp.GeoM.Translate(btnX, btnY)
		screen.DrawImage(btnImg, btnOp)
		
		// Button Text
		textW := text.Advance(label, face) * 2
		textOp := &text.DrawOptions{}
		textOp.GeoM.Scale(2, 2)
		textOp.GeoM.Translate(centerX - textW/2, y - 8) // Centered
		textOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, label, face, textOp)
	}
	
	drawButton("RESUME", centerY + 50)
	drawButton("EXIT", centerY + 110)
}
