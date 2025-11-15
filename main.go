package main

import (
	"image/color"
	"log"

	"github.com/golangdaddy/roadster/game"
	"github.com/golangdaddy/roadster/models"
	carmodel "github.com/golangdaddy/roadster/models/car"
	"github.com/golangdaddy/roadster/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

// GameState represents the current state of the game
type GameState int

const (
	StateLoadingScreen GameState = iota
	StateGarageScreen
	StateInGame
)

// Game implements ebiten.Game interface.
type Game struct {
	state         GameState
	loadingScreen *ui.LoadingScreen
	garageScreen  *ui.GarageScreen
	gameState     *models.GameState
	selectedCar   *carmodel.Car
	roadView      *game.RoadView
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	switch g.state {
	case StateLoadingScreen:
		if g.loadingScreen == nil {
			g.loadingScreen = ui.NewLoadingScreen(g.onNewGameClicked)
		}
		return g.loadingScreen.Update()
	case StateGarageScreen:
		if g.garageScreen == nil {
			g.garageScreen = ui.NewGarageScreen(g.onCarSelected)
		}
		return g.garageScreen.Update()
	case StateInGame:
		if g.roadView == nil && g.selectedCar != nil {
			g.roadView = game.NewRoadView(g.gameState, g.selectedCar, g.onReturnToGarage)
		}
		if g.roadView != nil {
			return g.roadView.Update()
		}
	}
	return nil
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	switch g.state {
	case StateLoadingScreen:
		if g.loadingScreen != nil {
			g.loadingScreen.Draw(screen)
		}
	case StateGarageScreen:
		if g.garageScreen != nil {
			g.garageScreen.Draw(screen)
		}
	case StateInGame:
		if g.roadView != nil {
			g.roadView.Draw(screen)
		} else {
			// Fallback if road view not initialized
			screen.Fill(color.RGBA{30, 30, 40, 255})
		}
	}
}

// Layout takes the outside size (e.g., the window size) and returns the (logical) screen size.
// If you don't have to adjust the screen size with the outside size, just return a fixed size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1024, 600
}

// onNewGameClicked is called when "New Game" is clicked on loading screen
func (g *Game) onNewGameClicked(gameState *models.GameState) {
	g.gameState = gameState
	g.state = StateGarageScreen // Go to garage screen to select car
	log.Printf("New game started: %s (Player: %s, Level: %d)",
		gameState.SaveName, gameState.Player.Stats.Name, gameState.Player.Stats.Level)
}

// onCarSelected is called when a car is selected from the garage
func (g *Game) onCarSelected(selectedCar *carmodel.Car) {
	g.selectedCar = selectedCar
	g.state = StateInGame
	// Reset road view so it gets recreated with new car
	g.roadView = nil
	log.Printf("Car selected: %s %s (Weight: %.0f kg, Brake Eff: %.1f%%)",
		selectedCar.Make, selectedCar.Model, selectedCar.Weight,
		selectedCar.Brakes.Condition*selectedCar.Brakes.Performance*selectedCar.Brakes.StoppingPower*100)
}

// onReturnToGarage is called when Escape is pressed during gameplay
func (g *Game) onReturnToGarage() {
	g.state = StateGarageScreen
	// Reset road view so it gets recreated when returning to game
	g.roadView = nil
	log.Printf("Returning to garage selection")
}

func main() {
	// Initialize global car inventory
	models.InitializeCarInventory()

	game := &Game{
		state: StateLoadingScreen,
	}
	// Specify the window size
	ebiten.SetWindowSize(1024, 600)
	ebiten.SetWindowTitle("Roadster")
	// Call ebiten.RunGame to start your game loop.
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
