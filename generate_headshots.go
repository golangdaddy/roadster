package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

type HeadshotGenerator struct{}

func (hg *HeadshotGenerator) generateHeadshot(name string, skinColor, hairColor color.RGBA) *image.RGBA {
	// Create 128x128 image for high-quality headshot
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))

	// Define colors
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	backgroundColor := color.RGBA{245, 245, 250, 255}

	// Fill background
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, backgroundColor)
		}
	}

	// Define head shape (centered, larger)
	centerX, centerY := 64, 64
	headRadius := 40

	// Draw head outline (black border)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := x - centerX
			dy := y - centerY
			dist := dx*dx + dy*dy
			
			// Head outline (slightly oval)
			if dist >= (headRadius-2)*(headRadius-2) && dist <= headRadius*headRadius {
				img.Set(x, y, black)
			}
		}
	}

	// Fill head with skin color
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := x - centerX
			dy := y - centerY
			dist := dx*dx + dy*dy
			
			// Head fill (slightly oval)
			if dist < (headRadius-2)*(headRadius-2) {
				img.Set(x, y, skinColor)
			}
		}
	}

	// Draw hair (above head, more detailed)
	hairStartY := centerY - headRadius - 8
	hairEndY := centerY - headRadius + 12
	
	for y := hairStartY; y < hairEndY; y++ {
		for x := centerX - headRadius - 4; x < centerX + headRadius + 4; x++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				// Create wavy hair effect
				wave := int(float64(x-centerX) * 0.3)
				if (y-hairStartY+wave)%4 < 2 {
					img.Set(x, y, hairColor)
				}
			}
		}
	}

	// Draw side hair (left and right)
	for y := centerY - headRadius + 5; y < centerY + headRadius - 5; y++ {
		// Left side
		for x := centerX - headRadius - 2; x < centerX - headRadius + 4; x++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				img.Set(x, y, hairColor)
			}
		}
		// Right side
		for x := centerX + headRadius - 4; x < centerX + headRadius + 2; x++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				img.Set(x, y, hairColor)
			}
		}
	}

	// Draw eyes (more detailed)
	eyeY := centerY - 8
	leftEyeX := centerX - 12
	rightEyeX := centerX + 12
	
	// Eye whites
	for dy := -3; dy <= 3; dy++ {
		for dx := -4; dx <= 4; dx++ {
			if dx*dx + dy*dy <= 9 {
				if leftEyeX+dx >= 0 && leftEyeX+dx < 128 && eyeY+dy >= 0 && eyeY+dy < 128 {
					img.Set(leftEyeX+dx, eyeY+dy, white)
				}
				if rightEyeX+dx >= 0 && rightEyeX+dx < 128 && eyeY+dy >= 0 && eyeY+dy < 128 {
					img.Set(rightEyeX+dx, eyeY+dy, white)
				}
			}
		}
	}
	
	// Eye pupils
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx*dx + dy*dy <= 4 {
				if leftEyeX+dx >= 0 && leftEyeX+dx < 128 && eyeY+dy >= 0 && eyeY+dy < 128 {
					img.Set(leftEyeX+dx, eyeY+dy, black)
				}
				if rightEyeX+dx >= 0 && rightEyeX+dx < 128 && eyeY+dy >= 0 && eyeY+dy < 128 {
					img.Set(rightEyeX+dx, eyeY+dy, black)
				}
			}
		}
	}

	// Draw eyebrows
	eyebrowY := eyeY - 8
	for x := leftEyeX - 6; x <= leftEyeX + 6; x++ {
		for y := eyebrowY; y < eyebrowY + 2; y++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				img.Set(x, y, hairColor)
			}
		}
	}
	for x := rightEyeX - 6; x <= rightEyeX + 6; x++ {
		for y := eyebrowY; y < eyebrowY + 2; y++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				img.Set(x, y, hairColor)
			}
		}
	}

	// Draw nose (subtle)
	noseY := centerY + 2
	noseX := centerX
	for dy := 0; dy <= 6; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx*dx + dy*dy <= 4 {
				if noseX+dx >= 0 && noseX+dx < 128 && noseY+dy >= 0 && noseY+dy < 128 {
					// Slightly darker skin for nose shadow
					r, g, b, _ := skinColor.RGBA()
					darkerSkin := color.RGBA{
						uint8(r>>8) - 15,
						uint8(g>>8) - 15,
						uint8(b>>8) - 15,
						255,
					}
					img.Set(noseX+dx, noseY+dy, darkerSkin)
				}
			}
		}
	}

	// Draw mouth (smile)
	mouthY := centerY + 12
	mouthWidth := 12
	for x := centerX - mouthWidth/2; x <= centerX + mouthWidth/2; x++ {
		// Smile curve
		curve := int(float64((x-centerX)*(x-centerX)) * 0.1)
		y := mouthY + curve/3
		if x >= 0 && x < 128 && y >= 0 && y < 128 {
			img.Set(x, y, black)
		}
		// Smile corners
		if x == centerX - mouthWidth/2 || x == centerX + mouthWidth/2 {
			if y+1 >= 0 && y+1 < 128 {
				img.Set(x, y+1, black)
			}
		}
	}

	// Add some shading for depth
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := x - centerX
			dy := y - centerY
			dist := dx*dx + dy*dy
			
			// Add subtle shadow on left side of face
			if dist < (headRadius-2)*(headRadius-2) && x < centerX {
				r, g, b, a := img.At(x, y).RGBA()
				if r > 0 || g > 0 || b > 0 {
					img.Set(x, y, color.RGBA{
						uint8(r>>8) - 8,
						uint8(g>>8) - 8,
						uint8(b>>8) - 8,
						uint8(a>>8),
					})
				}
			}
		}
	}

	return img
}

