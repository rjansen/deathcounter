package route

import "time"

// RunStatus represents the current state of a route run.
type RunStatus string

const (
	RunNotStarted RunStatus = "not_started"
	RunInProgress RunStatus = "in_progress"
	RunCompleted  RunStatus = "completed"
	RunAbandoned  RunStatus = "abandoned"
)

// RunState tracks the progress of a single route run.
type RunState struct {
	Route          *Route
	Status         RunStatus
	StartTime      time.Time
	CompletedFlags map[string]bool   // checkpoint ID -> done
	SplitTimes     map[string]int64  // checkpoint ID -> IGT ms
	SplitDeaths    map[string]uint32 // checkpoint ID -> deaths in segment
	LastDeathCount uint32
	LastIGT        int64
}

// CheckpointEvent is emitted when a checkpoint is completed.
type CheckpointEvent struct {
	Checkpoint    Checkpoint
	IGT           int64  // IGT at completion (ms)
	SplitDuration int64  // time for this segment (ms)
	Deaths        uint32 // deaths in this segment
}

// NewRunState creates a new run state for the given route.
func NewRunState(route *Route) *RunState {
	return &RunState{
		Route:          route,
		Status:         RunNotStarted,
		CompletedFlags: make(map[string]bool),
		SplitTimes:     make(map[string]int64),
		SplitDeaths:    make(map[string]uint32),
	}
}

// Start begins the run.
func (rs *RunState) Start() {
	rs.Status = RunInProgress
	rs.StartTime = time.Now()
}

// Abandon marks the run as abandoned.
func (rs *RunState) Abandon() {
	rs.Status = RunAbandoned
}

// ProcessTick checks which checkpoint flags are newly true and records events.
// flags maps event flag IDs to their current state (true = triggered).
func (rs *RunState) ProcessTick(flags map[uint32]bool, igt int64, deathCount uint32) []CheckpointEvent {
	if rs.Status != RunInProgress {
		return nil
	}

	var events []CheckpointEvent
	segmentDeaths := deathCount - rs.LastDeathCount

	for _, cp := range rs.Route.Checkpoints {
		if rs.CompletedFlags[cp.ID] {
			continue
		}

		if !flags[cp.EventFlagID] {
			continue
		}

		// Checkpoint newly completed
		rs.CompletedFlags[cp.ID] = true

		// Compute split duration: time since last completed checkpoint
		var splitDuration int64
		prevIGT := rs.lastCompletedIGT()
		splitDuration = igt - prevIGT

		rs.SplitTimes[cp.ID] = igt
		rs.SplitDeaths[cp.ID] = segmentDeaths

		events = append(events, CheckpointEvent{
			Checkpoint:    cp,
			IGT:           igt,
			SplitDuration: splitDuration,
			Deaths:        segmentDeaths,
		})

		// Reset segment death tracking after recording
		segmentDeaths = 0
	}

	rs.LastDeathCount = deathCount
	rs.LastIGT = igt

	if len(events) > 0 && rs.IsComplete() {
		rs.Status = RunCompleted
	}

	return events
}

// lastCompletedIGT returns the IGT of the most recently completed checkpoint,
// or 0 if none have been completed yet.
func (rs *RunState) lastCompletedIGT() int64 {
	var maxIGT int64
	for _, igt := range rs.SplitTimes {
		if igt > maxIGT {
			maxIGT = igt
		}
	}
	return maxIGT
}

// CompletionPercent returns the percentage of required checkpoints completed.
func (rs *RunState) CompletionPercent() float64 {
	required := 0
	completed := 0
	for _, cp := range rs.Route.Checkpoints {
		if cp.Optional {
			continue
		}
		required++
		if rs.CompletedFlags[cp.ID] {
			completed++
		}
	}
	if required == 0 {
		return 100.0
	}
	return float64(completed) / float64(required) * 100.0
}

// CurrentCheckpoint returns the next uncompleted required checkpoint, or nil.
func (rs *RunState) CurrentCheckpoint() *Checkpoint {
	for i := range rs.Route.Checkpoints {
		cp := &rs.Route.Checkpoints[i]
		if !rs.CompletedFlags[cp.ID] && !cp.Optional {
			return cp
		}
	}
	return nil
}

// IsComplete returns true when all required checkpoints are done.
func (rs *RunState) IsComplete() bool {
	for _, cp := range rs.Route.Checkpoints {
		if cp.Optional {
			continue
		}
		if !rs.CompletedFlags[cp.ID] {
			return false
		}
	}
	return true
}
