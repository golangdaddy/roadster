package road

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Road represents a highway with configurable lanes
type Road struct {
	NumLanes      int     // Number of lanes
	LaneWidth     float64 // Width of each lane in pixels
	RoadWidth     float64 // Total road width
	CenterLine    float64 // Y position of road center line
	SegmentLength float64 // Length of road segments
}

// NewRoad creates a new road with the specified number of lanes
func NewRoad(numLanes int, laneWidth float64) *Road {
	return &Road{
		NumLanes:      numLanes,
		LaneWidth:     laneWidth,
		RoadWidth:     float64(numLanes) * laneWidth,
		CenterLine:    0, // Will be set based on screen
		SegmentLength:  100,
	}
}

// Draw renders the road on the screen
// cameraX, cameraY are the world positions of the camera (car's position)
// Road scrolls in both X and Y as the camera moves
func (r *Road) Draw(screen *ebiten.Image, cameraX, cameraY float64) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2
	
	// Road is centered at world position (0, 0), camera is at (cameraX, cameraY)
	// Road scrolls in both X and Y as camera moves
	
	// Draw road background (dark gray)
	screen.Fill(color.RGBA{40, 40, 40, 255})
	
	// Draw road surface (slightly lighter gray)
	// Road extends infinitely, so we draw multiple segments that scroll in both X and Y
	segmentHeight := 100.0 // Height of each road segment
	segmentWidth := 100.0  // Width of each road segment
	
	// Draw road segments to cover visible area plus extra for scrolling
	worldYStart := cameraY - float64(height)/2 - 100
	worldYEnd := cameraY + float64(height)/2 + 100
	worldXStart := cameraX - float64(width)/2 - 100
	worldXEnd := cameraX + float64(width)/2 + 100
	
	// Draw road segments in a grid
	for worldY := worldYStart; worldY < worldYEnd; worldY += segmentHeight {
		for worldX := worldXStart; worldX < worldXEnd; worldX += segmentWidth {
			// Convert world coordinates to screen coordinates
			screenY := screenCenterY - (worldY - cameraY)
			
			// Only draw if segment is within road bounds and visible
			segmentWorldLeft := worldX
			segmentWorldRight := worldX + segmentWidth
			roadWorldLeft := -r.RoadWidth / 2
			roadWorldRight := r.RoadWidth / 2
			
			// Check if segment overlaps with road
			if segmentWorldRight > roadWorldLeft && segmentWorldLeft < roadWorldRight {
				// Calculate how much of this segment is on the road
				segLeft := segmentWorldLeft
				segRight := segmentWorldRight
				if segLeft < roadWorldLeft {
					segLeft = roadWorldLeft
				}
				if segRight > roadWorldRight {
					segRight = roadWorldRight
				}
				
				segScreenLeft := screenCenterX - (segLeft - cameraX)
				segScreenRight := screenCenterX - (segRight - cameraX)
				segScreenWidth := segScreenRight - segScreenLeft
				
				if segScreenWidth > 0 && screenY >= -segmentHeight && screenY < float64(height)+segmentHeight {
					roadRect := ebiten.NewImage(int(segScreenWidth), int(segmentHeight))
					roadRect.Fill(color.RGBA{60, 60, 60, 255})
					roadOp := &ebiten.DrawImageOptions{}
					roadOp.GeoM.Translate(segScreenLeft, screenY)
					screen.DrawImage(roadRect, roadOp)
				}
			}
		}
	}
	
	// Draw lane dividers (dashed yellow lines)
	dividerColor := color.RGBA{255, 255, 0, 255}
	dividerWidth := 2.0
	dashLength := 20.0
	dashGap := 15.0
	
	// Draw dividers between lanes - scroll in both X and Y
	for i := 1; i < r.NumLanes; i++ {
		// Lane divider is at world X = (i * laneWidth) - roadWidth/2 (relative to road center at 0)
		laneDividerWorldX := float64(i)*r.LaneWidth - r.RoadWidth/2
		
		// Calculate world Y range to draw (around camera position)
		worldYStart := cameraY - float64(height)/2 - 100
		worldYEnd := cameraY + float64(height)/2 + 100
		currentWorldY := worldYStart
		
		// Align dashes to a grid for consistent pattern
		dashPatternLength := dashLength + dashGap
		gridOffset := float64(int(currentWorldY) % int(dashPatternLength))
		if gridOffset < 0 {
			gridOffset += dashPatternLength
		}
		currentWorldY -= gridOffset
		
		for currentWorldY < worldYEnd {
			// Convert world coordinates to screen coordinates
			dividerScreenX := screenCenterX - (laneDividerWorldX - cameraX)
			screenY := screenCenterY - (currentWorldY - cameraY)
			
			// Draw dash if it's visible on screen
			if screenY >= -dashLength && screenY < float64(height)+dashLength &&
				dividerScreenX >= -dividerWidth && dividerScreenX < float64(width)+dividerWidth {
				dividerRect := ebiten.NewImage(int(dividerWidth), int(dashLength))
				dividerRect.Fill(dividerColor)
				dividerOp := &ebiten.DrawImageOptions{}
				dividerOp.GeoM.Translate(dividerScreenX-dividerWidth/2, screenY)
				screen.DrawImage(dividerRect, dividerOp)
			}
			currentWorldY += dashPatternLength
		}
	}
	
	// Draw road edges (white lines) - scroll with camera in both X and Y
	edgeColor := color.RGBA{255, 255, 255, 255}
	edgeWidth := 3.0
	leftEdgeWorldX := -r.RoadWidth / 2
	rightEdgeWorldX := r.RoadWidth / 2
	
	// Draw edge segments
	for worldY := worldYStart; worldY < worldYEnd; worldY += segmentHeight {
		screenY := screenCenterY - (worldY - cameraY)
		if screenY >= -segmentHeight && screenY < float64(height)+segmentHeight {
			// Left edge
			leftEdgeScreenX := screenCenterX - (leftEdgeWorldX - cameraX)
			leftEdge := ebiten.NewImage(int(edgeWidth), int(segmentHeight))
			leftEdge.Fill(edgeColor)
			leftEdgeOp := &ebiten.DrawImageOptions{}
			leftEdgeOp.GeoM.Translate(leftEdgeScreenX-edgeWidth, screenY)
			screen.DrawImage(leftEdge, leftEdgeOp)
			
			// Right edge
			rightEdgeScreenX := screenCenterX - (rightEdgeWorldX - cameraX)
			rightEdge := ebiten.NewImage(int(edgeWidth), int(segmentHeight))
			rightEdge.Fill(edgeColor)
			rightEdgeOp := &ebiten.DrawImageOptions{}
			rightEdgeOp.GeoM.Translate(rightEdgeScreenX, screenY)
			screen.DrawImage(rightEdge, rightEdgeOp)
		}
	}
	
	// Draw center line (double yellow for multi-lane roads) - scroll with camera in both X and Y
	if r.NumLanes > 1 {
		centerLineWidth := 4.0
		centerGap := 2.0
		centerWorldX := 0.0 // Road center is at world X = 0
		
		// Draw center line segments
		for worldY := worldYStart; worldY < worldYEnd; worldY += segmentHeight {
			screenY := screenCenterY - (worldY - cameraY)
			if screenY >= -segmentHeight && screenY < float64(height)+segmentHeight {
				centerScreenX := screenCenterX - (centerWorldX - cameraX)
				
				// Top line (left side of center)
				topLine := ebiten.NewImage(int(centerLineWidth), int(segmentHeight))
				topLine.Fill(dividerColor)
				topLineOp := &ebiten.DrawImageOptions{}
				topLineOp.GeoM.Translate(centerScreenX-centerLineWidth/2-centerGap, screenY)
				screen.DrawImage(topLine, topLineOp)
				
				// Bottom line (right side of center)
				bottomLine := ebiten.NewImage(int(centerLineWidth), int(segmentHeight))
				bottomLine.Fill(dividerColor)
				bottomLineOp := &ebiten.DrawImageOptions{}
				bottomLineOp.GeoM.Translate(centerScreenX+centerGap, screenY)
				screen.DrawImage(bottomLine, bottomLineOp)
			}
		}
	}
}

// GetLaneCenterX returns the X coordinate of the center of a lane (0-indexed)
func (r *Road) GetLaneCenterX(laneIndex int, screenWidth int) float64 {
	roadLeft := float64(screenWidth)/2 - r.RoadWidth/2
	return roadLeft + float64(laneIndex)*r.LaneWidth + r.LaneWidth/2
}

// GetLaneIndex returns which lane a given X coordinate is in (-1 if off road)
func (r *Road) GetLaneIndex(x float64, screenWidth int) int {
	roadLeft := float64(screenWidth)/2 - r.RoadWidth/2
	roadRight := float64(screenWidth)/2 + r.RoadWidth/2
	
	if x < roadLeft || x > roadRight {
		return -1 // Off road
	}
	
	relativeX := x - roadLeft
	laneIndex := int(relativeX / r.LaneWidth)
	
	if laneIndex >= r.NumLanes {
		return -1
	}
	
	return laneIndex
}

