package monitor

import (
	"errors"
	"log"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
)

// RouteTracker tracks death counts and speedrun route progress.
// It implements GameTracker.
type RouteTracker struct {
	baseTracker
	runner    *route.Runner
	route     *route.Route
	routeID   string
	routesDir string
	state     trackerState
}

// NewRouteTracker creates a new route tracker.
func NewRouteTracker(gameID, routeID, routesDir string, repo *data.Repository) *RouteTracker {
	return &RouteTracker{
		baseTracker: baseTracker{
			gameID: gameID,
			repo:   repo,
		},
		routeID:   routeID,
		routesDir: routesDir,
		state:     &routeStoppedState{},
	}
}

// setTrackerState transitions the tracker to a new state.
func (t *RouteTracker) setTrackerState(s trackerState) {
	t.state = s
}

// OnAttach delegates to the current state.
func (t *RouteTracker) OnAttach(gameID string) error {
	return t.state.OnAttach(t, gameID)
}

// OnDetach delegates to the current state.
func (t *RouteTracker) OnDetach() {
	t.state.OnDetach(t)
}

// Tick delegates to the current state.
func (t *RouteTracker) Tick(reader *memreader.GameReader) (DisplayUpdate, error) {
	return t.state.Tick(t, reader)
}

func (t *RouteTracker) handleSaveChanged(reader *memreader.GameReader) {
	if t.runner != nil && t.runner.IsActive() {
		t.runner.Pause()
	}
	t.setTrackerState(&routeStoppedState{})
	t.startRouteRun(reader)
}

func (t *RouteTracker) tickRun(reader *memreader.GameReader) (DisplayUpdate, error) {
	events, err := t.runner.Tick(reader)
	if err != nil {
		return t.buildUpdate(nil), err
	}
	t.recordDeathIfChanged(t.runner.LastDeathCount())
	return t.buildUpdate(events), nil
}

func (t *RouteTracker) startRouteRun(reader *memreader.GameReader) {
	if t.route == nil {
		return
	}

	t.runner = route.NewRunner(t.route, t.repo, nil)

	// Try to find the latest run for this route+save
	run, err := t.repo.FindLatestRun(t.route.ID, t.currentSaveID)
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		log.Printf("[Route] Failed to find latest run: %v", err)
	}
	if err == nil && (run.Status == string(route.RunNotStarted) || run.Status == string(route.RunInProgress) || run.Status == string(route.RunPaused)) {
		if err := t.runner.Resume(run.ID, 0); err != nil {
			log.Printf("[Route] Failed to resume run %d: %v", run.ID, err)
		} else {
			log.Printf("[Route] Resumed route: %s (run %d)", t.route.Name, run.ID)
			t.setTrackerState(&routeRunningState{})
			return
		}
	}

	// No resumable run found (or resume failed) → start fresh
	if err := t.runner.Start(0, t.currentSaveID); err != nil {
		log.Printf("Failed to start route run: %v", err)
		t.runner = nil
		return
	}
	log.Printf("[Route] Started route: %s", t.route.Name)
	if err := t.runner.CatchUp(reader); err == nil {
		t.setTrackerState(&routeRunningState{})
	} else {
		t.runner = nil
	}
}

func (t *RouteTracker) statusText() string {
	if t.state.IsRunning() {
		return "Tracking route"
	}
	return PhaseLoaded.StatusText()
}

func (t *RouteTracker) buildUpdate(events []route.CheckpointEvent) DisplayUpdate {
	update := DisplayUpdate{
		GameName:      t.gameLabel(),
		Status:        t.statusText(),
		DeathCount:    t.lastCount,
		CharacterName: t.currentCharName,
		SaveSlotIndex: t.currentSlotIdx,
	}

	if t.state.IsRunning() && t.runner != nil {
		update.IGT = t.runner.LastIGT()
		r := t.runner.GetRoute()
		cp := t.runner.CurrentCheckpoint()
		cpName := ""
		if cp != nil {
			cpName = cp.Name
		}
		update.Route = &RouteDisplay{
			RouteName:         r.Name,
			CompletionPercent: t.runner.CompletionPercent(),
			CompletedCount:    t.runner.CompletedCount(),
			TotalCount:        t.runner.TotalCount(),
			CurrentCheckpoint: cpName,
			SegmentDeaths:     t.runner.SegmentDeaths(),
		}
		if len(events) > 0 {
			notifs := make([]CheckpointNotification, len(events))
			for i, evt := range events {
				notifs[i] = CheckpointNotification{
					Name:     evt.Checkpoint.Name,
					IGT:      evt.IGT,
					Duration: evt.CheckpointDuration,
					Deaths:   evt.Deaths,
				}
			}
			update.Route.CompletedEvents = notifs
		}
	}

	return update
}
