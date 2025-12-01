package game

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/golangdaddy/roadster/pkg/models"
	"github.com/golangdaddy/roadster/pkg/models/car"
	"github.com/golangdaddy/roadster/pkg/road"
	"github.com/golangdaddy/roadster/pkg/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

type GameLogic struct {
	levels    []*road.RoadController
	levelData []*LevelData
}

func (g *GameLogic) Levels() []*road.RoadController {
	return g.levels
}

func (g *GameLogic) LevelData() []*LevelData {
	return g.levelData
}

func (game *GameLogic) LoadLevels() error {
	// Find all level files
	levelFiles, err := filepath.Glob("assets/level/*.json")
	if err != nil {
		return err
	}

	game.levels = make([]*road.RoadController, 0, len(levelFiles))
	game.levelData = make([]*LevelData, 0, len(levelFiles))

	for _, levelFile := range levelFiles {
		roadController, levelData, err := game.loadLevel(levelFile)
		if err != nil {
			return err
		}
		game.levels = append(game.levels, roadController)
		game.levelData = append(game.levelData, levelData)
	}

	return nil
}

// LevelData represents the parsed level information for rendering
type LevelData struct {
	Segments []RoadSegment
}

// RoadSegment represents a segment of road with its type and lane count
type RoadSegment struct {
	LaneCount      int
	RoadTypes      []string // Road type for each lane (left to right)
	LanePositions  []int    // Character position in level file for each rendered lane (maps rendered index to actual position)
	StartLaneIndex int      // Index of the starting lane (player's original lane)
	Y              float64  // World position (added for gameplay rendering)
}

func (game *GameLogic) loadLevel(filename string) (*road.RoadController, *LevelData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	roadController := road.NewRoadController()
	levelData := &LevelData{
		Segments: make([]RoadSegment, 0),
	}

	// Parse JSON level definition
	var levelDef road.LevelDefinition
	if err := json.NewDecoder(file).Decode(&levelDef); err != nil {
		return nil, nil, err
	}

	// Reconstruct lines from Layout and Sections
	reconstructedLines := make([]string, 0)
	
	// Import strings for Repeat function if not already imported
	// Since we can't see imports here, we assume "strings" is needed.
	
	lastLaneCount := 0
	
	for _, sectionName := range levelDef.Layout {
		if section, ok := levelDef.Sections[sectionName]; ok {
			for _, seg := range section.Segments {
				currentLaneCount := len(seg)
				
				// Auto-insert Transition Segment if lane count increases by 1
				if lastLaneCount > 0 && currentLaneCount == lastLaneCount + 1 {
					// Create transition segment: previous lanes + "D"
					// We need to construct a string of 'A's of length lastLaneCount
					// Since we don't have strings imported in this context snippet, let's do it manually or assume strings is imported.
					// Given previous file content had "strings" import, it should be safe.
					// If not, we will fix it.
					
					// Construct "AAAA...D"
					transition := ""
					for i := 0; i < lastLaneCount; i++ {
						transition += "A"
					}
					transition += "D"
					
					// Initialize with "X" + transition to assume empty lane 0
					reconstructedLines = append(reconstructedLines, "X"+transition)
				}
				
				// Initialize with "X" + segment to assume empty lane 0
				reconstructedLines = append(reconstructedLines, "X"+seg)
				
				lastLaneCount = currentLaneCount
			}
		}
	}

	// Apply Laybys
	// Note: We need to make sure we don't overwrite existing road types if we can avoid it,
	// but the previous logic just replaced the line.
	// We also need to handle the case where Laybys might be defined relative to the start of the level.
	for _, layby := range levelDef.Laybys {
		idx := layby.StartSegment
		if idx >= len(reconstructedLines) {
			continue
		}

		// The original segment string (without the prepended X)
		// We need to fetch it from the reconstructedLines (stripping the X we just added)
		originalSegment := reconstructedLines[idx][1:]

		// Auto-insert On-ramp/Off-ramp logic
		// We need to look at the segment to decide what letters to put where.
		// Assuming originalSegment defines the main road.
		
		// We need to prepend the special characters to the original segment string.
		// This effectively adds a lane to the left.
		// e.g. "AAA" -> "BAAA" (Off-ramp)
		
		// Start of layby (Off-ramp - B)
		reconstructedLines[idx] = "B" + originalSegment
		idx++

		// Services
		for _, service := range layby.Services {
			if idx >= len(reconstructedLines) {
				break
			}
			
			originalSegment = reconstructedLines[idx][1:]
			
			// Map service type to character
			char := "F" // Default to Petrol/Services (F)
			// TODO: Add mapping for other service types when textures exist
			// For now we just use F regardless of service.Type
			_ = service 
			
			reconstructedLines[idx] = char + originalSegment
			idx++
		}

		// Padding (Empty Layby Lane - G) - extends layby by 1 segment
		if idx < len(reconstructedLines) {
			originalSegment = reconstructedLines[idx][1:]
			reconstructedLines[idx] = "G" + originalSegment
			idx++
		}

		// End of layby (On-ramp - C)
		if idx < len(reconstructedLines) {
			originalSegment = reconstructedLines[idx][1:]
			reconstructedLines[idx] = "C" + originalSegment
		}
	}

	// Process reconstructed lines using existing logic
	for _, line := range reconstructedLines {
		if line == "" {
			continue
		}

		// Parse road types - each character represents a lane position
		// 'X' means no lane at that position (blank/skip)
		// Any other letter is a lane type at that position
		// IMPORTANT: Position 0 in the string is always lane 0, even if it's 'X'
		roadTypes := make([]string, 0)
		lanePositions := make([]int, 0) // Maps rendered lane index to character position
		startLaneIndex := -1

		// Find which position in the string has lanes
		// Store both the road type and the character position
		for pos, char := range line {
			roadType := string(char)
			if roadType != "X" {
				roadTypes = append(roadTypes, roadType)
				lanePositions = append(lanePositions, pos) // Store the actual character position

				// The starting lane is at position 1 (second character) if it exists
				// Otherwise default to the first non-X lane
				if pos == 1 {
					startLaneIndex = len(roadTypes) - 1
				}
			}
		}

		laneCount := len(roadTypes)

		// If no valid lanes found, skip this line
		if laneCount == 0 {
			continue
		}

		// If no starting lane was found, default to rightmost lane (last one)
		if startLaneIndex == -1 {
			startLaneIndex = laneCount - 1
		}

		// Create lane controllers for this segment
		for i := 0; i < laneCount; i++ {
			laneController := road.NewLaneController(i)
			roadController.AddLaneController(laneController)
		}

		// Store segment data for rendering
		segment := RoadSegment{
			LaneCount:      laneCount,
			RoadTypes:      roadTypes,
			LanePositions:  lanePositions,
			StartLaneIndex: startLaneIndex,
		}
		levelData.Segments = append(levelData.Segments, segment)
	}

	return roadController, levelData, nil
}

