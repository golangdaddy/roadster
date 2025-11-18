package ui

import (
	"fmt"
	"image/color"

	"github.com/golangdaddy/roadster/pkg/models/car"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// PetrolStationScreen represents the petrol station refueling screen
type PetrolStationScreen struct {
	carModel      *car.Car
	onExit        func()
	litersToAdd   float64
	maxLiters     float64
}

// NewPetrolStationScreen creates a new petrol station screen
func NewPetrolStationScreen(carModel *car.Car, onExit func()) *PetrolStationScreen {
	if carModel == nil || carModel.FuelCapacity <= 0 {
		return nil
	}
	
	// Calculate how much fuel can be added
	currentLiters := carModel.FuelLevel * carModel.FuelCapacity
	maxLiters := carModel.FuelCapacity
	litersToAdd := maxLiters - currentLiters
	
	return &PetrolStationScreen{
		carModel:    carModel,
		onExit:      onExit,
		litersToAdd: litersToAdd,
		maxLiters:   maxLiters,
	}
}

// Update handles input for the petrol station screen
func (ps *PetrolStationScreen) Update() error {
	if ps.carModel == nil {
		return nil
	}

	// Check for exit (Escape key)
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if ps.onExit != nil {
			ps.onExit()
		}
		return nil
	}

	// Refuel on Enter/Space
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		// Fill tank to full
		ps.carModel.FuelLevel = 1.0
		// Exit after refueling
		if ps.onExit != nil {
			ps.onExit()
		}
		return nil
	}

	return nil
}

// Draw renders the petrol station screen
func (ps *PetrolStationScreen) Draw(screen *ebiten.Image) {
	width := screen.Bounds().Dx()
	
	// Background (dark gray)
	screen.Fill(color.RGBA{40, 40, 50, 255})

	face := text.NewGoXFace(bitmapfont.Face)
	
	// Title
	titleColor := color.RGBA{255, 200, 0, 255} // Yellow/gold
	titleText := "PETROL STATION"
	titleSize := 32.0
	titleWidth := text.Advance(titleText, face) * (titleSize / 16.0)
	titleX := float64(width)/2 - titleWidth/2
	titleY := 80.0
	drawTextAt(screen, titleText, titleX, titleY, titleSize, titleColor, face)

	// Car info
	textColor := color.RGBA{200, 200, 200, 255}
	lineHeight := 30.0
	currentY := 150.0
	startX := float64(width) / 2 - 200.0

	if ps.carModel != nil {
		carInfoText := fmt.Sprintf("Car: %s %s", ps.carModel.Make, ps.carModel.Model)
		drawTextAt(screen, carInfoText, startX, currentY, 18, textColor, face)
		currentY += lineHeight

		// Current fuel
		currentLiters := ps.carModel.FuelLevel * ps.carModel.FuelCapacity
		fuelText := fmt.Sprintf("Current Fuel: %.1f / %.1f L (%.1f%%)",
			currentLiters, ps.carModel.FuelCapacity, ps.carModel.FuelLevel*100)
		drawTextAt(screen, fuelText, startX, currentY, 18, textColor, face)
		currentY += lineHeight * 1.5

		// Fuel to add
		if ps.litersToAdd > 0.01 {
			addText := fmt.Sprintf("Fuel to add: %.1f L", ps.litersToAdd)
			drawTextAt(screen, addText, startX, currentY, 18, textColor, face)
		} else {
			fullText := "Tank is full!"
			fullColor := color.RGBA{100, 255, 100, 255} // Green
			drawTextAt(screen, fullText, startX, currentY, 18, fullColor, face)
		}
		currentY += lineHeight * 2
	}

	// Instructions
	instructionColor := color.RGBA{150, 150, 200, 255}
	instructionText := "Press ENTER or SPACE to refuel"
	drawTextAt(screen, instructionText, startX, currentY, 16, instructionColor, face)
	currentY += lineHeight

	exitText := "Press ESCAPE to exit without refueling"
	drawTextAt(screen, exitText, startX, currentY, 14, instructionColor, face)
}

// drawTextAt draws text at a specific position (helper function)
func drawTextAt(screen *ebiten.Image, str string, x, y float64, size float64, clr color.Color, face text.Face) {
	scale := size / 16.0
	scaledHeight := 16.0 * scale

	textX := x
	textY := y - scaledHeight/2 + 8

	op := &text.DrawOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(textX/scale, textY/scale)
	op.ColorScale.ScaleWithColor(clr)

	text.Draw(screen, str, face, op)
}

