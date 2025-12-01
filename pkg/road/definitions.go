package road

const (
	ServiceTypePetrol = iota
	ServiceTypeFood
	ServiceTypeRestroom
	ServiceTypeShop
	ServiceTypeHotel
	ServiceTypeMotel
	ServiceTypeCampground
	ServiceTypeRVPark
	ServiceTypeCamping
)

type LevelDefinition struct {
	Laybys []*Layby
	// keep the lane definitions eg. "AAA" (without the X as laybys handles this)
	Segments []string
}

// use layby to generate the length of a layby from the services it contains. each service should be at least 1 segment long.
type Layby struct {
	StartSegment int
	Services     []*Service
}

type Service struct {
	Type     int
	Position int
}
