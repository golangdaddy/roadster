package vehicle

type Vehicle interface {
	Wheels() int
	TopSpeed() int
	BrakingEfficiency() float64
}
