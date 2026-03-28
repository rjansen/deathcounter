package route

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/backup"
	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
)

// GameReader is the subset of memreader.GameReader used by the route runner.
// Defining it locally allows testing without Windows memory internals.
type GameReader interface {
	ReadEventFlag(flagID uint32) (bool, error)
	ReadMemoryValue(pathName string, extraOffset int64, size int) (uint32, error)
	ReadIGT() (int64, error)
	ReadInventoryItemQuantity(itemID uint32) (uint32, error)
	ReadDeathCount() (uint32, error)
}

// stateVarData tracks cumulative inventory acquired and consumed counts.
type stateVarData struct {
	ItemID       uint32
	LastQuantity uint32
	Acquired     uint32
	Consumed     uint32
	Initialized  bool
	Dirty        bool
}

// Runner orchestrates a route run, connecting the state machine to memory reading,
// persistence, and save backups.
type Runner struct {
	state     *RunState
	route     *Route
	repo      *data.Repository
	backup    *backup.Manager
	runID     int64
	stateVars map[string]*stateVarData // state_var name → tracking data
}

// NewRunner creates a new route runner.
// If backupMgr is nil, a default backup manager is created.
func NewRunner(route *Route, repo *data.Repository, backupMgr *backup.Manager) *Runner {
	if backupMgr == nil {
		backupMgr = backup.NewManager("backups")
	}
	return &Runner{
		route:  route,
		repo:   repo,
		backup: backupMgr,
		state:  NewRunState(route),
	}
}

// LastDeathCount returns the last known death count from the run state.
func (r *Runner) LastDeathCount() uint32 {
	return r.state.LastDeathCount
}

// LastIGT returns the last known in-game time in milliseconds.
func (r *Runner) LastIGT() int64 {
	return r.state.LastIGT
}

// Start begins a new route run, recording it in the database.
// initialDeathCount should be the current death count so the first segment
// only tracks deaths that occur after the run starts.
// saveID links the run to a character save slot (0 means no save tracking).
func (r *Runner) Start(initialDeathCount uint32, saveID int64) error {
	run, err := r.repo.StartRouteRun(r.route.ID, r.route.Game, saveID)
	if err != nil {
		return fmt.Errorf("failed to start route run: %w", err)
	}
	r.runID = run.ID
	r.state.Start()
	r.state.LastDeathCount = initialDeathCount
	r.initStateVars()
	return nil
}

// Resume re-attaches to an existing in-progress run from the database.
// It restores completed checkpoints and state vars without creating a new DB record.
func (r *Runner) Resume(runID int64, initialDeathCount uint32) error {
	r.runID = runID
	r.state.Start()
	r.state.LastDeathCount = initialDeathCount
	r.initStateVars()
	if err := r.RestoreFromDB(); err != nil {
		return fmt.Errorf("failed to resume route run: %w", err)
	}
	return nil
}

// initStateVars registers all unique state_var names from the route checkpoints.
func (r *Runner) initStateVars() {
	r.stateVars = make(map[string]*stateVarData)
	for _, cp := range r.route.Checkpoints {
		if cp.InventoryCheck == nil || cp.InventoryCheck.StateVar == "" {
			continue
		}
		name, _ := parseStateVar(cp.InventoryCheck.StateVar)
		if _, exists := r.stateVars[name]; !exists {
			r.stateVars[name] = &stateVarData{
				ItemID: cp.InventoryCheck.ItemID,
			}
		}
	}

	// Try to restore persisted state vars (for future run resumption)
	if len(r.stateVars) > 0 {
		rows, err := r.repo.LoadStateVars(r.runID)
		if err != nil {
			log.Printf("[Route] Failed to load state vars: %v", err)
			return
		}
		for _, row := range rows {
			if sv, ok := r.stateVars[row.VarName]; ok {
				sv.LastQuantity = row.LastQuantity
				sv.Acquired = row.Acquired
				sv.Consumed = row.Consumed
				sv.Initialized = true
			}
		}
	}
}

// Pause stops tracking the run without persisting any status change.
// The run stays in_progress in the database and can be resumed later.
func (r *Runner) Pause() {
	r.state.Pause()
}

