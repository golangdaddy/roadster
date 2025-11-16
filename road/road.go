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
	NumLanes           int     // Number of lanes in this segment (includes petrol station lane if present)
	StartY             float64 // World Y position where this segment starts
	EndY               float64 // World Y position where this segment ends
	HasPetrolStationLane bool  // Whether this segment has a petrol station lane (40mph) on the right (lane 0)
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

		// Check for 'P' suffix indicating petrol station lane
		hasPetrolStation := false
		laneStr := line
		if len(line) > 0 && line[len(line)-1] == 'P' {
			hasPetrolStation = true
			laneStr = line[:len(line)-1] // Remove 'P' suffix
		}

		numLanes, err := strconv.Atoi(laneStr)
		if err != nil {
			return nil, fmt.Errorf("invalid lane count '%s': %w", line, err)
		}

		if numLanes < 1 {
			numLanes = 1
		}

		// If petrol station lane is present, add one extra lane (the petrol station lane)
		// The petrol station lane will be lane 0 (rightmost/starting lane)
		if hasPetrolStation {
			numLanes++ // Add the petrol station lane
		}

		segment := RoadSegment{
			NumLanes:            numLanes,
			StartY:              currentY,
			EndY:                currentY + segmentHeight,
			HasPetrolStationLane: hasPetrolStation,
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

// GetLaneCenterX returns the world X coordinate of the center of the given lane
// Accounts for petrol station lane on the left side
func (r *Road) GetLaneCenterX(lane int, worldY float64) float64 {
	segment := r.GetSegmentAtY(worldY)
	if segment == nil {
		return float64(lane)*r.LaneWidth + r.LaneWidth/2
	}
	
	// If segment has a petrol station lane:
	// - P lane (lane 0) is at X=-LaneWidth (to the left of normal lanes)
	// - Normal lane 0 (lane 1) is at X=0
	// - Normal lane 1 (lane 2) is at X=LaneWidth
	// - Normal lane 2 (lane 3) is at X=2*LaneWidth, etc.
	if segment.HasPetrolStationLane {
		if lane == 0 {
			// P lane is at X=-LaneWidth (left side)
			return -r.LaneWidth + r.LaneWidth/2
		}
		// Normal lanes are at their standard positions, but lane index is offset by 1
		// Lane 1 (first normal) is at X=0, Lane 2 (second normal) is at X=LaneWidth, etc.
		normalLaneIndex := lane - 1 // Convert to normal lane index (0, 1, 2, ...)
		return float64(normalLaneIndex)*r.LaneWidth + r.LaneWidth/2
	}
	
	// Standard lane positioning (no P lane)
	// Lane 0 at X=0, Lane 1 at X=LaneWidth, etc.
	return float64(lane)*r.LaneWidth + r.LaneWidth/2
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
		// Normal lanes always at fixed positions: X=0, X=LaneWidth, X=2*LaneWidth, etc.
		// If segment has P lane, it's added on the left at X=-LaneWidth
		normalLaneCount := segment.NumLanes
		if segment.HasPetrolStationLane {
			normalLaneCount = segment.NumLanes - 1 // Exclude P lane from normal count
		}
		
		// Normal road extends from X=0 to X=normalLaneCount*LaneWidth
		roadWorldLeft := 0.0
		roadWorldRight := float64(normalLaneCount) * r.LaneWidth

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
		
		// If segment has P lane, draw it separately on the left at X=-LaneWidth
		if segment.HasPetrolStationLane {
			// Draw P lane at X=-LaneWidth (left side, opposite to normal lanes)
			pLaneWorldLeft := -r.LaneWidth
			pLaneWorldRight := 0.0
			pLaneLeftScreenX := screenCenterX - (pLaneWorldLeft - cameraX)
			pLaneRightScreenX := screenCenterX - (pLaneWorldRight - cameraX)
			
			// Ensure left < right
			if pLaneLeftScreenX > pLaneRightScreenX {
				pLaneLeftScreenX, pLaneRightScreenX = pLaneRightScreenX, pLaneLeftScreenX
			}
			
			// Clamp to screen
			if pLaneRightScreenX > 0 && pLaneLeftScreenX < float64(width) {
				pLaneDrawLeftX := pLaneLeftScreenX
				if pLaneDrawLeftX < 0 {
					pLaneDrawLeftX = 0
				}
				pLaneDrawRightX := pLaneRightScreenX
				if pLaneDrawRightX > float64(width) {
					pLaneDrawRightX = float64(width)
				}
				pLaneDrawWidth := pLaneDrawRightX - pLaneDrawLeftX
				if pLaneDrawWidth > 0 && roadHeightPx > 0 {
					pLaneRect := ebiten.NewImage(int(pLaneDrawWidth), int(roadHeightPx))
					pLaneRect.Fill(roadColor)
					pLaneOp := &ebiten.DrawImageOptions{}
					pLaneOp.GeoM.Translate(pLaneDrawLeftX, drawStartY)
					screen.DrawImage(pLaneRect, pLaneOp)
				}
			}
		}
		
		// Draw normal road surface (always at X=0 and beyond, regardless of P lane)
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

		// Draw dividers between lanes
		// If segment has P lane: divider between P lane (0) and first normal lane (1) at X=0
		// Then dividers between normal lanes at X=LaneWidth, 2*LaneWidth, etc.
		// If no P lane: dividers at X=LaneWidth, 2*LaneWidth, etc.
		if segment.HasPetrolStationLane {
			// Draw divider between P lane (0) and first normal lane (1) at X=0
			laneDividerWorldX := 0.0
			dividerScreenX := screenCenterX - (laneDividerWorldX - cameraX)
			
			// Draw dashed line for P lane divider
			currentY := drawStartY
			for currentY < drawEndY {
				dashEndY := currentY + dividerDashLength
				if dashEndY > drawEndY {
					dashEndY = drawEndY
				}
				dashHeight := dashEndY - currentY
				if dashHeight <= 0 {
					break
				}
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
				currentY = dashEndY + dividerGapLength
			}
		}

		// Draw dividers between normal lanes (always at X=LaneWidth, 2*LaneWidth, etc.)
		// Normal lanes start at lane 1 if P lane exists, or lane 0 if no P lane
		normalLaneStart := 1
		if segment.HasPetrolStationLane {
			normalLaneStart = 2 // Skip P lane (0) and first normal lane divider (already drawn)
		} else {
			normalLaneStart = 1 // Start from lane 1 divider
		}
		
		for lane := normalLaneStart; lane < segment.NumLanes; lane++ {
			// Calculate divider position in world space
			// For normal lanes: divider between normal lane N and N+1 is at X = (N+1) * LaneWidth
			// But we need to account for P lane offset
			normalLaneIndex := lane
			if segment.HasPetrolStationLane {
				normalLaneIndex = lane - 1 // Convert to normal lane index (P lane is lane 0)
			}
			// Divider between normal lane N and N+1 is at X = (N+1) * LaneWidth
			laneDividerWorldX := float64(normalLaneIndex) * r.LaneWidth
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
				// Left edge of normal road (at X=0)
				leftEdgeRect := ebiten.NewImage(edgeWidthInt, edgeHeightInt)
				leftEdgeRect.Fill(edgeColor)
				leftEdgeOp := &ebiten.DrawImageOptions{}
				leftEdgeOp.GeoM.Translate(drawLeftX-edgeWidth/2, drawStartY)
				screen.DrawImage(leftEdgeRect, leftEdgeOp)
				
				// If segment has P lane, also draw left edge of P lane at X=-LaneWidth
				if segment.HasPetrolStationLane {
					pLaneLeftWorldX := -r.LaneWidth
					pLaneLeftScreenX := screenCenterX - (pLaneLeftWorldX - cameraX)
					if pLaneLeftScreenX >= -edgeWidth && pLaneLeftScreenX <= float64(width) {
						pLaneLeftEdgeRect := ebiten.NewImage(edgeWidthInt, edgeHeightInt)
						pLaneLeftEdgeRect.Fill(edgeColor)
						pLaneLeftEdgeOp := &ebiten.DrawImageOptions{}
						pLaneLeftEdgeOp.GeoM.Translate(pLaneLeftScreenX-edgeWidth/2, drawStartY)
						screen.DrawImage(pLaneLeftEdgeRect, pLaneLeftEdgeOp)
					}
				}

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
