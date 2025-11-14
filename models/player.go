package models

// PlayerStats represents the player's statistics and progression
type PlayerStats struct {
	// Basic Info
	Name     string // Player's name
	Level    int    // Player level
	XP       int    // Current experience points
	XPToNext int    // Experience needed for next level

	// Currency
	Money float64 // Available currency

	// Career Stats
	RacesWon      int     // Number of races won
	RacesLost     int     // Number of races lost
	RacesCompleted int    // Total races completed
	WinRate       float64 // Win rate percentage

	// Driving Stats
	TotalDistance    float64 // Total distance driven in km
	TotalTimeDriven  float64 // Total time spent driving in hours
	TopSpeedReached  float64 // Highest speed achieved in km/h
	PerfectLaps      int     // Number of perfect laps completed
	Crashes          int     // Number of crashes
	NearMisses       int     // Number of near misses

	// Skill Ratings (0.0 to 1.0)
	DrivingSkill    float64 // Overall driving ability
	CorneringSkill  float64 // Ability to take corners
	BrakingSkill    float64 // Braking technique
	AccelerationSkill float64 // Acceleration control
	RacingLineSkill  float64 // Understanding of racing lines

	// Achievements
	Achievements []string // List of achievement names unlocked
}

// Player represents the game player
type Player struct {
	Stats PlayerStats
}

// NewPlayer creates a new player with default starting stats
func NewPlayer(name string) *Player {
	return &Player{
		Stats: PlayerStats{
			Name:            name,
			Level:           1,
			XP:              0,
			XPToNext:        100,
			Money:           10000.0, // Starting money
			RacesWon:        0,
			RacesLost:       0,
			RacesCompleted:  0,
			WinRate:         0.0,
			TotalDistance:   0.0,
			TotalTimeDriven: 0.0,
			TopSpeedReached: 0.0,
			PerfectLaps:     0,
			Crashes:         0,
			NearMisses:      0,
			DrivingSkill:    0.5, // Starting at 50%
			CorneringSkill:  0.5,
			BrakingSkill:    0.5,
			AccelerationSkill: 0.5,
			RacingLineSkill:  0.5,
			Achievements:    make([]string, 0),
		},
	}
}

// AddXP adds experience points and levels up if needed
func (p *Player) AddXP(amount int) {
	p.Stats.XP += amount
	for p.Stats.XP >= p.Stats.XPToNext {
		p.LevelUp()
	}
}

// LevelUp increases the player's level
func (p *Player) LevelUp() {
	p.Stats.XP -= p.Stats.XPToNext
	p.Stats.Level++
	p.Stats.XPToNext = int(float64(p.Stats.XPToNext) * 1.5) // Exponential growth
}

// AddMoney adds currency to the player
func (p *Player) AddMoney(amount float64) {
	p.Stats.Money += amount
}

// SpendMoney attempts to spend currency, returns true if successful
func (p *Player) SpendMoney(amount float64) bool {
	if p.Stats.Money >= amount {
		p.Stats.Money -= amount
		return true
	}
	return false
}

// RecordRaceWin records a race win
func (p *Player) RecordRaceWin() {
	p.Stats.RacesWon++
	p.Stats.RacesCompleted++
	p.UpdateWinRate()
}

// RecordRaceLoss records a race loss
func (p *Player) RecordRaceLoss() {
	p.Stats.RacesLost++
	p.Stats.RacesCompleted++
	p.UpdateWinRate()
}

// UpdateWinRate recalculates the win rate
func (p *Player) UpdateWinRate() {
	if p.Stats.RacesCompleted > 0 {
		p.Stats.WinRate = float64(p.Stats.RacesWon) / float64(p.Stats.RacesCompleted) * 100.0
	}
}

// UpdateTopSpeed updates the top speed if a new record is set
func (p *Player) UpdateTopSpeed(speed float64) {
	if speed > p.Stats.TopSpeedReached {
		p.Stats.TopSpeedReached = speed
	}
}

// AddAchievement adds an achievement if not already unlocked
func (p *Player) AddAchievement(achievement string) {
	for _, a := range p.Stats.Achievements {
		if a == achievement {
			return // Already unlocked
		}
	}
	p.Stats.Achievements = append(p.Stats.Achievements, achievement)
}

// ImproveSkill increases a skill rating (capped at 1.0)
func (p *Player) ImproveSkill(skill *float64, amount float64) {
	*skill += amount
	if *skill > 1.0 {
		*skill = 1.0
	}
	if *skill < 0.0 {
		*skill = 0.0
	}
}

// GetOverallSkill returns the average of all skill ratings
func (p *Player) GetOverallSkill() float64 {
	return (p.Stats.DrivingSkill + p.Stats.CorneringSkill + p.Stats.BrakingSkill +
		p.Stats.AccelerationSkill + p.Stats.RacingLineSkill) / 5.0
}

