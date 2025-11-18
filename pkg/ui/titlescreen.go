package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// TitleScreen represents the main title screen
type TitleScreen struct {
	startTime      time.Time
	onStartPressed func() // Callback when user presses to start
}

// NewTitleScreen creates a new title screen
func NewTitleScreen(onStartPressed func()) *TitleScreen {
	return &TitleScreen{
		startTime:      time.Now(),
		onStartPressed: onStartPressed,
	}
}

// Update handles input for the title screen
func (ts *TitleScreen) Update() error {
	// Any key or mouse click to start
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if ts.onStartPressed != nil {
			ts.onStartPressed()
		}
	}
	return nil
}

// Draw renders the title screen
func (ts *TitleScreen) Draw(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Draw gradient background (dark blue to darker blue)
	screen.Fill(color.RGBA{15, 20, 35, 255})

	// Calculate elapsed time for animations
	elapsed := time.Since(ts.startTime).Seconds()

	// Draw title with pulsing effect
	titleText := "ROADSTER"
	face := text.NewGoXFace(bitmapfont.Face)
	textWidth := text.Advance(titleText, face)
	
	centerX := float64(width) / 2
	centerY := float64(height) / 3
	
	// Pulsing scale effect (1.0 to 1.1)
	pulseScale := 1.0 + 0.1*float32(sinWave(elapsed*2.0))
	titleScale := 8.0 * pulseScale
	
	scaledTextWidth := textWidth * float64(titleScale)
	scaledTextX := centerX - scaledTextWidth/2
	textY := centerY - 8

	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Reset()
	titleOp.GeoM.Scale(float64(titleScale), float64(titleScale))
	titleOp.GeoM.Translate(scaledTextX, textY)
	
	// Gold/yellow color with slight pulsing brightness
	brightness := 1.0 + 0.2*sinWave(elapsed*1.5)
	if brightness > 1.0 {
		brightness = 1.0
	}
	titleColor := color.RGBA{
		uint8(255 * brightness),
		uint8(200 * brightness),
		uint8(50 * brightness),
		255,
	}
	titleOp.ColorScale.ScaleWithColor(titleColor)
	text.Draw(screen, titleText, face, titleOp)

	// Draw subtitle
	subtitleText := "Highway Racing"
	subtitleWidth := text.Advance(subtitleText, face)
	subtitleScale := 2.0
	scaledSubtitleWidth := subtitleWidth * subtitleScale
	subtitleX := centerX - scaledSubtitleWidth/2
	subtitleY := centerY + 80

	subtitleOp := &text.DrawOptions{}
	subtitleOp.GeoM.Scale(subtitleScale, subtitleScale)
	subtitleOp.GeoM.Translate(subtitleX, subtitleY)
	subtitleOp.ColorScale.ScaleWithColor(color.RGBA{180, 180, 200, 255})
	text.Draw(screen, subtitleText, face, subtitleOp)

	// Draw "Press to Start" with blinking effect
	pressText := "Press ENTER or SPACE to Start"
	if int(elapsed*2)%2 == 0 { // Blink every 0.5 seconds
		pressWidth := text.Advance(pressText, face)
		pressScale := 1.5
		scaledPressWidth := pressWidth * pressScale
		pressX := centerX - scaledPressWidth/2
		pressY := float64(height) - 100

		pressOp := &text.DrawOptions{}
		pressOp.GeoM.Scale(pressScale, pressScale)
		pressOp.GeoM.Translate(pressX, pressY)
		pressOp.ColorScale.ScaleWithColor(color.RGBA{150, 200, 255, 255})
		text.Draw(screen, pressText, face, pressOp)
	}

	// Draw decorative elements (simple lines/patterns)
	drawDecorativeElements(screen, width, height, elapsed)
}

// sinWave returns a sine wave value between -1 and 1
func sinWave(t float64) float32 {
	return float32(math.Sin(t))
}

// drawDecorativeElements draws decorative elements on the title screen
func drawDecorativeElements(screen *ebiten.Image, width, height int, elapsed float64) {
	// Draw some simple decorative lines or patterns
	
	// Top decorative line
	lineY1 := float64(height) / 6
	lineY2 := float64(height) * 5 / 6
	
	// Draw lines using filled rectangles
	lineColor := color.RGBA{50, 60, 80, 100}
	lineThickness := 2.0
	
	// Top line
	lineImg1 := ebiten.NewImage(width, int(lineThickness))
	lineImg1.Fill(lineColor)
	op1 := &ebiten.DrawImageOptions{}
	op1.GeoM.Translate(0, lineY1)
	screen.DrawImage(lineImg1, op1)
	
	// Bottom line
	lineImg2 := ebiten.NewImage(width, int(lineThickness))
	lineImg2.Fill(lineColor)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(0, lineY2)
	screen.DrawImage(lineImg2, op2)
}

