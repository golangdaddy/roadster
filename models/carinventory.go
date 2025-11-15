package models

import (
	"github.com/golangdaddy/roadster/models/car"
)

// CarInventory holds the global collection of available cars
var CarInventory *Garage

// InitializeCarInventory creates the global car inventory with test cars
func InitializeCarInventory() {
	CarInventory = NewGarage(0) // Unlimited capacity
	
	// Light sports car - fast, good brakes, light weight
	sportsCar := car.NewCar("Ferrari", "Testarossa", 1985)
	sportsCar.Weight = 1200.0 // kg - lighter
	sportsCar.Length = 4.48
	sportsCar.Width = 1.98
	sportsCar.Height = 1.13
	sportsCar.Seats = 2
	sportsCar.TopSpeed = 290.0
	sportsCar.Acceleration = 5.2
	sportsCar.Handling = 0.9
	sportsCar.Stability = 0.85
	sportsCar.Engine.Horsepower = 390
	sportsCar.Engine.Torque = 361
	sportsCar.Engine.Displacement = 4.9
	sportsCar.Engine.FuelEfficiency = 18.0
	sportsCar.Brakes.StoppingPower = 1.2 // Excellent brakes
	sportsCar.Brakes.Condition = 1.0
	sportsCar.Brakes.Performance = 1.0
	sportsCar.Brakes.Type = "carbon-ceramic"
	sportsCar.Transmission = "manual"
	sportsCar.DriveType = "RWD"
	CarInventory.AddCar(sportsCar)
	
	// Medium sedan - average weight, good brakes
	sedan := car.NewCar("Toyota", "Camry", 2020)
	sedan.Weight = 1500.0 // kg - average
	sedan.Length = 4.89
	sedan.Width = 1.84
	sedan.Height = 1.44
	sedan.Seats = 5
	sedan.TopSpeed = 200.0
	sedan.Acceleration = 8.5
	sedan.Handling = 0.7
	sedan.Stability = 0.9
	sedan.Engine.Horsepower = 203
	sedan.Engine.Torque = 184
	sedan.Engine.Displacement = 2.5
	sedan.Engine.FuelEfficiency = 32.0
	sedan.Brakes.StoppingPower = 1.0 // Good brakes
	sedan.Brakes.Condition = 1.0
	sedan.Brakes.Performance = 1.0
	sedan.Brakes.Type = "disc"
	sedan.Transmission = "automatic"
	sedan.DriveType = "FWD"
	CarInventory.AddCar(sedan)
	
	// Heavy truck - heavy, slower brakes
	truck := car.NewCar("Ford", "F-150", 2022)
	truck.Weight = 2200.0 // kg - heavy
	truck.Length = 5.91
	truck.Width = 2.03
	truck.Height = 1.99
	truck.Seats = 5
	truck.TopSpeed = 180.0
	truck.Acceleration = 7.8
	truck.Handling = 0.6
	truck.Stability = 0.85
	truck.Engine.Horsepower = 400
	truck.Engine.Torque = 500
	truck.Engine.Displacement = 5.0
	truck.Engine.FuelEfficiency = 22.0
	truck.Brakes.StoppingPower = 0.9 // Decent brakes
	truck.Brakes.Condition = 1.0
	truck.Brakes.Performance = 1.0
	truck.Brakes.Type = "disc"
	truck.Transmission = "automatic"
	truck.DriveType = "AWD"
	CarInventory.AddCar(truck)
	
	// Lightweight race car - very light, excellent brakes
	raceCar := car.NewCar("McLaren", "F1", 1993)
	raceCar.Weight = 1000.0 // kg - very light
	raceCar.Length = 4.29
	raceCar.Width = 1.82
	raceCar.Height = 1.14
	raceCar.Seats = 2
	raceCar.TopSpeed = 386.0
	raceCar.Acceleration = 3.2
	raceCar.Handling = 0.95
	raceCar.Stability = 0.9
	raceCar.Engine.Horsepower = 627
	raceCar.Engine.Torque = 479
	raceCar.Engine.Displacement = 6.1
	raceCar.Engine.FuelEfficiency = 12.0
	raceCar.Brakes.StoppingPower = 1.5 // Racing brakes
	raceCar.Brakes.Condition = 1.0
	raceCar.Brakes.Performance = 1.0
	raceCar.Brakes.Type = "carbon-ceramic"
	raceCar.Transmission = "manual"
	raceCar.DriveType = "RWD"
	CarInventory.AddCar(raceCar)
	
	// Old car with worn brakes - heavy, poor brakes
	oldCar := car.NewCar("Chevrolet", "Impala", 1970)
	oldCar.Weight = 1800.0 // kg - heavy
	oldCar.Length = 5.49
	oldCar.Width = 2.01
	oldCar.Height = 1.40
	oldCar.Seats = 6
	oldCar.TopSpeed = 180.0
	oldCar.Acceleration = 10.5
	oldCar.Handling = 0.5
	oldCar.Stability = 0.7
	oldCar.Engine.Horsepower = 250
	oldCar.Engine.Torque = 360
	oldCar.Engine.Displacement = 5.7
	oldCar.Engine.FuelEfficiency = 15.0
	oldCar.Brakes.StoppingPower = 0.6 // Worn brakes
	oldCar.Brakes.Condition = 0.5 // Poor condition
	oldCar.Brakes.Performance = 0.7 // Reduced performance
	oldCar.Brakes.Type = "drum"
	oldCar.Transmission = "automatic"
	oldCar.DriveType = "RWD"
	CarInventory.AddCar(oldCar)
}

