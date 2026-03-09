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
	BackupDone     map[string]bool   // checkpoint ID -> backup already triggered
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
		BackupDone:     make(map[string]bool),
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

// TickInput holds all memory readings for a single tick cycle.
type TickInput struct {
	Flags      map[uint32]bool   // event flag ID → set
	MemValues  map[string]uint32 // checkpoint ID → current memory value (for mem_check checkpoints)
	IGT        int64
	DeathCount uint32
}

// BackupEvent is emitted when a backup flag is newly set (e.g. boss encountered).
type BackupEvent struct {
	Checkpoint Checkpoint
}

// TickResult holds the outputs of a single ProcessTick call.
type TickResult struct {
	Checkpoints []CheckpointEvent
	Backups     []BackupEvent
}

// ProcessTick checks which checkpoint conditions are newly met and records events.
func (rs *RunState) ProcessTick(input TickInput) TickResult {
	if rs.Status != RunInProgress {
		return TickResult{}
	}

	var result TickResult
	segmentDeaths := input.DeathCount - rs.LastDeathCount

	for _, cp := range rs.Route.Checkpoints {
		// Check backup flag (boss encounter) independently from checkpoint completion
		if cp.BackupFlagID != 0 && !rs.BackupDone[cp.ID] {
			if input.Flags[cp.BackupFlagID] {
				rs.BackupDone[cp.ID] = true
				result.Backups = append(result.Backups, BackupEvent{Checkpoint: cp})
			}
		}

		if rs.CompletedFlags[cp.ID] {
			continue
		}

		if !rs.checkCondition(cp, input) {
			continue
		}

		// Checkpoint newly completed
		rs.CompletedFlags[cp.ID] = true

		// Compute split duration: time since last completed checkpoint
		var splitDuration int64
		prevIGT := rs.lastCompletedIGT()
		splitDuration = input.IGT - prevIGT

		rs.SplitTimes[cp.ID] = input.IGT
		rs.SplitDeaths[cp.ID] = segmentDeaths

		result.Checkpoints = append(result.Checkpoints, CheckpointEvent{
			Checkpoint:    cp,
			IGT:           input.IGT,
			SplitDuration: splitDuration,
			Deaths:        segmentDeaths,
		})

		// Reset segment death tracking after recording
		segmentDeaths = 0
	}

	rs.LastDeathCount = input.DeathCount
	rs.LastIGT = input.IGT

	if len(result.Checkpoints) > 0 && rs.IsComplete() {
		rs.Status = RunCompleted
	}

	return result
}

// checkCondition returns true if the checkpoint's condition is met.
func (rs *RunState) checkCondition(cp Checkpoint, input TickInput) bool {
	// Flag-based check
	if cp.EventFlagID != 0 {
		return input.Flags[cp.EventFlagID]
	}

	// Memory value check
	if cp.MemCheck != nil {
		val, ok := input.MemValues[cp.ID]
		if !ok {
			return false
		}
		switch cp.MemCheck.Comparison {
		case "gte":
			return val >= cp.MemCheck.Value
		case "gt":
			return val > cp.MemCheck.Value
		case "eq":
			return val == cp.MemCheck.Value
		}
	}

	return false
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
