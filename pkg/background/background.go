package background

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

// Generator creates rich background textures
type Generator struct {
	Width  int
	Height int
}

// NewGenerator creates a new background generator
func NewGenerator(width, height int) *Generator {
	return &Generator{
		Width:  width,
		Height: height,
	}
}

// GenerateForest creates a dense forest background
func (g *Generator) GenerateForest(seed int64) *ebiten.Image {
	img := ebiten.NewImage(g.Width, g.Height)
	rng := rand.New(rand.NewSource(seed))

	// Base grass layer (dark rich green)
	img.Fill(color.RGBA{30, 100, 30, 255})

	// Add noise/texture to grass
	for i := 0; i < g.Width*g.Height/10; i++ {
		x := rng.Intn(g.Width)
		y := rng.Intn(g.Height)
		// Varying shades of green
		shade := uint8(80 + rng.Intn(60))
		c := color.RGBA{30, shade, 30, 255}
		img.Set(x, y, c)
	}

	// Draw dense vegetation
	// Draw from top to bottom for correct layering
	for y := 0; y < g.Height; y += 10 {
		// Density varies
		density := 0.5 + 0.3*math.Sin(float64(y)*0.01)
		
		for x := 0; x < g.Width; x += 5 + rng.Intn(15) {
			if rng.Float64() > density {
				continue
			}
			
			// Randomize position slightly
			drawX := x + rng.Intn(10) - 5
			drawY := y + rng.Intn(10) - 5
			
			// Decide type: Tree or Bush
			if rng.Float64() < 0.3 {
				g.drawTree(img, drawX, drawY, rng)
			} else {
				g.drawBush(img, drawX, drawY, rng)
			}
		}
	}

	return img
}

// drawTree draws a simple pine/forest tree
func (g *Generator) drawTree(img *ebiten.Image, x, y int, rng *rand.Rand) {
	height := 40 + rng.Intn(30)
	width := 20 + rng.Intn(15)
	
	// Trunk
	trunkColor := color.RGBA{60, 40, 20, 255}
	trunkW := 4 + rng.Intn(4)
	for ty := 0; ty < height/3; ty++ {
		for tx := -trunkW/2; tx < trunkW/2; tx++ {
			px, py := x+tx, y-ty
			if px >= 0 && px < g.Width && py >= 0 && py < g.Height {
				img.Set(px, py, trunkColor)
			}
		}
	}
	
	// Leaves (Triangle shape)
	leavesColor := color.RGBA{
		uint8(20 + rng.Intn(30)),
		uint8(80 + rng.Intn(60)),
		uint8(20 + rng.Intn(30)),
		255,
	}
	
	layers := 3
	for l := 0; l < layers; l++ {
		layerY := y - (height/3) - (l * height / 4)
		layerW := width - (l * 5)
		if layerW < 5 { layerW = 5 }
		
		for ly := 0; ly < height/3; ly++ {
			rowW := layerW * (height/3 - ly) / (height/3)
			for lx := -rowW/2; lx < rowW/2; lx++ {
				px, py := x+lx, layerY-ly
				if px >= 0 && px < g.Width && py >= 0 && py < g.Height {
					img.Set(px, py, leavesColor)
				}
			}
		}
	}
}

// drawBush draws a round bush
func (g *Generator) drawBush(img *ebiten.Image, x, y int, rng *rand.Rand) {
	radius := 5 + rng.Intn(10)
	c := color.RGBA{
		uint8(40 + rng.Intn(40)),
		uint8(100 + rng.Intn(50)),
		uint8(40 + rng.Intn(40)),
		255,
	}
	
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx + dy*dy <= radius*radius {
				px, py := x+dx, y+dy
				if px >= 0 && px < g.Width && py >= 0 && py < g.Height {
					img.Set(px, py, c)
				}
			}
		}
	}
}