// Abandon marks the current run as abandoned.
func (r *Runner) Abandon() error {
	r.state.Abandon()
	return r.repo.EndRouteRun(r.runID, string(RunAbandoned), r.state.LastDeathCount, r.state.LastIGT)
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

// CatchUp scans all checkpoint flags and marks any that are already set as completed.
// Returns nil when the scan completes, or an error if flag reading isn't ready yet (caller retries).
// Only CatchUp and Tick may read game data via the reader parameter.
func (r *Runner) CatchUp(reader GameReader) error {
	if !r.IsActive() {
		return nil
	}

	for _, cp := range r.route.Checkpoints {
		if r.state.CompletedFlags[cp.ID] {
			continue
		}

		if cp.EventFlagCheck != nil {
			flagSet, err := reader.ReadEventFlag(cp.EventFlagCheck.FlagID)
			if err != nil {
				// Not ready yet — caller should retry later
				return err
			}
			if flagSet {
				r.state.CompletedFlags[cp.ID] = true
				log.Printf("[Route] Already completed: %s", cp.Name)
				if err := r.repo.RecordCheckpoint(r.runID, cp.ID, cp.Name, 0, 0); err != nil {
					log.Printf("[Route] Failed to record caught-up checkpoint %s: %v", cp.ID, err)
				}
			}
		}

		if cp.InventoryCheck != nil && !r.state.CompletedFlags[cp.ID] {
			qty, err := reader.ReadInventoryItemQuantity(cp.InventoryCheck.ItemID)
			if err != nil {
				return err
			}
			checkQty := qty
			// Initialize state_var with current quantity as seed
			if cp.InventoryCheck.StateVar != "" {
				name, field := parseStateVar(cp.InventoryCheck.StateVar)
				if sv, ok := r.stateVars[name]; ok && !sv.Initialized {
					sv.LastQuantity = qty
					sv.Acquired = qty
					sv.Initialized = true
					sv.Dirty = true
				}
				if sv, ok := r.stateVars[name]; ok {
					if field == "consumed" {
						checkQty = sv.Consumed // consumed starts at 0 — won't auto-complete
					} else {
						checkQty = sv.Acquired
					}
				}
			}
			if compareValue(checkQty, cp.InventoryCheck.Comparison, cp.InventoryCheck.Value) {
				r.state.CompletedFlags[cp.ID] = true
				log.Printf("[Route] Already completed: %s", cp.Name)
				if err := r.repo.RecordCheckpoint(r.runID, cp.ID, cp.Name, 0, 0); err != nil {
					log.Printf("[Route] Failed to record caught-up checkpoint %s: %v", cp.ID, err)
				}
			}
		}

		// Also mark backup as done for already-completed checkpoints
		if cp.BackupFlagCheck != nil && r.state.CompletedFlags[cp.ID] {
			r.state.BackupDone[cp.ID] = true
		}
	}

	// Persist dirty state vars after catchup
	r.persistDirtyStateVars()

	return nil
}

// RestoreFromDB restores completed checkpoint state from the database
// instead of re-scanning game memory via CatchUp.
func (r *Runner) RestoreFromDB() error {
	ids, err := r.repo.LoadCompletedCheckpoints(r.runID)
	if err != nil {
		return fmt.Errorf("failed to restore from DB: %w", err)
	}
	// Build name lookup from route checkpoints
	nameOf := make(map[string]string, len(r.route.Checkpoints))
	for _, cp := range r.route.Checkpoints {
		nameOf[cp.ID] = cp.Name
	}
	for _, id := range ids {
		r.state.CompletedFlags[id] = true
		name := nameOf[id]
		if name == "" {
			name = id
		}
		log.Printf("[Route] Restored from DB: %s", name)
	}
	// Mark backup as done for completed checkpoints that have a backup flag
	for _, cp := range r.route.Checkpoints {
		if cp.BackupFlagCheck != nil && r.state.CompletedFlags[cp.ID] {
			r.state.BackupDone[cp.ID] = true
		}
	}
	return nil
}

// Tick is called every poll cycle. It reads event flags, death count, and IGT
// from the reader, processes the state machine, records splits, and triggers backups.
// Only CatchUp and Tick may read game data via the reader parameter.
func (r *Runner) Tick(reader GameReader) ([]CheckpointEvent, error) {
	if !r.IsActive() {
		return nil, nil
	}

	// Read death count (fall back to last known on transient error)
	deathCount, err := reader.ReadDeathCount()
	if err != nil {
		if !errors.Is(err, memreader.ErrNullPointer) {
			return nil, fmt.Errorf("read death count: %w", memreader.ErrGameRead)
		}
		deathCount = r.state.LastDeathCount
	}

	// Build tick input by reading only the active checkpoint window
	input := TickInput{
		Flags:           make(map[uint32]bool),
		BackupFlags:     make(map[uint32]bool),
		MemValues:       make(map[string]uint32),
		InventoryValues: make(map[string]uint32),
		DeathCount:      deathCount,
	}

	// Update ALL state vars regardless of active window (cumulative tracking)
	for name, sv := range r.stateVars {
		qty, err := reader.ReadInventoryItemQuantity(sv.ItemID)
		if err != nil {
			if !errors.Is(err, memreader.ErrNullPointer) {
				return nil, fmt.Errorf("read inventory for state var %s: %w", name, memreader.ErrGameRead)
			}
			continue
		}
		if !sv.Initialized {
			sv.LastQuantity = qty
			sv.Acquired = qty
			sv.Initialized = true
			sv.Dirty = true
		} else if qty > sv.LastQuantity {
			sv.Acquired += qty - sv.LastQuantity
			sv.Dirty = true
		} else if qty < sv.LastQuantity {
			sv.Consumed += sv.LastQuantity - qty
			sv.Dirty = true
		}
		sv.LastQuantity = qty
	}

	// Read conditions only for the active checkpoint window
	for _, cp := range r.state.ActiveCheckpoints() {
		// Read backup flag for active checkpoints
		if cp.BackupFlagCheck != nil && !r.state.BackupDone[cp.ID] {
			bfID := cp.BackupFlagCheck.FlagID
			if _, exists := input.BackupFlags[bfID]; !exists {
				flagSet, err := reader.ReadEventFlag(bfID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("read backup flag %d: %w", bfID, memreader.ErrGameRead)
					}
				} else {
					input.BackupFlags[bfID] = flagSet
				}
			}
		}

		// Flag-based checkpoint
		if cp.EventFlagCheck != nil {
			efID := cp.EventFlagCheck.FlagID
			if _, exists := input.Flags[efID]; !exists {
				flagSet, err := reader.ReadEventFlag(efID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("read event flag %d: %w", efID, memreader.ErrGameRead)
					}
					continue
				}
				input.Flags[efID] = flagSet
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
					return nil, fmt.Errorf("read memory value for %s: %w", cp.ID, memreader.ErrGameRead)
				}
				continue
			}
			input.MemValues[cp.ID] = val
		}

		// Inventory item quantity checkpoint
		if cp.InventoryCheck != nil {
			if cp.InventoryCheck.StateVar != "" {
				name, field := parseStateVar(cp.InventoryCheck.StateVar)
				if sv, ok := r.stateVars[name]; ok {
					if field == "consumed" {
						input.InventoryValues[cp.ID] = sv.Consumed
					} else {
						input.InventoryValues[cp.ID] = sv.Acquired
					}
				}
			} else {
				qty, err := reader.ReadInventoryItemQuantity(cp.InventoryCheck.ItemID)
				if err != nil {
					if !errors.Is(err, memreader.ErrNullPointer) {
						return nil, fmt.Errorf("read inventory for %s: %w", cp.ID, memreader.ErrGameRead)
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
			return nil, fmt.Errorf("read IGT: %w", memreader.ErrGameRead)
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
		if err := r.repo.RecordCheckpoint(r.runID, evt.Checkpoint.ID, evt.Checkpoint.Name,
			evt.IGT, evt.CheckpointDuration); err != nil {
			log.Printf("Failed to record checkpoint: %v", err)
		}

		// Update PB if better
		if err := r.repo.UpdatePersonalBest(r.route.ID, evt.Checkpoint.ID,
			evt.IGT, evt.CheckpointDuration); err != nil {
			log.Printf("Failed to update PB: %v", err)
		}

		// Trigger save backup on kill if no encounter backup was configured
		if evt.Checkpoint.BackupFlagCheck == nil {
			r.triggerBackup(evt.Checkpoint.ID)
		}
	}

	// Check if run is complete
	if r.state.Status == RunCompleted {
		if err := r.repo.EndRouteRun(r.runID, string(RunCompleted),
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
		if err := r.repo.SaveStateVar(r.runID, name, sv.ItemID, sv.LastQuantity, sv.Acquired, sv.Consumed); err != nil {
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
	cfg, ok := memreader.GetGameConfig(r.route.Game)
	if !ok {
		return nil
	}
	return cfg
}
