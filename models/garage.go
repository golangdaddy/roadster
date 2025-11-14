package models

import (
	"errors"
	"fmt"

	"github.com/golangdaddy/roadster/models/car"
)

// Garage represents a collection of cars that can be stored and retrieved
type Garage struct {
	Cars      []*car.Car // Slice of cars in the garage
	Capacity  int        // Maximum number of cars (0 = unlimited)
	ActiveCar int        // Index of the currently active/selected car (-1 if none)
}

// NewGarage creates a new garage with optional capacity limit
// If capacity is 0, the garage has unlimited capacity
func NewGarage(capacity int) *Garage {
	return &Garage{
		Cars:      make([]*car.Car, 0),
		Capacity:  capacity,
		ActiveCar: -1, // No active car initially
	}
}

// AddCar adds a car to the garage
// Returns an error if the garage is at capacity
func (g *Garage) AddCar(c *car.Car) error {
	if g.Capacity > 0 && len(g.Cars) >= g.Capacity {
		return errors.New("garage is at capacity")
	}
	if c == nil {
		return errors.New("cannot add nil car")
	}
	g.Cars = append(g.Cars, c)
	
	// If this is the first car, set it as active
	if len(g.Cars) == 1 {
		g.ActiveCar = 0
	}
	
	return nil
}

// RemoveCar removes a car from the garage by index
// Returns an error if the index is invalid
func (g *Garage) RemoveCar(index int) error {
	if index < 0 || index >= len(g.Cars) {
		return errors.New("invalid car index")
	}
	
	// Remove the car from the slice
	g.Cars = append(g.Cars[:index], g.Cars[index+1:]...)
	
	// Adjust active car index if necessary
	if g.ActiveCar >= len(g.Cars) {
		if len(g.Cars) > 0 {
			g.ActiveCar = len(g.Cars) - 1
		} else {
			g.ActiveCar = -1
		}
	} else if g.ActiveCar > index {
		g.ActiveCar--
	}
	
	return nil
}

// GetCar retrieves a car by index
// Returns nil if the index is invalid
func (g *Garage) GetCar(index int) *car.Car {
	if index < 0 || index >= len(g.Cars) {
		return nil
	}
	return g.Cars[index]
}

// GetActiveCar returns the currently active car
// Returns nil if no car is active
func (g *Garage) GetActiveCar() *car.Car {
	if g.ActiveCar < 0 || g.ActiveCar >= len(g.Cars) {
		return nil
	}
	return g.Cars[g.ActiveCar]
}

// SetActiveCar sets the active car by index
// Returns an error if the index is invalid
func (g *Garage) SetActiveCar(index int) error {
	if index < 0 || index >= len(g.Cars) {
		return errors.New("invalid car index")
	}
	g.ActiveCar = index
	return nil
}

// FindCarByMakeModel searches for a car by make and model
// Returns the first matching car and its index, or nil and -1 if not found
func (g *Garage) FindCarByMakeModel(make, model string) (*car.Car, int) {
	for i, c := range g.Cars {
		if c.Make == make && c.Model == model {
			return c, i
		}
	}
	return nil, -1
}

// GetAllCars returns all cars in the garage
func (g *Garage) GetAllCars() []*car.Car {
	return g.Cars
}

// GetCarCount returns the number of cars in the garage
func (g *Garage) GetCarCount() int {
	return len(g.Cars)
}

// IsFull returns true if the garage is at capacity
func (g *Garage) IsFull() bool {
	if g.Capacity == 0 {
		return false // Unlimited capacity
	}
	return len(g.Cars) >= g.Capacity
}

// GetRemainingSlots returns the number of remaining slots in the garage
// Returns -1 if capacity is unlimited
func (g *Garage) GetRemainingSlots() int {
	if g.Capacity == 0 {
		return -1 // Unlimited
	}
	remaining := g.Capacity - len(g.Cars)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Clear removes all cars from the garage
func (g *Garage) Clear() {
	g.Cars = make([]*car.Car, 0)
	g.ActiveCar = -1
}

// String returns a string representation of the garage
func (g *Garage) String() string {
	capacityStr := "unlimited"
	if g.Capacity > 0 {
		capacityStr = fmt.Sprintf("%d/%d", len(g.Cars), g.Capacity)
	} else {
		capacityStr = fmt.Sprintf("%d", len(g.Cars))
	}
	
	activeStr := "none"
	if g.ActiveCar >= 0 && g.ActiveCar < len(g.Cars) {
		activeCar := g.Cars[g.ActiveCar]
		activeStr = fmt.Sprintf("%s %s", activeCar.Make, activeCar.Model)
	}
	
	return fmt.Sprintf("Garage: %s cars, Active: %s", capacityStr, activeStr)
}

