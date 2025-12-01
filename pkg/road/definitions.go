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

	LaybyTypeServices = iota
	LaybyTypeExit
)

// this should be the new contents of files in assets/level/*.level
type LevelDefinition struct {
	Laybys   []*Layby            `json:"laybys"`
	Sections map[string]*Section `json:"sections"`
	// the layout is a list of section names in the order they should be placed in the level baed on their key in the map above.
	Layout []string `json:"layout"`
}

// allows sections to be defined that can be reused in the level definition.
type Section struct {
	Segments []string `json:"segments"`
}

// use layby to generate the length of a layby from the services it contains. each service should be at least 1 segment long.
type Layby struct {
	Type            int        `json:"type"`
	StartSegment    int        `json:"start_segment"`
	Services        []*Service `json:"services"`
	ExitDestination string     `json:"exit_destination"`
}

type Service struct {
	Type     int `json:"type"`
	Position int
}
