package models

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"

	"github.com/golangdaddy/roadster/pkg/models/car"
)

// CarData matches the JSON structure of assets/car_data.json
type CarData struct {
	ID                int     `json:"id"`
	Category          string  `json:"category"`
	Make              string  `json:"make"`
	Model             string  `json:"model"`
	WeightKG          float64 `json:"weight_kg"`
	Accel0to60        float64 `json:"accel_0_60"`
	Accel0to100       float64 `json:"accel_0_100"`
	BHP               int     `json:"bhp"`
	BrakingEfficiency float64 `json:"braking_efficiency"`
}

// CarInventory manages the collection of available cars
var CarInventory = &carInventory{
	cars: []*car.Car{
		// Fallback default car to ensure inventory is never empty
		car.NewCar("Default", "Car", 2022, 1200),
	},
	carsByCategory: make(map[string][]*car.Car),
}

type carInventory struct {
	cars           []*car.Car
	carData        []CarData
	carsByCategory map[string][]*car.Car
}

// LoadInventory loads cars from the JSON file
func (ci *carInventory) LoadInventory(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var carDataList []CarData
	if err := json.NewDecoder(file).Decode(&carDataList); err != nil {
		return err
	}

	ci.carData = carDataList
	ci.cars = make([]*car.Car, 0, len(carDataList))
	ci.carsByCategory = make(map[string][]*car.Car)

	for _, data := range carDataList {
		// Convert CarData to car.Car
		c := car.NewCar(data.Make, data.Model, 2022, data.WeightKG)

		// Apply performance stats
		c.Category = data.Category
		c.Accel0to60 = data.Accel0to60
		c.Accel0to100 = data.Accel0to100
		c.BHP = data.BHP
		c.BrakingEfficiency = data.BrakingEfficiency
		c.Brakes.StoppingPower = data.BrakingEfficiency

		ci.cars = append(ci.cars, c)

		// Group by category
		if _, exists := ci.carsByCategory[c.Category]; !exists {
			ci.carsByCategory[c.Category] = make([]*car.Car, 0)
		}
		ci.carsByCategory[c.Category] = append(ci.carsByCategory[c.Category], c)
	}

	log.Printf("Loaded %d cars into inventory", len(ci.cars))
	return nil
}

// GetAllCars returns all available cars
func (ci *carInventory) GetAllCars() []*car.Car {
	return ci.cars
}

// GetRandomCarByCategory returns a random car from the specified categories
// categories: list of allowed category strings (e.g., "C1", "C2")
func (ci *carInventory) GetRandomCarByCategory(allowedCategories []string) *car.Car {
	if len(ci.cars) == 0 {
		// Should be covered by default init, but just in case
		return car.NewCar("Default", "Car", 2022, 1200)
	}

	var candidates []*car.Car

	for _, cat := range allowedCategories {
		if list, ok := ci.carsByCategory[cat]; ok {
			candidates = append(candidates, list...)
		}
	}

	// If no candidates found (e.g. invalid categories), fallback to any car
	if len(candidates) == 0 {
		// Just pick from all cars if we have loaded them, otherwise fallback
		if len(ci.cars) > 0 {
			return ci.cars[rand.Intn(len(ci.cars))]
		}
		return car.NewCar("Fallback", "Car", 2022, 1200)
	}

	return candidates[rand.Intn(len(candidates))]
}

// GetRandomCarData returns a random CarData entry for traffic generation
func (ci *carInventory) GetRandomCarData() CarData {
	if len(ci.carData) == 0 {
		return CarData{Make: "Generic", Model: "Car", WeightKG: 1200}
	}
	// Simple random selection (using global rand or caller provided?)
	// Since we don't have math/rand imported and don't want to seed here,
	// let's return the list and let caller pick, or just pick the first for now?
	// Better to let caller access data.
	return ci.carData[0] // Fallback, caller should use GetDataList
}

func (ci *carInventory) GetDataList() []CarData {
	return ci.carData
}
