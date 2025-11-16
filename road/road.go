package road

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// RoadSegment represents a single segment of road with a specific number of lanes
type RoadSegment struct {
	NumLanes             int     // Number of lanes in this segment (includes | lane if present)
	StartY               float64 // World Y position where this segment starts
	EndY                 float64 // World Y position where this segment ends
	HasPetrolStationLane bool    // Whether this segment has a | lane (40mph) on the left
	HasDiagonalBefore    bool    // Whether this segment has a \ symbol - merging in
	HasDiagonalAfter     bool    // Whether this segment has a / symbol - merging out
	TileType             string  // Which sprite to use: "normal", "onramp", "offramp"
}

// Road represents a highway made of segments loaded from a level file
type Road struct {
	Segments      []RoadSegment // All road segments
	LaneWidth     float64       // Width of each lane in pixels
	SegmentHeight float64       // Height of each segment (window height)

	// Pre-rendered segment tiles
	normalRoadTile *ebiten.Image // Normal road segment tile
	onRampTile     *ebiten.Image // On-ramp diagonal segment tile (\)
	offRampTile    *ebiten.Image // Off-ramp diagonal segment tile (/)
	laybyTile      *ebiten.Image // Layby (| lane) sprite tile
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

		// Check for suffixes: '|' (special lane), '\' (diagonal before), '/' (diagonal after)
		hasPetrolStation := false
		tileType := "normal"
		laneStr := line

		if len(line) > 0 {
			lastChar := line[len(line)-1]
			if lastChar == '|' {
				hasPetrolStation = true
				laneStr = line[:len(line)-1] // Remove '|' suffix
			} else if lastChar == '/' {
				tileType = "offramp"
				laneStr = line[:len(line)-1] // Remove '/' suffix
			} else if lastChar == '\\' {
				tileType = "onramp"
				laneStr = line[:len(line)-1] // Remove '\' suffix
			}
		}

		numLanes, err := strconv.Atoi(laneStr)
		if err != nil {
			return nil, fmt.Errorf("invalid lane count '%s': %w", line, err)
		}

		if numLanes < 1 {
			numLanes = 1
		}

		// If | lane is present, add one extra lane (the | lane)
		// The | lane will be lane 0 (leftmost lane)
		if hasPetrolStation {
			numLanes++ // Add the | lane
		}

		segment := RoadSegment{
			NumLanes:             numLanes,
			StartY:               currentY,
			EndY:                 currentY + segmentHeight,
			HasPetrolStationLane: hasPetrolStation,
			HasDiagonalBefore:    tileType == "onramp",
			HasDiagonalAfter:     tileType == "offramp",
			TileType:             tileType,
		}
		segments = append(segments, segment)

		currentY += segmentHeight
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading level file: %w", err)
	}

	road := &Road{
		Segments:      segments,
		LaneWidth:     laneWidth,
		SegmentHeight: segmentHeight,
	}

	// Load road segment tiles from sprite files
	// If loading fails, tiles will be nil and won't be drawn
	_ = road.loadRoadTiles()

	return road, nil
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
// Accounts for | lane on the left side
func (r *Road) GetLaneCenterX(lane int, worldY float64) float64 {
	segment := r.GetSegmentAtY(worldY)
	if segment == nil {
		return float64(lane)*r.LaneWidth + r.LaneWidth/2
	}

	// If segment has a | lane:
	// - | lane (lane 0) is at X=-LaneWidth (to the left of normal lanes)
	// - Normal lane 0 (lane 1) is at X=0
	// - Normal lane 1 (lane 2) is at X=LaneWidth
	// - Normal lane 2 (lane 3) is at X=2*LaneWidth, etc.
	if segment.HasPetrolStationLane {
		if lane == 0 {
			// | lane is at X=-LaneWidth (left side)
			return -r.LaneWidth + r.LaneWidth/2
		}
		// Normal lanes are at their standard positions, but lane index is offset by 1
		// Lane 1 (first normal) is at X=0, Lane 2 (second normal) is at X=LaneWidth, etc.
		normalLaneIndex := lane - 1 // Convert to normal lane index (0, 1, 2, ...)
		return float64(normalLaneIndex)*r.LaneWidth + r.LaneWidth/2
	}

	// Standard lane positioning (no | lane)
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
		// If segment has | lane, it's added on the left at X=-LaneWidth
		normalLaneCount := segment.NumLanes
		if segment.HasPetrolStationLane {
			normalLaneCount = segment.NumLanes - 1 // Exclude | lane from normal count
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

		// Get sprite tile based on pre-assigned TileType (no logic here, just lookup)
		var roadTile *ebiten.Image
		switch segment.TileType {
		case "onramp":
			roadTile = r.onRampTile
		case "offramp":
			roadTile = r.offRampTile
		default:
			roadTile = r.normalRoadTile
		}

		// Draw main road sprite tile - tile horizontally to fill road width
		if roadTile != nil {
			tileWidth := float64(roadTile.Bounds().Dx())

			// Tile horizontally to fill the road width
			currentX := drawLeftX
			for currentX < drawRightX {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(currentX, drawStartY)
				screen.DrawImage(roadTile, op)
				currentX += tileWidth
			}
		}

		// Draw layby (| lane) - tile horizontally to fill layby width
		if segment.HasPetrolStationLane && r.laybyTile != nil {
			// Layby is at X=-LaneWidth (left of normal lanes at X=0)
			laybyWorldLeft := -r.LaneWidth
			laybyWorldRight := 0.0
			laybyLeftScreenX := screenCenterX - (laybyWorldLeft - cameraX)
			laybyRightScreenX := screenCenterX - (laybyWorldRight - cameraX)

			// Ensure left < right
			if laybyLeftScreenX > laybyRightScreenX {
				laybyLeftScreenX, laybyRightScreenX = laybyRightScreenX, laybyLeftScreenX
			}

			// Clamp to screen
			if laybyRightScreenX > 0 && laybyLeftScreenX < float64(width) {
				laybyDrawLeftX := laybyLeftScreenX
				if laybyDrawLeftX < 0 {
					laybyDrawLeftX = 0
				}
				laybyDrawRightX := laybyRightScreenX
				if laybyDrawRightX > float64(width) {
					laybyDrawRightX = float64(width)
				}

				// Tile horizontally to fill layby width
				tileWidth := float64(r.laybyTile.Bounds().Dx())
				currentX := laybyDrawLeftX
				for currentX < laybyDrawRightX {
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Translate(currentX, drawStartY)
					screen.DrawImage(r.laybyTile, op)
					currentX += tileWidth
				}
			}
		}

	}
}

