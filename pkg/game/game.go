package game

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	levelFiles, err := filepath.Glob("assets/level/*.level")
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

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse road types - each character represents a lane position
		// 'X' means no lane at that position (blank/skip)
		// Any other letter is a lane type at that position
		roadTypes := make([]string, 0)
		startLaneIndex := -1
		firstLineStartPos := -1 // Track the position of the first non-X in the entire level

		// Find which position in the string has lanes
		for pos, char := range line {
			roadType := string(char)
			if roadType != "X" {
				roadTypes = append(roadTypes, roadType)

				// Track the position of the first segment's lane to use as reference
				// This will be done better - for now, assume position with most consistency
				if firstLineStartPos == -1 {
					firstLineStartPos = pos
				}

				// The starting lane is whichever lane is at the original starting position
				// For now, we'll mark lanes by their actual position in the string
				if pos == 1 { // Position 1 in the string is typically the starting position in your level
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
			StartLaneIndex: startLaneIndex,
		}
		levelData.Segments = append(levelData.Segments, segment)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
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
