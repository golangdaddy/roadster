package car

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// RenderCar renders a top-down view of the car
func RenderCar(screen *ebiten.Image, x, y, angle float64, carColor color.Color) {
	// Car dimensions
	carWidth := 30.0
	carHeight := 50.0
	
	// Create car image
	carImg := ebiten.NewImage(int(carWidth), int(carHeight))
	
	// Draw car body (rectangle)
	carImg.Fill(carColor)
	
	// Draw car outline (darker color)
	outlineColor := color.RGBA{20, 20, 20, 255}
	outlineWidth := 2.0
	
	// Top edge
	topLine := ebiten.NewImage(int(carWidth), int(outlineWidth))
	topLine.Fill(outlineColor)
	topOp := &ebiten.DrawImageOptions{}
	topOp.GeoM.Translate(0, 0)
	carImg.DrawImage(topLine, topOp)
	
	// Bottom edge
	bottomLine := ebiten.NewImage(int(carWidth), int(outlineWidth))
	bottomLine.Fill(outlineColor)
	bottomOp := &ebiten.DrawImageOptions{}
	bottomOp.GeoM.Translate(0, carHeight-outlineWidth)
	carImg.DrawImage(bottomLine, bottomOp)
	
	// Left edge
	leftLine := ebiten.NewImage(int(outlineWidth), int(carHeight))
	leftLine.Fill(outlineColor)
	leftOp := &ebiten.DrawImageOptions{}
	leftOp.GeoM.Translate(0, 0)
	carImg.DrawImage(leftLine, leftOp)
	
	// Right edge
	rightLine := ebiten.NewImage(int(outlineWidth), int(carHeight))
	rightLine.Fill(outlineColor)
	rightOp := &ebiten.DrawImageOptions{}
	rightOp.GeoM.Translate(carWidth-outlineWidth, 0)
	carImg.DrawImage(rightLine, rightOp)
	
	// Draw windshield (front of car - lighter rectangle)
	windshieldColor := color.RGBA{150, 200, 255, 200}
	windshieldWidth := carWidth * 0.6
	windshieldHeight := carHeight * 0.2
	windshield := ebiten.NewImage(int(windshieldWidth), int(windshieldHeight))
	windshield.Fill(windshieldColor)
	windshieldOp := &ebiten.DrawImageOptions{}
	windshieldOp.GeoM.Translate((carWidth-windshieldWidth)/2, 0)
	carImg.DrawImage(windshield, windshieldOp)
	
	// Draw wheels (simple circles as rectangles)
	wheelColor := color.RGBA{30, 30, 30, 255}
	wheelWidth := 6.0
	wheelHeight := 8.0
	
	// Front left wheel
	frontLeftWheel := ebiten.NewImage(int(wheelWidth), int(wheelHeight))
	frontLeftWheel.Fill(wheelColor)
	frontLeftOp := &ebiten.DrawImageOptions{}
	frontLeftOp.GeoM.Translate(2, 5)
	carImg.DrawImage(frontLeftWheel, frontLeftOp)
	
	// Front right wheel
	frontRightWheel := ebiten.NewImage(int(wheelWidth), int(wheelHeight))
	frontRightWheel.Fill(wheelColor)
	frontRightOp := &ebiten.DrawImageOptions{}
	frontRightOp.GeoM.Translate(carWidth-wheelWidth-2, 5)
	carImg.DrawImage(frontRightWheel, frontRightOp)
	
	// Rear left wheel
	rearLeftWheel := ebiten.NewImage(int(wheelWidth), int(wheelHeight))
	rearLeftWheel.Fill(wheelColor)
	rearLeftOp := &ebiten.DrawImageOptions{}
	rearLeftOp.GeoM.Translate(2, carHeight-wheelHeight-5)
	carImg.DrawImage(rearLeftWheel, rearLeftOp)
	
	// Rear right wheel
	rearRightWheel := ebiten.NewImage(int(wheelWidth), int(wheelHeight))
	rearRightWheel.Fill(wheelColor)
	rearRightOp := &ebiten.DrawImageOptions{}
	rearRightOp.GeoM.Translate(carWidth-wheelWidth-2, carHeight-wheelHeight-5)
	carImg.DrawImage(rearRightWheel, rearRightOp)
	
	// Draw car on screen with rotation
	op := &ebiten.DrawImageOptions{}
	
	// Center the car image for rotation
	op.GeoM.Translate(-carWidth/2, -carHeight/2)
	
	// Rotate (angle in degrees, converted to radians)
	// For top-down view, angle 0 = facing up (north/top of screen)
	// Add 180 degrees to flip car to face upward (bonnet at top)
	op.GeoM.Rotate((angle + 180) * 3.14159 / 180.0)
	
	// Rotate another 180 degrees so bonnet faces upward
	op.GeoM.Rotate(180 * 3.14159 / 180.0)
	
	// Translate to final position
	op.GeoM.Translate(x, y)
	
	screen.DrawImage(carImg, op)
}