// loadRoadTiles loads road segment tiles from sprite files
func (r *Road) loadRoadTiles() error {
	// Try to load normal road tile
	normalRoadImg, _, err := ebitenutil.NewImageFromFile("assets/road/normal.png")
	if err != nil {
		fmt.Printf("Warning: Could not load normal road sprite: %v\n", err)
		// Don't return error, just continue without it
	} else {
		r.normalRoadTile = normalRoadImg
		fmt.Printf("Loaded normal road sprite: %dx%d\n", normalRoadImg.Bounds().Dx(), normalRoadImg.Bounds().Dy())
	}

	// Try to load on-ramp tile
	onRampImg, _, err := ebitenutil.NewImageFromFile("assets/road/onramp.png")
	if err != nil {
		fmt.Printf("Warning: Could not load on-ramp sprite: %v\n", err)
		// Don't return error, just continue without it
	} else {
		r.onRampTile = onRampImg
		fmt.Printf("Loaded on-ramp sprite: %dx%d\n", onRampImg.Bounds().Dx(), onRampImg.Bounds().Dy())
	}

	// Try to load off-ramp tile
	offRampImg, _, err := ebitenutil.NewImageFromFile("assets/road/offramp.png")
	if err != nil {
		fmt.Printf("Warning: Could not load off-ramp sprite: %v\n", err)
		// Don't return error, just continue without it
	} else {
		r.offRampTile = offRampImg
		fmt.Printf("Loaded off-ramp sprite: %dx%d\n", offRampImg.Bounds().Dx(), offRampImg.Bounds().Dy())
	}

	// Try to load layby tile
	laybyImg, _, err := ebitenutil.NewImageFromFile("assets/road/layby.png")
	if err != nil {
		fmt.Printf("Warning: Could not load layby sprite: %v\n", err)
		// Don't return error, just continue without it
	} else {
		r.laybyTile = laybyImg
		fmt.Printf("Loaded layby sprite: %dx%d\n", laybyImg.Bounds().Dx(), laybyImg.Bounds().Dy())
	}

	return nil
}
