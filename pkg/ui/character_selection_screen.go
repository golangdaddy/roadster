package ui

import (
	"image/color"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/golangdaddy/roadster/pkg/data"
	"github.com/golangdaddy/roadster/pkg/models/profile"
	"github.com/hajimehoshi/bitmapfont/v4"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type CharacterOption struct {
	Name         string
	AvatarPath   string
	HeadshotPath string
	Headshot     *ebiten.Image
}

type CharacterSelectionScreen struct {
	onProfileCreated func(*profile.PlayerProfile)
	options          []CharacterOption
	selectedIndex    int
	
	// UI State
	initialized bool
}

func NewCharacterSelectionScreen(onProfileCreated func(*profile.PlayerProfile)) *CharacterSelectionScreen {
	rand.Seed(time.Now().UnixNano())
	
	screen := &CharacterSelectionScreen{
		onProfileCreated: onProfileCreated,
		options:          make([]CharacterOption, 0),
	}
	
	// Generate options based on assets
	// We know we have woman1-4 and man1-4
	
	// Women
	for i := 1; i <= 4; i++ {
		name := data.CommonNames.Female[rand.Intn(len(data.CommonNames.Female))]
		charID := "woman" + string(rune('0'+i))
		
		screen.options = append(screen.options, CharacterOption{
			Name:         name,
			AvatarPath:   filepath.Join("assets", "characters", charID+".png"),
			HeadshotPath: filepath.Join("assets", "characters", "headshots", charID+"_headshot.png"),
		})
	}
	
	// Men
	for i := 1; i <= 4; i++ {
		name := data.CommonNames.Male[rand.Intn(len(data.CommonNames.Male))]
		charID := "man" + string(rune('0'+i))
		
		screen.options = append(screen.options, CharacterOption{
			Name:         name,
			AvatarPath:   filepath.Join("assets", "characters", charID+".png"),
			HeadshotPath: filepath.Join("assets", "characters", "headshots", charID+"_headshot.png"),
		})
	}
	
	return screen
}

func (cs *CharacterSelectionScreen) Update() error {
	// Initialize images if needed
	if !cs.initialized {
		for i := range cs.options {
			img, _, err := ebitenutil.NewImageFromFile(cs.options[i].HeadshotPath)
			if err == nil {
				cs.options[i].Headshot = img
			}
		}
		cs.initialized = true
	}
	
	// Navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		cs.selectedIndex--
		if cs.selectedIndex < 0 {
			cs.selectedIndex = len(cs.options) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		cs.selectedIndex++
		if cs.selectedIndex >= len(cs.options) {
			cs.selectedIndex = 0
		}
	}
	
	// Selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		selected := cs.options[cs.selectedIndex]
		profile := profile.NewProfile(selected.Name, selected.AvatarPath, selected.HeadshotPath)
		if cs.onProfileCreated != nil {
			cs.onProfileCreated(profile)
		}
	}
	
	return nil
}

func (cs *CharacterSelectionScreen) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 40, 255}) // Dark background
	
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	face := text.NewGoXFace(bitmapfont.Face)
	
	// Title
	title := "SELECT YOUR DRIVER"
	titleW := text.Advance(title, face) * 3
	
	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Scale(3, 3)
	titleOp.GeoM.Translate(float64(w)/2 - titleW/2, 50)
	titleOp.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, title, face, titleOp)
	
	// Grid layout for characters
	cols := 4
	
	gridStartX := float64(w)/2 - 300
	gridStartY := 150.0
	cellW := 150.0
	cellH := 180.0
	
	for i, opt := range cs.options {
		row := i / cols
		col := i % cols
		
		x := gridStartX + float64(col)*cellW
		y := gridStartY + float64(row)*cellH
		
		// Selection highlight
		if i == cs.selectedIndex {
			highlight := ebiten.NewImage(130, 160)
			highlight.Fill(color.RGBA{255, 215, 0, 100}) // Gold highlight
			
			hlOp := &ebiten.DrawImageOptions{}
			hlOp.GeoM.Translate(x-5, y-5)
			screen.DrawImage(highlight, hlOp)
		}
		
		// Draw Headshot
		if opt.Headshot != nil {
			op := &ebiten.DrawImageOptions{}
			// Headshots are 128x128, maybe scale down slightly?
			// Actually 128x128 fits nicely in 150x180 cell
			op.GeoM.Scale(0.8, 0.8) // Scale to ~100x100
			op.GeoM.Translate(x+10, y)
			screen.DrawImage(opt.Headshot, op)
		} else {
			// Placeholder box
			ph := ebiten.NewImage(100, 100)
			ph.Fill(color.RGBA{100, 100, 100, 255})
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(x+10, y)
			screen.DrawImage(ph, op)
		}
		
		// Name
		nameW := text.Advance(opt.Name, face)
		nameOp := &text.DrawOptions{}
		nameOp.GeoM.Translate(x + 60 - nameW/2, y + 110)
		
		if i == cs.selectedIndex {
			nameOp.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 255})
		} else {
			nameOp.ColorScale.ScaleWithColor(color.White)
		}
		
		text.Draw(screen, opt.Name, face, nameOp)
	}
	
	// Instructions
	instr := "ARROWS to Select   ENTER to Confirm"
	instrW := text.Advance(instr, face) * 1.5
	
	instrOp := &text.DrawOptions{}
	instrOp.GeoM.Scale(1.5, 1.5)
	instrOp.GeoM.Translate(float64(w)/2 - instrW/2, float64(h) - 50)
	instrOp.ColorScale.ScaleWithColor(color.RGBA{200, 200, 200, 255})
	text.Draw(screen, instr, face, instrOp)
}

