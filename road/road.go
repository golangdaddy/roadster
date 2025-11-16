package road

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// RoadSegment represents a single segment of road with a specific number of lanes
type RoadSegment struct {
	NumLanes int     // Number of lanes in this segment
	StartY   float64 // World Y position where this segment starts
	EndY     float64 // World Y position where this segment ends
}

// Road represents a highway made of segments loaded from a level file
type Road struct {
	Segments      []RoadSegment // All road segments
	LaneWidth    float64       // Width of each lane in pixels
	SegmentHeight float64      // Height of each segment (window height)
}

// LoadRoadFromFile loads a road from a level file
// Each line contains an integer representing the number of lanes for that segment
// Each segment is as long as the window height
func LoadRoadFromFile(filename string, segmentHeight float64, laneWidth float64) (*Road, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open level file: %w", err)
	}
	defer file.Close()

	var segments []RoadSegment
	currentY := 0.0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		numLanes, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("invalid lane count '%s': %w", line, err)
		}

		if numLanes < 1 {
			numLanes = 1
		}

		segment := RoadSegment{
			NumLanes: numLanes,
			StartY:   currentY,
			EndY:     currentY + segmentHeight,
		}
		segments = append(segments, segment)

		currentY += segmentHeight
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading level file: %w", err)
	}

	return &Road{
		Segments:      segments,
		LaneWidth:     laneWidth,
		SegmentHeight: segmentHeight,
	}, nil
}

// GetSegmentAtY returns the road segment at the given world Y position
func (r *Road) GetSegmentAtY(worldY float64) *RoadSegment {
	for i := range r.Segments {
		if worldY >= r.Segments[i].StartY && worldY < r.Segments[i].EndY {
			return &r.Segments[i]
		}
	}
	// If beyond all segments, return the last segment
	if len(r.Segments) > 0 {
		last := &r.Segments[len(r.Segments)-1]
		if worldY >= last.StartY {
			return last
		}
		// If before all segments, return the first
		return &r.Segments[0]
	}
	return nil
}

// GetNumLanesAtY returns the number of lanes at the given world Y position
func (r *Road) GetNumLanesAtY(worldY float64) int {
	segment := r.GetSegmentAtY(worldY)
	if segment == nil {
		return 3 // Default
	}
	return segment.NumLanes
}

// GetRoadWidthAtY returns the total road width at the given world Y position
func (r *Road) GetRoadWidthAtY(worldY float64) float64 {
	return float64(r.GetNumLanesAtY(worldY)) * r.LaneWidth
}

