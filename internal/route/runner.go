package route

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/backup"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// GameReader is the subset of memreader.GameReader used by the route runner.
// Defining it locally allows testing without Windows memory internals.
type GameReader interface {
	ReadEventFlag(flagID uint32) (bool, error)
	ReadMemoryValue(pathName string, extraOffset int64, size int) (uint32, error)
	ReadIGT() (int64, error)
	ReadInventoryItemQuantity(itemID uint32) (uint32, error)
}

// stateVarData tracks cumulative inventory pickup counts.
type stateVarData struct {
	ItemID       uint32
	LastQuantity uint32
	Accumulated  uint32
	Initialized  bool
	Dirty        bool
}

// Runner orchestrates a route run, connecting the state machine to memory reading,
// persistence, and save backups.
type Runner struct {
	state     *RunState
	route     *Route
	tracker   *stats.Tracker
	backup    *backup.Manager
	runID     int64
	stateVars map[string]*stateVarData // state_var name → tracking data
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
// initialDeathCount should be the current death count so the first segment
// only tracks deaths that occur after the run starts.
// saveID links the run to a character save slot (0 means no save tracking).
func (r *Runner) Start(initialDeathCount uint32, saveID int64) error {
	runID, err := r.tracker.StartRouteRun(r.route.ID, r.route.Game, saveID)
	if err != nil {
		return fmt.Errorf("failed to start route run: %w", err)
	}
	r.runID = runID
	r.state.Start()
	r.state.LastDeathCount = initialDeathCount
	r.initStateVars()
	return nil
}

// initStateVars registers all unique state_var names from the route checkpoints.
func (r *Runner) initStateVars() {
	r.stateVars = make(map[string]*stateVarData)
	for _, cp := range r.route.Checkpoints {
		if cp.InventoryCheck == nil || cp.InventoryCheck.StateVar == "" {
			continue
		}
		name := cp.InventoryCheck.StateVar
		if _, exists := r.stateVars[name]; !exists {
			r.stateVars[name] = &stateVarData{
				ItemID: cp.InventoryCheck.ItemID,
			}
		}
	}

	// Try to restore persisted state vars (for future run resumption)
	if len(r.stateVars) > 0 {
		rows, err := r.tracker.LoadStateVars(r.runID)
		if err != nil {
			log.Printf("[Route] Failed to load state vars: %v", err)
			return
		}
		for _, row := range rows {
			if sv, ok := r.stateVars[row.VarName]; ok {
				sv.LastQuantity = row.LastQuantity
				sv.Accumulated = row.Accumulated
				sv.Initialized = true
			}
		}
	}
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

// SegmentDeaths returns the deaths for the current segment toward the next checkpoint.
func (r *Runner) SegmentDeaths() uint32 {
	cp := r.CurrentCheckpoint()
	if cp == nil {
		return 0
	}
	return r.state.CheckpointDeaths[cp.ID]
}

// CatchUp scans all checkpoint flags and marks any that are already set as completed.
// Returns true when the scan completes, false if flag reading isn't ready yet (retry later).
func (r *Runner) CatchUp(reader GameReader) bool {
	if !r.IsActive() {
		return true
	}

	for _, cp := range r.route.Checkpoints {
		if r.state.CompletedFlags[cp.ID] {
			continue
		}

		if cp.EventFlagID != 0 {
			flagSet, err := reader.ReadEventFlag(cp.EventFlagID)
			if err != nil {
				// Not ready yet — caller should retry later
				return false
			}
			if flagSet {
				r.state.CompletedFlags[cp.ID] = true
				log.Printf("[Route] Already completed: %s", cp.Name)
				if err := r.tracker.RecordCheckpoint(r.runID, cp.ID, cp.Name, 0, 0, 0); err != nil {
					log.Printf("[Route] Failed to record caught-up checkpoint %s: %v", cp.ID, err)
				}
			}
		}

		if cp.InventoryCheck != nil && !r.state.CompletedFlags[cp.ID] {
			qty, err := reader.ReadInventoryItemQuantity(cp.InventoryCheck.ItemID)
			if err != nil {
				return false
			}
			checkQty := qty
			// Initialize state_var with current quantity as seed
			if cp.InventoryCheck.StateVar != "" {
				if sv, ok := r.stateVars[cp.InventoryCheck.StateVar]; ok && !sv.Initialized {
					sv.LastQuantity = qty
					sv.Accumulated = qty
					sv.Initialized = true
					sv.Dirty = true
				}
				if sv, ok := r.stateVars[cp.InventoryCheck.StateVar]; ok {
					checkQty = sv.Accumulated
				}
			}
			if compareValue(checkQty, cp.InventoryCheck.Comparison, cp.InventoryCheck.Value) {
				r.state.CompletedFlags[cp.ID] = true
				log.Printf("[Route] Already completed: %s", cp.Name)
				if err := r.tracker.RecordCheckpoint(r.runID, cp.ID, cp.Name, 0, 0, 0); err != nil {
					log.Printf("[Route] Failed to record caught-up checkpoint %s: %v", cp.ID, err)
				}
			}
		}

		// Also mark backup as done for already-completed checkpoints
		if cp.BackupFlagID != 0 && r.state.CompletedFlags[cp.ID] {
			r.state.BackupDone[cp.ID] = true
		}
	}

	// Persist dirty state vars after catchup
	r.persistDirtyStateVars()

	return true
}

// RestoreFromDB restores completed checkpoint state from the database
// instead of re-scanning game memory via CatchUp.
func (r *Runner) RestoreFromDB() error {
	ids, err := r.tracker.LoadCompletedCheckpoints(r.runID)
	if err != nil {
		return fmt.Errorf("failed to restore from DB: %w", err)
	}
	for _, id := range ids {
		r.state.CompletedFlags[id] = true
	}
	// Mark backup as done for completed checkpoints that have a backup flag
	for _, cp := range r.route.Checkpoints {
		if cp.BackupFlagID != 0 && r.state.CompletedFlags[cp.ID] {
			r.state.BackupDone[cp.ID] = true
		}
	}
	return nil
}

// Tick is called every poll cycle. It reads event flags and IGT from the reader,
// processes the state machine, records splits, and triggers backups.
func (r *Runner) Tick(reader GameReader, deathCount uint32) ([]CheckpointEvent, error) {
	if !r.IsActive() {
		return nil, nil
	}

	// Build tick input by reading all unfinished checkpoint conditions
	input := TickInput{
		Flags:           make(map[uint32]bool),
		MemValues:       make(map[string]uint32),
		InventoryValues: make(map[string]uint32),
		DeathCount:      deathCount,
	}
	stateVarUpdated := make(map[string]bool) // track which state vars already processed this tick

	for _, cp := range r.route.Checkpoints {
		// Read backup flag even if checkpoint is already completed
		if cp.BackupFlagID != 0 && !r.state.BackupDone[cp.ID] {
			if _, exists := input.Flags[cp.BackupFlagID]; !exists {
				flagSet, err := reader.ReadEventFlag(cp.BackupFlagID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("failed to read backup flag %d: %w", cp.BackupFlagID, err)
					}
					// ErrNullPointer: skip this backup flag for now
				} else {
					input.Flags[cp.BackupFlagID] = flagSet
				}
			}
		}

		if r.state.CompletedFlags[cp.ID] {
			continue
		}

		// Flag-based checkpoint
		if cp.EventFlagID != 0 {
			if _, exists := input.Flags[cp.EventFlagID]; !exists {
				flagSet, err := reader.ReadEventFlag(cp.EventFlagID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("failed to read event flag %d: %w", cp.EventFlagID, err)
					}
					// ErrNullPointer: skip this checkpoint for now
					continue
				}
				input.Flags[cp.EventFlagID] = flagSet
			}
		}

		// Memory value checkpoint
		if cp.MemCheck != nil {
			size := cp.MemCheck.Size
			if size == 0 {
				size = 4
			}
			val, err := reader.ReadMemoryValue(cp.MemCheck.Path, cp.MemCheck.Offset, size)
			if err != nil {
				if !errors.Is(err, memreader.ErrNullPointer) {
					return nil, fmt.Errorf("failed to read memory value for %s: %w", cp.ID, err)
				}
				// ErrNullPointer: skip this checkpoint for now
				continue
			}
			input.MemValues[cp.ID] = val
		}

		// Inventory item quantity checkpoint
		if cp.InventoryCheck != nil {
			if cp.InventoryCheck.StateVar != "" {
				svName := cp.InventoryCheck.StateVar
				sv := r.stateVars[svName]
				// Only read and accumulate once per state_var per tick
				if !stateVarUpdated[svName] {
					qty, err := reader.ReadInventoryItemQuantity(cp.InventoryCheck.ItemID)
					if err != nil {
						if !errors.Is(err, memreader.ErrNullPointer) {
							return nil, fmt.Errorf("failed to read inventory for %s: %w", cp.ID, err)
						}
						continue
					}
					if !sv.Initialized {
						sv.LastQuantity = qty
						sv.Accumulated = qty
						sv.Initialized = true
						sv.Dirty = true
					} else if qty > sv.LastQuantity {
						sv.Accumulated += qty - sv.LastQuantity
						sv.Dirty = true
					}
					sv.LastQuantity = qty
					stateVarUpdated[svName] = true
				}
				input.InventoryValues[cp.ID] = sv.Accumulated
			} else {
				qty, err := reader.ReadInventoryItemQuantity(cp.InventoryCheck.ItemID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("failed to read inventory for %s: %w", cp.ID, err)
					}
					continue
				}
				input.InventoryValues[cp.ID] = qty
			}
		}
	}

	// Persist dirty state vars
	r.persistDirtyStateVars()

	// Read IGT (fall back to last known value if transient failure)
	igt, err := reader.ReadIGT()
	if err != nil {
		if !errors.Is(err, memreader.ErrNullPointer) {
			return nil, fmt.Errorf("failed to read IGT: %w", err)
		}
		igt = r.state.LastIGT
	}
	input.IGT = igt

	// Process tick through state machine
	result := r.state.ProcessTick(input)

	// Trigger save backups for encounter events (before the fight)
	for _, bk := range result.Backups {
		log.Printf("[Route] Backup triggered: %s (encounter)", bk.Checkpoint.Name)
		r.triggerBackup(bk.Checkpoint.ID)
	}

	// Record each completed checkpoint
	for _, evt := range result.Checkpoints {
		log.Printf("[Route] Checkpoint completed: %s", evt.Checkpoint.Name)
		if err := r.tracker.RecordCheckpoint(r.runID, evt.Checkpoint.ID, evt.Checkpoint.Name,
			evt.IGT, evt.CheckpointDuration, evt.Deaths); err != nil {
			log.Printf("Failed to record checkpoint: %v", err)
		}

		// Update PB if better
		if err := r.tracker.UpdatePersonalBest(r.route.ID, evt.Checkpoint.ID,
			evt.IGT, evt.CheckpointDuration); err != nil {
			log.Printf("Failed to update PB: %v", err)
		}

		// Trigger save backup on kill if no encounter backup was configured
		if evt.Checkpoint.BackupFlagID == 0 {
			r.triggerBackup(evt.Checkpoint.ID)
		}
	}

	// Check if run is complete
	if r.state.Status == RunCompleted {
		if err := r.tracker.EndRouteRun(r.runID, string(RunCompleted),
			deathCount, igt); err != nil {
			log.Printf("Failed to end route run: %v", err)
		}
	}

	return result.Checkpoints, nil
}

