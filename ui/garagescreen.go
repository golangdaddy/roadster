package ui

import (
	"fmt"
	"image/color"

	"github.com/golangdaddy/roadster/models"
	"github.com/golangdaddy/roadster/models/car"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// GarageScreen represents the car selection screen
type GarageScreen struct {
	selectedCarIndex int
	onCarSelected    func(*car.Car) // Callback when car is selected
}

// NewGarageScreen creates a new garage selection screen
func NewGarageScreen(onCarSelected func(*car.Car)) *GarageScreen {
	return &GarageScreen{
		selectedCarIndex: 0,
		onCarSelected:    onCarSelected,
	}
}

// Update handles input for the garage screen
func (gs *GarageScreen) Update() error {
	cars := models.CarInventory.GetAllCars()
	if len(cars) == 0 {
		return nil
	}

	// Handle keyboard navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		gs.selectedCarIndex--
		if gs.selectedCarIndex < 0 {
			gs.selectedCarIndex = len(cars) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		gs.selectedCarIndex++
		if gs.selectedCarIndex >= len(cars) {
			gs.selectedCarIndex = 0
		}
	}

	// Handle selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if gs.selectedCarIndex >= 0 && gs.selectedCarIndex < len(cars) {
			selectedCar := cars[gs.selectedCarIndex]
			if gs.onCarSelected != nil {
				gs.onCarSelected(selectedCar)
			}
		}
	}

	return nil
}

// Draw renders the garage screen
func (gs *GarageScreen) Draw(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	
	// Draw background
	screen.Fill(color.RGBA{20, 20, 30, 255})

	// Title
	titleText := "SELECT CAR"
	face := text.NewGoXFace(bitmapfont.Face)
	textWidth := text.Advance(titleText, face)
	titleScale := 4.0
	scaledTextWidth := textWidth * titleScale
	centerX := float64(width) / 2
	titleX := centerX - scaledTextWidth/2
	titleY := 50.0
	
	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Scale(titleScale, titleScale)
	titleOp.GeoM.Translate(titleX, titleY)
	titleOp.ColorScale.ScaleWithColor(color.RGBA{255, 200, 50, 255})
	text.Draw(screen, titleText, face, titleOp)

	// Draw car list
	cars := models.CarInventory.GetAllCars()
	if len(cars) == 0 {
		drawText(screen, "No cars available", centerX, float64(height)/2, 24, color.RGBA{255, 255, 255, 255})
		return
	}

	// Car list starting position
	startY := 150.0
	carSpacing := 80.0
	buttonWidth := 600.0
	buttonHeight := 60.0
	buttonX := centerX - buttonWidth/2

	for i, c := range cars {
		carY := startY + float64(i)*carSpacing
		
		// Car info text
		carInfo := formatCarInfo(c)
		
		// Button colors
		bgColor := color.RGBA{40, 40, 60, 255}
		textColor := color.RGBA{255, 255, 255, 255}
		if i == gs.selectedCarIndex {
			bgColor = color.RGBA{60, 100, 140, 255}
			textColor = color.RGBA{200, 240, 255, 255}
		}
		
		drawButton(screen, carInfo, buttonX, carY, buttonWidth, buttonHeight, bgColor, textColor)
	}

	// Instructions
	drawText(screen, "Arrow Keys: Navigate | Enter: Select", centerX, float64(height)-50, 20, color.RGBA{150, 150, 150, 255})
}

// formatCarInfo formats car information for display
func formatCarInfo(c *car.Car) string {
	brakeEfficiency := c.Brakes.Condition * c.Brakes.Performance * c.Brakes.StoppingPower
	return fmt.Sprintf("%s %s (%d) - Weight: %.0f kg | Brake Eff: %.1f%% | Type: %s",
		c.Make, c.Model, c.Year, c.Weight, brakeEfficiency*100, c.Brakes.Type)
}

