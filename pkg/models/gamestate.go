package models

import (
	"encoding/json"
	"os"
	"time"
)

// GameState represents the current state of a game session
type GameState struct {
	Name         string    `json:"name"`
	PlayerName   string    `json:"player_name"`
	CurrentLevel int       `json:"current_level"`
	Score        int       `json:"score"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewGameState creates a new game state
func NewGameState(name, playerName string) *GameState {
	now := time.Now()
	return &GameState{
		Name:         name,
		PlayerName:   playerName,
		CurrentLevel: 0,
		Score:        0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// SaveToFile saves the game state to a JSON file
func (gs *GameState) SaveToFile(filename string) error {
	gs.UpdatedAt = time.Now()

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
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, err
	}

	return &gs, nil
}
