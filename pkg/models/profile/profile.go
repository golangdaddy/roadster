package profile

import (
	"time"

	"github.com/golangdaddy/roadster/pkg/models/car"
)

// PlayerProfile represents a user's save game and identity
type PlayerProfile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	AvatarPath   string    `json:"avatar_path"`   // Path to full body sprite
	HeadshotPath string    `json:"headshot_path"` // Path to profile image
	Created      time.Time `json:"created"`
	LastPlayed   time.Time `json:"last_played"`
	
	// Game Progress
	Level             int     `json:"level"`
	TotalCarsPassed   int     `json:"total_cars_passed"`
	DistanceTravelled float64 `json:"distance_travelled"`
	
	// Current State
	CurrentCar *car.Car `json:"current_car"`
	Money      float64  `json:"money"`
	
	// Player Stats
	FoodCapacity float64 `json:"food_capacity"` // 0-100 scale
	FoodLevel    float64 `json:"food_level"`    // 0-100 scale
}

// NewProfile creates a new player profile
func NewProfile(name, avatarPath, headshotPath string) *PlayerProfile {
	return &PlayerProfile{
		ID:           name + "_" + time.Now().Format("20060102150405"),
		Name:         name,
		AvatarPath:   avatarPath,
		HeadshotPath: headshotPath,
		Created:      time.Now(),
		LastPlayed:   time.Now(),
		Level:        1,
		Money:        1000.0, // Starting money
		FoodCapacity: 100.0,
		FoodLevel:    100.0, // Start full
	}
}

