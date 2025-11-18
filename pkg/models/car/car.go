package car

// Brakes represents the braking system of a car
type Brakes struct {
	Type           string  `json:"type"`
	Condition      float64 `json:"condition"`      // 0.0 to 1.0
	Performance    float64 `json:"performance"`    // 0.0 to 1.0
	StoppingPower  float64 `json:"stopping_power"` // 0.0 to 1.0
}

// Car represents a car in the game
type Car struct {
	Make         string  `json:"make"`
	Model        string  `json:"model"`
	Year         int     `json:"year"`
	Weight       float64 `json:"weight"`       // in kg
	FuelCapacity float64 `json:"fuel_capacity"` // in liters
	FuelLevel    float64 `json:"fuel_level"`    // in liters
	Brakes       Brakes  `json:"brakes"`
}

// NewCar creates a new car with default values
func NewCar(make, model string, year int, weight float64) *Car {
	fuelCapacity := 50.0 // Default 50 liter tank
	return &Car{
		Make:         make,
		Model:        model,
		Year:         year,
		Weight:       weight,
		FuelCapacity: fuelCapacity,
		FuelLevel:    fuelCapacity, // Start with full tank
		Brakes: Brakes{
			Type:          "Standard",
			Condition:     0.8,
			Performance:   0.7,
			StoppingPower: 0.6,
		},
	}
}
