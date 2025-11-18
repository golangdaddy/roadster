package models

import (
	"github.com/golangdaddy/roadster/pkg/models/car"
)

// CarInventory manages the collection of available cars
var CarInventory = &carInventory{
	cars: []*car.Car{
		car.NewCar("Toyota", "Corolla", 2020, 1400),
		car.NewCar("Honda", "Civic", 2021, 1350),
		car.NewCar("Ford", "Mustang", 2019, 1600),
		car.NewCar("BMW", "3 Series", 2022, 1500),
		car.NewCar("Audi", "A4", 2021, 1550),
	},
}

type carInventory struct {
	cars []*car.Car
}

// GetAllCars returns all available cars
func (ci *carInventory) GetAllCars() []*car.Car {
	return ci.cars
}
