package main

import (
	"log"

	"github.com/golangdaddy/roadster/pkg/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	// Create the game instance
	g := game.NewGame()

	// Set up Ebiten game settings
	ebiten.SetWindowSize(1024, 600)
	ebiten.SetWindowTitle("ROADSTER - Highway Racing")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	// Run the game
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