// Draw renders the road on the screen
// cameraX, cameraY are the world positions of the camera (car's position)
func (r *Road) Draw(screen *ebiten.Image, cameraX, cameraY float64) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	screenCenterX := float64(width) / 2
	screenCenterY := float64(height) / 2

	// Don't fill entire screen - let countryside background show through
	// Road surface will be drawn per segment below

	// Calculate visible world Y range
	worldYStart := cameraY - float64(height)/2 - 100
	worldYEnd := cameraY + float64(height)/2 + 100

	// Draw road segments that are visible
	for _, segment := range r.Segments {
		// Check if segment is visible
		// Segment is visible if it overlaps with the visible range
		// Note: segment.StartY < segment.EndY (start is behind, end is ahead)
		// worldYStart < worldYEnd (start is behind, end is ahead)
		segmentVisible := !(segment.EndY < worldYStart || segment.StartY > worldYEnd)
		if !segmentVisible {
			continue
		}

		// Calculate segment bounds in world space
		// Lane 0 starts at X=0, lanes extend to the right (positive X)
		roadWidth := float64(segment.NumLanes) * r.LaneWidth
		roadWorldLeft := 0.0  // Lane 0 starts at X=0
		roadWorldRight := roadWidth  // Road extends to the right

		// Convert segment bounds to screen coordinates
		// World Y increases as car moves forward (toward top of screen)
		// Screen Y increases downward (0 = top, height = bottom)
		// Car is always at screen center (screenCenterY)
		// Things ahead (higher worldY) should appear ABOVE the car (lower screenY)
		// Things behind (lower worldY) should appear BELOW the car (higher screenY)
		// Formula: screenY = screenCenterY - (worldY - cameraY)
		// If worldY = cameraY: screenY = screenCenterY (car position) ✓
		// If worldY > cameraY: screenY < screenCenterY (above car, ahead) ✓
		// If worldY < cameraY: screenY > screenCenterY (below car, behind) ✓
		segmentStartScreenY := screenCenterY - (segment.StartY - cameraY)
		segmentEndScreenY := screenCenterY - (segment.EndY - cameraY)

		// Ensure we draw from top to bottom
		drawStartY := segmentStartScreenY
		drawEndY := segmentEndScreenY
		if drawStartY > drawEndY {
			drawStartY, drawEndY = drawEndY, drawStartY
		}

		// Clamp to screen bounds
		if drawStartY < 0 {
			drawStartY = 0
		}
		if drawEndY > float64(height) {
			drawEndY = float64(height)
		}
		if drawStartY >= drawEndY {
			continue
		}

		// Convert road edges to screen coordinates
		roadLeftScreenX := screenCenterX - (roadWorldLeft - cameraX)
		roadRightScreenX := screenCenterX - (roadWorldRight - cameraX)

		// Ensure left < right (handle coordinate system inversion)
		if roadLeftScreenX > roadRightScreenX {
			roadLeftScreenX, roadRightScreenX = roadRightScreenX, roadLeftScreenX
		}

		// Calculate dimensions
		roadWidthPx := roadRightScreenX - roadLeftScreenX
		roadHeightPx := drawEndY - drawStartY
		
		// Only skip if dimensions are truly invalid
		if roadWidthPx <= 0 || roadHeightPx <= 0 {
			continue
		}
		
		// Clamp road X coordinates to screen bounds if needed
		drawLeftX := roadLeftScreenX
		drawRightX := roadRightScreenX
		if drawRightX < 0 || drawLeftX > float64(width) {
			continue // Entirely off-screen horizontally
		}
		if drawLeftX < 0 {
			drawLeftX = 0
		}
		if drawRightX > float64(width) {
			drawRightX = float64(width)
		}
		roadWidthPx = drawRightX - drawLeftX
		if roadWidthPx <= 0 {
			continue
		}

		// Draw road surface (lighter gray)
		roadColor := color.RGBA{60, 60, 60, 255}
		// Ensure dimensions are valid integers before creating image
		roadWidthInt := int(roadWidthPx)
		roadHeightInt := int(roadHeightPx)
		if roadWidthInt <= 0 || roadHeightInt <= 0 {
			continue
		}
		roadRect := ebiten.NewImage(roadWidthInt, roadHeightInt)
		roadRect.Fill(roadColor)
		roadOp := &ebiten.DrawImageOptions{}
		roadOp.GeoM.Translate(drawLeftX, drawStartY)
		screen.DrawImage(roadRect, roadOp)

		// Draw lane dividers
		dividerColor := color.RGBA{255, 255, 0, 255} // Yellow
		dividerWidth := 2.0
		dividerDashLength := 20.0
		dividerGapLength := 10.0

		for lane := 1; lane < segment.NumLanes; lane++ {
			// Calculate divider position in world space
			// Lane 0 starts at X=0, so divider between lane N and N+1 is at X = N * LaneWidth
			laneDividerWorldX := float64(lane) * r.LaneWidth
			dividerScreenX := screenCenterX - (laneDividerWorldX - cameraX)

			// Draw dashed line
			currentY := drawStartY
			for currentY < drawEndY {
				// Draw dash
				dashEndY := currentY + dividerDashLength
				if dashEndY > drawEndY {
					dashEndY = drawEndY
				}
				dashHeight := dashEndY - currentY
				if dashHeight <= 0 {
					break
				}
				// Ensure dimensions are valid before creating image
				dividerWidthInt := int(dividerWidth)
				dashHeightInt := int(dashHeight)
				if dividerWidthInt <= 0 || dashHeightInt <= 0 {
					break
				}
				dividerRect := ebiten.NewImage(dividerWidthInt, dashHeightInt)
				dividerRect.Fill(dividerColor)
				dividerOp := &ebiten.DrawImageOptions{}
				dividerOp.GeoM.Translate(dividerScreenX-dividerWidth/2, currentY)
				screen.DrawImage(dividerRect, dividerOp)

				// Move to next dash
				currentY = dashEndY + dividerGapLength
			}
		}

		// Draw road edges
		edgeColor := color.RGBA{255, 255, 255, 255} // White
		edgeWidth := 3.0
		edgeHeight := drawEndY - drawStartY
		if edgeHeight > 0 {
			// Ensure dimensions are valid integers
			edgeWidthInt := int(edgeWidth)
			edgeHeightInt := int(edgeHeight)
			if edgeWidthInt > 0 && edgeHeightInt > 0 {
				// Left edge
				leftEdgeRect := ebiten.NewImage(edgeWidthInt, edgeHeightInt)
				leftEdgeRect.Fill(edgeColor)
				leftEdgeOp := &ebiten.DrawImageOptions{}
				leftEdgeOp.GeoM.Translate(drawLeftX-edgeWidth/2, drawStartY)
				screen.DrawImage(leftEdgeRect, leftEdgeOp)

				// Right edge
				rightEdgeRect := ebiten.NewImage(edgeWidthInt, edgeHeightInt)
				rightEdgeRect.Fill(edgeColor)
				rightEdgeOp := &ebiten.DrawImageOptions{}
				rightEdgeOp.GeoM.Translate(drawRightX-edgeWidth/2, drawStartY)
				screen.DrawImage(rightEdgeRect, rightEdgeOp)
			}
		}
	}
}
