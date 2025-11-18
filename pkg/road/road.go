package road

import "github.com/golangdaddy/roadster/pkg/vehicle"

type RoadController struct {
	currentSegment int
	traffic        []*LaneController
}

type LaneController struct {
	index    int
	vehicles []vehicle.Vehicle
}

func NewLaneController(laneIndex int) *LaneController {
	return &LaneController{
		index:    laneIndex,
		vehicles: make([]vehicle.Vehicle, 0),
	}
}

func NewRoadController() *RoadController {
	return &RoadController{
		currentSegment: 0,
		traffic:        make([]*LaneController, 0),
	}
}

func (rc *RoadController) AddLaneController(laneController *LaneController) {
	rc.traffic = append(rc.traffic, laneController)
}