// Game implements the ebiten.Game interface and manages the overall game state
type Game struct {
	gameLogic     *GameLogic
	currentScreen Screen
}

// Screen represents a UI screen interface
type Screen interface {
	Update() error
	Draw(screen *ebiten.Image)
}

// NewGame creates a new game instance
func NewGame() *Game {
	game := &Game{
		gameLogic: &GameLogic{},
	}

	// Initialize with title screen
	game.currentScreen = ui.NewTitleScreen(func() {
		// Transition to loading screen
		game.currentScreen = ui.NewLoadingScreen(func(gameState *models.GameState) {
			// Transition to garage screen
			game.currentScreen = ui.NewGarageScreen(func(selectedCar *car.Car) {
				// Start the actual game with selected car
				game.startGameplay(selectedCar)
			})
		})
	})

	// Load levels
	if err := game.gameLogic.LoadLevels(); err != nil {
		// Handle error - for now just continue
		log.Printf("Failed to load levels: %v", err)
	}

	return game
}

// Update handles game logic updates
func (g *Game) Update() error {
	if g.currentScreen != nil {
		return g.currentScreen.Update()
	}
	return nil
}

// Draw renders the current screen
func (g *Game) Draw(screen *ebiten.Image) {
	if g.currentScreen != nil {
		g.currentScreen.Draw(screen)
	}
}

// Layout returns the game's screen dimensions
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1024, 600 // Standard window size
}

// startGameplay transitions to the actual gameplay
func (g *Game) startGameplay(selectedCar *car.Car) {
	// Use the first level for now
	levelData := g.gameLogic.LevelData()
	if len(levelData) > 0 {
		g.currentScreen = NewGameplayScreen(selectedCar, levelData[0], func() {
			// When game ends, go back to title
			g.currentScreen = ui.NewTitleScreen(func() {
				g.currentScreen = ui.NewLoadingScreen(func(gameState *models.GameState) {
					g.currentScreen = ui.NewGarageScreen(func(car *car.Car) {
						g.startGameplay(car)
					})
				})
			})
		})
	} else {
		// Fallback to title if no levels loaded
		g.currentScreen = ui.NewTitleScreen(func() {
			g.currentScreen = ui.NewLoadingScreen(func(gameState *models.GameState) {
				g.currentScreen = ui.NewGarageScreen(func(car *car.Car) {
					g.startGameplay(car)
				})
			})
		})
	}
}
