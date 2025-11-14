package models

import (
	"encoding/json"
	"os"
	"time"
)

// GameState represents the complete state of a game in progress
type GameState struct {
	// Metadata
	SaveName    string    // Name of the save file
	CreatedAt   time.Time // When the game was created
	LastPlayed  time.Time // When the game was last played
	PlayTime    float64   // Total play time in hours

	// Game Data
	Player *Player // Player data
	Garage *Garage // Player's garage

	// Game Progress
	CurrentLocation string  // Current location/area
	StoryProgress   float64 // Story progression (0.0 to 1.0)
	UnlockedAreas   []string // List of unlocked areas/locations
}

// NewGameState creates a new game state with a new player and garage
func NewGameState(saveName, playerName string) *GameState {
	return &GameState{
		SaveName:     saveName,
		CreatedAt:    time.Now(),
		LastPlayed:   time.Now(),
		PlayTime:     0.0,
		Player:       NewPlayer(playerName),
		Garage:       NewGarage(20), // Default garage capacity of 20
		CurrentLocation: "Home",
		StoryProgress:   0.0,
		UnlockedAreas:   []string{"Home"},
	}
}

// SaveToFile saves the game state to a JSON file
func (gs *GameState) SaveToFile(filename string) error {
	gs.LastPlayed = time.Now()
	
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, data, 0644)
}

// LoadFromFile loads a game state from a JSON file
func LoadFromFile(filename string) (*GameState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var gs GameState
	err = json.Unmarshal(data, &gs)
	if err != nil {
		return nil, err
	}
	
	return &gs, nil
}

// UpdatePlayTime adds time to the total play time
func (gs *GameState) UpdatePlayTime(duration time.Duration) {
	gs.PlayTime += duration.Hours()
}

// UnlockArea adds a new area to the unlocked areas list if not already present
func (gs *GameState) UnlockArea(area string) {
	for _, a := range gs.UnlockedAreas {
		if a == area {
			return // Already unlocked
		}
	}
	gs.UnlockedAreas = append(gs.UnlockedAreas, area)
}

// IsAreaUnlocked checks if an area is unlocked
func (gs *GameState) IsAreaUnlocked(area string) bool {
	for _, a := range gs.UnlockedAreas {
		if a == area {
			return true
		}
	}
	return false
}