func (hg *HeadshotGenerator) saveHeadshot(img image.Image, filename string) error {
	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode as PNG
	return png.Encode(file, img)
}

func main() {
	hg := &HeadshotGenerator{}

	// Define headshot variations (same colors as the full characters)
	characters := []struct {
		name      string
		skinColor color.RGBA
		hairColor color.RGBA
	}{
		// Women
		{"woman1", color.RGBA{255, 220, 180, 255}, color.RGBA{100, 50, 30, 255}},   // Brunette
		{"woman2", color.RGBA{240, 210, 160, 255}, color.RGBA{220, 180, 50, 255}},  // Blonde
		{"woman3", color.RGBA{200, 150, 120, 255}, color.RGBA{50, 30, 80, 255}},    // Dark hair
		{"woman4", color.RGBA{255, 210, 170, 255}, color.RGBA{180, 80, 40, 255}},   // Red hair

		// Men
		{"man1", color.RGBA{220, 180, 150, 255}, color.RGBA{60, 40, 20, 255}},       // Dark hair
		{"man2", color.RGBA{240, 200, 160, 255}, color.RGBA{150, 100, 50, 255}},    // Brown hair
		{"man3", color.RGBA{200, 160, 130, 255}, color.RGBA{30, 20, 10, 255}},       // Very dark hair
		{"man4", color.RGBA{250, 220, 180, 255}, color.RGBA{100, 60, 30, 255}},      // Brown hair
	}

	// Create assets/characters/headshots directory if it doesn't exist
	err := os.MkdirAll("assets/characters/headshots", 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	// Generate and save each headshot
	for _, char := range characters {
		img := hg.generateHeadshot(char.name, char.skinColor, char.hairColor)

		filename := filepath.Join("assets", "characters", "headshots", char.name+"_headshot.png")
		err := hg.saveHeadshot(img, filename)
		if err != nil {
			fmt.Printf("Error saving %s: %v\n", filename, err)
			continue
		}

		fmt.Printf("Generated headshot: %s\n", filename)
	}

	fmt.Println("Headshot generation complete!")
}