package vehicle

type Car struct {
	wheels int
}

func (car *Car) Wheels() int {
	return car.wheels
}

// TopSpeed is in MPH
func (car *Car) TopSpeed() int {
	return car.wheels
}

func (car *Car) BrakingEfficiency() float64 {
	return 0.8 // Placeholder value
}