// persistDirtyStateVars saves any state vars that changed since last persist.
func (r *Runner) persistDirtyStateVars() {
	for name, sv := range r.stateVars {
		if !sv.Dirty {
			continue
		}
		if err := r.tracker.SaveStateVar(r.runID, name, sv.ItemID, sv.LastQuantity, sv.Accumulated); err != nil {
			log.Printf("[Route] Failed to save state var %s: %v", name, err)
		}
		sv.Dirty = false
	}
}

func (r *Runner) triggerBackup(checkpointID string) {
	if r.backup == nil {
		return
	}
	game := r.findGameConfig()
	if game == nil || game.SaveFilePattern == "" {
		return
	}
	savePath, err := r.backup.ResolveSavePath(game.SaveFilePattern)
	if err != nil {
		log.Printf("Failed to resolve save path: %v", err)
		return
	}
	label := fmt.Sprintf("%s_%s", r.route.ID, checkpointID)
	if _, err := r.backup.Backup(savePath, label); err != nil {
		log.Printf("Failed to backup save: %v", err)
	}
}

func (r *Runner) findGameConfig() *memreader.GameConfig {
	for _, g := range memreader.GetSupportedGameConfigs() {
		if g.Name == r.route.Game {
			return &g
		}
	}
	return nil
}
