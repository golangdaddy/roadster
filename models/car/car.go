package car

// Part represents a car part with condition and performance attributes
type Part struct {
	Name        string  // e.g., "Engine", "Wheel", "Brake"
	Condition   float64 // 0.0 to 1.0, where 1.0 is perfect condition
	Performance float64 // Performance multiplier (0.0 to 1.0+)
}

// Wheel represents a single wheel on the car
type Wheel struct {
	Part
	Position string // "front-left", "front-right", "rear-left", "rear-right"
}

// Engine represents the car's engine
type Engine struct {
	Part
	Horsepower     int     // Engine power in HP
	Torque         int     // Torque in lb-ft
	Displacement   float64 // Engine displacement in liters
	FuelEfficiency float64 // Miles per gallon
}

// Brake represents the car's braking system
type Brake struct {
	Part
	StoppingPower float64 // Braking force multiplier
	Type          string  // "disc", "drum", "carbon-ceramic"
}

// Car represents a vehicle in the game
type Car struct {
	// Basic Information
	Make  string // e.g., "Toyota", "Ferrari", "Tesla"
	Model string // e.g., "Camry", "F40", "Model S"
	Year  int    // Manufacturing year

	// Physical Attributes
	Seats  int     // Number of seats
	Weight float64 // Weight in kg
	Length float64 // Length in meters
	Width  float64 // Width in meters
	Height float64 // Height in meters

	// Performance Attributes
	TopSpeed     float64 // Maximum speed in km/h
	Acceleration float64 // 0-100 km/h time in seconds
	Handling     float64 // Handling rating (0.0 to 1.0)
	Stability    float64 // Stability rating (0.0 to 1.0)

	// Parts
	Engine Engine
	Brakes Brake
	Wheels [4]Wheel // Four wheels

	// Additional Attributes
	Color        string  // Car color
	FuelLevel    float64 // Current fuel level (0.0 to 1.0)
	FuelCapacity float64 // Fuel tank capacity in liters
	Mileage      float64 // Total distance traveled in km
	Condition    float64 // Overall car condition (0.0 to 1.0)
	Price        float64 // Car price in currency
	Transmission string  // "manual", "automatic", "CVT"
	DriveType    string  // "FWD", "RWD", "AWD"
}

// NewCar creates a new car with default values
func NewCar(make, model string, year int) *Car {
	return &Car{
		Make:  make,
		Model: model,
		Year:  year,
		Seats: 4,
		Engine: Engine{
			Part: Part{
				Name:        "Engine",
				Condition:   1.0,
				Performance: 1.0,
			},
			Horsepower:     150,
			Torque:         200,
			Displacement:   2.0,
			FuelEfficiency: 30.0,
		},
		Brakes: Brake{
			Part: Part{
				Name:        "Brakes",
				Condition:   1.0,
				Performance: 1.0,
			},
			StoppingPower: 1.0,
			Type:          "disc",
		},
		Wheels: [4]Wheel{
			{Part: Part{Name: "Wheel", Condition: 1.0, Performance: 1.0}, Position: "front-left"},
			{Part: Part{Name: "Wheel", Condition: 1.0, Performance: 1.0}, Position: "front-right"},
			{Part: Part{Name: "Wheel", Condition: 1.0, Performance: 1.0}, Position: "rear-left"},
			{Part: Part{Name: "Wheel", Condition: 1.0, Performance: 1.0}, Position: "rear-right"},
		},
		Color:        "white",
		FuelLevel:    1.0,
		FuelCapacity: 50.0, // Default 50 liters
		Condition:    1.0,
		Transmission: "automatic",
		DriveType:    "FWD",
	}
}

// GetOverallPerformance calculates the overall performance rating based on parts
func (c *Car) GetOverallPerformance() float64 {
	enginePerf := c.Engine.Performance * c.Engine.Condition
	brakePerf := c.Brakes.Performance * c.Brakes.Condition

	var wheelPerf float64
	for i := range c.Wheels {
		wheelPerf += c.Wheels[i].Performance * c.Wheels[i].Condition
	}
	wheelPerf /= 4.0 // Average wheel performance

	// Weighted average of all parts
	return (enginePerf*0.4 + brakePerf*0.2 + wheelPerf*0.2 + c.Condition*0.2)
}

// UpdateCondition updates the overall car condition based on parts
func (c *Car) UpdateCondition() {
	engineCond := c.Engine.Condition
	brakeCond := c.Brakes.Condition

	var wheelCond float64
	for i := range c.Wheels {
		wheelCond += c.Wheels[i].Condition
	}
	wheelCond /= 4.0

	// Average condition of all parts
	c.Condition = (engineCond + brakeCond + wheelCond) / 3.0
}

// GetBrakeDeceleration calculates realistic brake deceleration based on car weight and braking efficiency
// Returns the deceleration coefficient (0.0 to 1.0) that should be applied per frame
// Physics: deceleration = (braking_efficiency × friction_coefficient × gravity) / (weight_factor)
// For game purposes, we simplify to: deceleration_coefficient = base_coefficient × (braking_efficiency / weight_factor)
func (c *Car) GetBrakeDeceleration(currentSpeed float64) float64 {
	// Base brake coefficient (realistic value for good brakes at 60 FPS)
	// This represents the maximum deceleration rate
	// Reduced by 3x for softer, more gradual braking, then halved again for 2x softer braking
	baseBrakeCoefficient := 0.02 // ~2% per frame at 60 FPS for a well-braked car (6x softer total)
	
	// Calculate braking efficiency
	// Combines brake condition, brake performance, and stopping power
	brakeEfficiency := c.Brakes.Condition * c.Brakes.Performance * c.Brakes.StoppingPower
	
	// Weight factor: heavier cars take longer to stop
	// Typical car weight: 1000-2000 kg
	// Normalize to a factor: lighter = faster braking, heavier = slower braking
	// Use inverse relationship: lighter cars brake better
	baseWeight := 1500.0 // Reference weight in kg (average car)
	weightFactor := baseWeight / c.Weight // Lighter cars have higher factor (brake better)
	if weightFactor > 1.5 {
		weightFactor = 1.5 // Cap at 1.5x for very light cars
	}
	if weightFactor < 0.5 {
		weightFactor = 0.5 // Cap at 0.5x for very heavy vehicles
	}
	
	// Calculate final brake coefficient
	// Formula: brake_coefficient = base × efficiency × weight_factor
	brakeCoefficient := baseBrakeCoefficient * brakeEfficiency * weightFactor
	
	// Ensure it's within reasonable bounds (0.01 to 0.04)
	// Adjusted for 6x softer braking globally (3x + 2x softer)
	if brakeCoefficient < 0.01 {
		brakeCoefficient = 0.01 // Minimum braking (very poor brakes)
	}
	if brakeCoefficient > 0.04 {
		brakeCoefficient = 0.04 // Maximum braking (racing brakes)
	}
	
	return brakeCoefficient
}
