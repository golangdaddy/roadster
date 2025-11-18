package ui

import (
	"image/color"
	"time"

	"github.com/golangdaddy/roadster/pkg/models"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/bitmapfont/v4"
)

// LoadingScreen represents the main menu/loading screen
type LoadingScreen struct {
	selectedOption int // 0 = New Game, 1 = Load Game
	lastInputTime  time.Time
	gameState      *models.GameState
	onGameStart    func(*models.GameState) // Callback when game starts
}

// NewLoadingScreen creates a new loading screen
func NewLoadingScreen(onGameStart func(*models.GameState)) *LoadingScreen {
	return &LoadingScreen{
		selectedOption: 0,
		lastInputTime:  time.Now(),
		onGameStart:    onGameStart,
	}
}

// Update handles input for the loading screen
func (ls *LoadingScreen) Update() error {
	// Handle keyboard navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		ls.selectedOption = 1 - ls.selectedOption // Toggle between 0 and 1
	}

	// Handle selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if ls.selectedOption == 0 {
			// New Game
			ls.startNewGame()
		} else {
			// Load Game (placeholder - would show save file list)
			ls.loadGame()
		}
	}

	return nil
}

// Draw renders the loading screen
func (ls *LoadingScreen) Draw(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	
	// Draw background
	screen.Fill(color.RGBA{20, 20, 30, 255})

	// Title - use exact same approach as button text, but with scaling
	titleText := "ROADSTER"
	face := text.NewGoXFace(bitmapfont.Face)
	
	// Get text width at natural size (16px) - same as buttons
	textWidth := text.Advance(titleText, face)
	
	// Calculate center position - same as buttons
	centerX := float64(width) / 2
	centerY := float64(height) / 4
	
	// Calculate position for scaled text (same logic as buttons)
	titleScale := 6.0
	scaledTextWidth := textWidth * titleScale
	scaledTextX := centerX - scaledTextWidth/2  // Left edge to center horizontally
	textY := centerY - 8  // Same vertical offset as buttons
	
	titleOp := &text.DrawOptions{}
	// Reset to ensure clean transform
	titleOp.GeoM.Reset()
	// Scale first (scales around origin), then translate to final position
	// Translation is in screen coordinates, so use the calculated position directly
	titleOp.GeoM.Scale(titleScale, titleScale)
	titleOp.GeoM.Translate(scaledTextX, textY)
	titleOp.ColorScale.ScaleWithColor(color.RGBA{255, 200, 50, 255})
	text.Draw(screen, titleText, face, titleOp)
	
	// Menu options - adjusted for 1024x600 resolution
	buttonWidth := 300.0
	buttonHeight := 50.0
	optionY := float64(height) / 2
	optionSpacing := 80.0
	buttonX := float64(width)/2 - buttonWidth/2
	
	// New Game button
	newGameBgColor := color.RGBA{40, 40, 60, 255}
	newGameTextColor := color.RGBA{255, 255, 255, 255}
	if ls.selectedOption == 0 {
		newGameBgColor = color.RGBA{60, 100, 140, 255} // Highlighted background
		newGameTextColor = color.RGBA{200, 240, 255, 255} // Highlighted text
	}
	drawButton(screen, "New Game", buttonX, optionY, buttonWidth, buttonHeight, newGameBgColor, newGameTextColor)
	
	// Load Game button
	loadGameBgColor := color.RGBA{40, 40, 60, 255}
	loadGameTextColor := color.RGBA{255, 255, 255, 255}
	if ls.selectedOption == 1 {
		loadGameBgColor = color.RGBA{60, 100, 140, 255} // Highlighted background
		loadGameTextColor = color.RGBA{200, 240, 255, 255} // Highlighted text
	}
	drawButton(screen, "Load Game", buttonX, optionY+optionSpacing, buttonWidth, buttonHeight, loadGameBgColor, loadGameTextColor)
	
	// Instructions - centered horizontally, adjusted for new resolution
	drawText(screen, "Arrow Keys: Navigate | Enter: Select", float64(width)/2, float64(height)-50, 20, color.RGBA{150, 150, 150, 255})
}

// startNewGame creates a new game state
func (ls *LoadingScreen) startNewGame() {
	// Create a new game with default player name
	gameState := models.NewGameState("Save_"+time.Now().Format("20060102_150405"), "Player")
	ls.gameState = gameState
	
	// Call the callback to start the game
	if ls.onGameStart != nil {
		ls.onGameStart(gameState)
	}
}

// loadGame loads an existing game (placeholder implementation)
func (ls *LoadingScreen) loadGame() {
	// TODO: Implement save file selection UI
	// For now, try to load a default save file
	filename := "save.json"
	gameState, err := models.LoadFromFile(filename)
	if err != nil {
		// If no save file exists, create a new game instead
		ls.startNewGame()
		return
	}
	
	ls.gameState = gameState
	if ls.onGameStart != nil {
		ls.onGameStart(gameState)
	}
}

// drawButton draws a button with background and text
func drawButton(screen *ebiten.Image, label string, x, y, width, height float64, bgColor, textColor color.Color) {
	// Draw button background
	buttonImg := ebiten.NewImage(int(width), int(height))
	buttonImg.Fill(bgColor)
	
	// Draw border (2px border)
	borderColor := color.RGBA{80, 80, 100, 255}
	borderWidth := 2
	w, h := int(width), int(height)
	
	// Top and bottom borders
	for i := 0; i < w; i++ {
		for j := 0; j < borderWidth; j++ {
			buttonImg.Set(i, j, borderColor)
			buttonImg.Set(i, h-1-j, borderColor)
		}
	}
	// Left and right borders
	for i := 0; i < h; i++ {
		for j := 0; j < borderWidth; j++ {
			buttonImg.Set(j, i, borderColor)
			buttonImg.Set(w-1-j, i, borderColor)
		}
	}
	
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(buttonImg, op)
	
	// Draw text centered on button - simplified approach
	face := text.NewGoXFace(bitmapfont.Face)
	
	// Get text width at natural size (16px for bitmap font)
	textWidth := text.Advance(label, face)
	
	// Calculate center position of button
	centerX := x + width/2
	centerY := y + height/2
	
	// Center text horizontally
	textX := centerX - textWidth/2
	
	// Center text vertically - bitmap font is 16px tall
	// Position baseline so text center aligns with button center
	// Text height is ~16px, so center is ~8px from baseline
	textY := centerY - 8
	
	textOp := &text.DrawOptions{}
	textOp.GeoM.Translate(textX, textY)
	textOp.ColorScale.ScaleWithColor(textColor)
	text.Draw(screen, label, face, textOp)
}

// drawText draws text on the screen using ebiten's bitmap font
// x, y is the center point where text should be positioned
func drawText(screen *ebiten.Image, str string, centerX, centerY float64, size float64, clr color.Color) {
	// Use bitmapfont which is included with ebiten
	face := text.NewGoXFace(bitmapfont.Face)
	
	// Get text width at natural size
	textWidth := text.Advance(str, face)
	scale := size / 16.0
	scaledWidth := textWidth * scale
	
	// Calculate position so text is centered at (centerX, centerY)
	// Center horizontally
	textX := centerX - scaledWidth/2
	
	// Center vertically - text height at scaled size
	// Position baseline so text center aligns with centerY
	scaledHeight := 16.0 * scale
	textY := centerY - scaledHeight/2 + 8 // Adjust for baseline
	
	op := &text.DrawOptions{}
	// Apply scaling, then translate to position
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(textX/scale, textY/scale)
	op.ColorScale.ScaleWithColor(clr)
	
	// Draw the text
	text.Draw(screen, str, face, op)
}

