package lanecontroller

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

// LoadLaneControllersFromFile loads lane controllers from a level file
// Returns an array of lane controllers, one for each lane in each segment
func LoadLaneControllersFromFile(filename string, segmentHeight, laneWidth float64) ([]*LaneController, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open level file: %w", err)
	}
	defer file.Close()

	var laneControllers []*LaneController
	currentY := 0.0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Check for suffixes: '|' (layby), '\' (onramp), '/' (offramp)
		hasLayby := false
		tileType := "normal"
		laneStr := line

		if len(line) > 0 {
			lastChar := line[len(line)-1]
			if lastChar == '|' {
				hasLayby = true
				tileType = "layby"
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

		// Create lane controllers for this segment
		// Normal lanes are always at X=0, X=LaneWidth, X=2*LaneWidth, etc. (fixed positions)
		// If layby exists, it's added on the LEFT side (X=-LaneWidth)
		
		// Create layby lane controller first (if exists) - on the LEFT side
		// Only ONE lane controller should have the layby - this is the special annex lane
		if hasLayby {
			// Layby is at X=-LaneWidth (to the left of normal lanes at X=0)
			laybyWorldX := -laneWidth
			// Create layby controller with HasLayby flag set
			laybyController := NewLaneController(-1, laybyWorldX, currentY, currentY+segmentHeight, "layby")
			laybyController.HasLayby = true // This flag identifies it as the layby lane
			laneControllers = append(laneControllers, laybyController)
		}
		
		// Create normal lane controllers (always at fixed X positions)
		for i := 0; i < numLanes; i++ {
			worldX := float64(i) * laneWidth // Normal lanes at X=0, X=LaneWidth, X=2*LaneWidth, etc.
			// Normal lanes use the tileType (normal/onramp/offramp), but never layby
			spriteType := tileType
			if spriteType == "layby" {
				spriteType = "normal" // Normal lanes always use normal sprite, never layby
			}
			
			// Normal lanes are always indexed 0, 1, 2, etc. (regardless of layby)
			laneIndex := i
			
			controller := NewLaneController(laneIndex, worldX, currentY, currentY+segmentHeight, spriteType)
			laneControllers = append(laneControllers, controller)
		}

		currentY += segmentHeight
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading level file: %w", err)
	}

	return laneControllers, nil
}

