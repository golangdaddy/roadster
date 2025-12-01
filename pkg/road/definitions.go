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

// this should be the new contents of files in assets/level/*.level
type LevelDefinition struct {
	Laybys []*Layby `json:"laybys"`
	// keep the lane definitions eg. "AAA" (without the X as laybys handles this)
	Segments []string `json:"segments"`
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
