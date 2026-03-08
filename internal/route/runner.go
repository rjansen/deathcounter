package route

import (
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/backup"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// Runner orchestrates a route run, connecting the state machine to memory reading,
// persistence, and save backups.
type Runner struct {
	state   *RunState
	route   *Route
	tracker *stats.Tracker
	backup  *backup.Manager
	runID   int64
}

// NewRunner creates a new route runner.
func NewRunner(route *Route, tracker *stats.Tracker, backupMgr *backup.Manager) *Runner {
	return &Runner{
		route:   route,
		tracker: tracker,
		backup:  backupMgr,
		state:   NewRunState(route),
	}
}

// Start begins a new route run, recording it in the database.
func (r *Runner) Start() error {
	runID, err := r.tracker.StartRouteRun(r.route.ID, r.route.Game)
	if err != nil {
		return fmt.Errorf("failed to start route run: %w", err)
	}
	r.runID = runID
	r.state.Start()
	return nil
}

// Abandon marks the current run as abandoned.
func (r *Runner) Abandon() error {
	r.state.Abandon()
	return r.tracker.EndRouteRun(r.runID, string(RunAbandoned), r.state.LastDeathCount, r.state.LastIGT)
}

// IsActive returns whether a run is currently in progress.
func (r *Runner) IsActive() bool {
	return r.state.Status == RunInProgress
}

// CompletionPercent returns the completion percentage of the current run.
func (r *Runner) CompletionPercent() float64 {
	return r.state.CompletionPercent()
}

// CurrentCheckpoint returns the next uncompleted required checkpoint.
func (r *Runner) CurrentCheckpoint() *Checkpoint {
	return r.state.CurrentCheckpoint()
}

// GetRoute returns the route being run.
func (r *Runner) GetRoute() *Route {
	return r.route
}

// CompletedCount returns the number of completed checkpoints.
func (r *Runner) CompletedCount() int {
	count := 0
	for _, cp := range r.route.Checkpoints {
		if r.state.CompletedFlags[cp.ID] {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of checkpoints.
func (r *Runner) TotalCount() int {
	return len(r.route.Checkpoints)
}

// SplitDeaths returns the deaths for the current segment.
func (r *Runner) SplitDeaths() uint32 {
	cp := r.CurrentCheckpoint()
	if cp == nil {
		return 0
	}
	return r.state.SplitDeaths[cp.ID]
}

// Tick is called every poll cycle. It reads event flags and IGT from the reader,
// processes the state machine, records splits, and triggers backups.
func (r *Runner) Tick(reader *memreader.GameReader, deathCount uint32) ([]CheckpointEvent, error) {
	if !r.IsActive() {
		return nil, nil
	}

	// Build tick input by reading all unfinished checkpoint conditions
	input := TickInput{
		Flags:      make(map[uint32]bool),
		MemValues:  make(map[string]uint32),
		DeathCount: deathCount,
	}

	for _, cp := range r.route.Checkpoints {
		if r.state.CompletedFlags[cp.ID] {
			continue
		}

		// Flag-based checkpoint
		if cp.EventFlagID != 0 {
			flagSet, err := reader.ReadEventFlag(cp.EventFlagID)
			if err != nil {
				return nil, fmt.Errorf("failed to read event flag %d: %w", cp.EventFlagID, err)
			}
			input.Flags[cp.EventFlagID] = flagSet
		}

		// Memory value checkpoint
		if cp.MemCheck != nil {
			size := cp.MemCheck.Size
			if size == 0 {
				size = 4
			}
			val, err := reader.ReadMemoryValue(cp.MemCheck.Path, cp.MemCheck.Offset, size)
			if err != nil {
				return nil, fmt.Errorf("failed to read memory value for %s: %w", cp.ID, err)
			}
			input.MemValues[cp.ID] = val
		}
	}

	// Read IGT
	igt, err := reader.ReadIGT()
	if err != nil {
		return nil, fmt.Errorf("failed to read IGT: %w", err)
	}
	input.IGT = igt

	// Process tick through state machine
	events := r.state.ProcessTick(input)

	// Record each completed checkpoint
	for _, evt := range events {
		if err := r.tracker.RecordSplit(r.runID, evt.Checkpoint.ID, evt.Checkpoint.Name,
			evt.IGT, evt.SplitDuration, evt.Deaths); err != nil {
			log.Printf("Failed to record split: %v", err)
		}

		// Update PB if better
		if err := r.tracker.UpdatePersonalBest(r.route.ID, evt.Checkpoint.ID,
			evt.IGT, evt.SplitDuration); err != nil {
			log.Printf("Failed to update PB: %v", err)
		}

		// Trigger save backup
		if r.backup != nil {
			game := r.findGameConfig()
			if game != nil && game.SaveFilePattern != "" {
				savePath, err := r.backup.ResolveSavePath(game.SaveFilePattern)
				if err == nil {
					label := fmt.Sprintf("%s_%s", r.route.ID, evt.Checkpoint.ID)
					if _, err := r.backup.Backup(savePath, label); err != nil {
						log.Printf("Failed to backup save: %v", err)
					}
				}
			}
		}
	}

	// Check if run is complete
	if r.state.Status == RunCompleted {
		if err := r.tracker.EndRouteRun(r.runID, string(RunCompleted),
			deathCount, igt); err != nil {
			log.Printf("Failed to end route run: %v", err)
		}
	}

	return events, nil
}

func (r *Runner) findGameConfig() *memreader.GameConfig {
	for _, g := range memreader.GetSupportedGameConfigs() {
		if g.Name == r.route.Game {
			return &g
		}
	}
	return nil
}
