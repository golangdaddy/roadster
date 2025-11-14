package main

import (
	"image/color"
	"log"

	"github.com/golangdaddy/roadster/models"
	"github.com/golangdaddy/roadster/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

// GameState represents the current state of the game
type GameState int

const (
	StateLoadingScreen GameState = iota
	StateInGame
)

// Game implements ebiten.Game interface.
type Game struct {
	state         GameState
	loadingScreen *ui.LoadingScreen
	gameState     *models.GameState
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	switch g.state {
	case StateLoadingScreen:
		if g.loadingScreen == nil {
			g.loadingScreen = ui.NewLoadingScreen(g.onGameStart)
		}
		return g.loadingScreen.Update()
	case StateInGame:
		// Game logic will go here
		return nil
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
	case StateInGame:
		// Draw game world
		screen.Fill(color.RGBA{30, 30, 40, 255})
		// TODO: Draw game content
	}
}

// Layout takes the outside size (e.g., the window size) and returns the (logical) screen size.
// If you don't have to adjust the screen size with the outside size, just return a fixed size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 640, 480
}

// onGameStart is called when a new game is started or loaded
func (g *Game) onGameStart(gameState *models.GameState) {
	g.gameState = gameState
	g.state = StateInGame
	log.Printf("Game started: %s (Player: %s, Level: %d)",
		gameState.SaveName, gameState.Player.Stats.Name, gameState.Player.Stats.Level)
}

func main() {
	game := &Game{
		state: StateLoadingScreen,
	}
	// Specify the window size as you like. Here, a doubled size is specified.
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Roadster")
	// Call ebiten.RunGame to start your game loop.
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
