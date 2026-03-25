package monitor

import (
	"errors"
	"fmt"
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
	running   bool // true when a route run is active
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
	}
}

// OnAttach validates the attached game and loads the route definition.
func (t *RouteTracker) OnAttach(gameID string) error {
	if t.gameID != gameID {
		log.Printf("[Route] Selected game %q does not match attached game %q", t.gameID, gameID)
		return ErrAttachedGameMismatch
	}
	r, err := route.LoadRouteByID(gameID, t.routeID, t.routesDir)
	if err != nil {
		log.Printf("[Route] Failed to load route %q: %v", t.routeID, err)
		return fmt.Errorf("failed to load route %q of the game %q: %w", t.routeID, t.gameID, err)
	}
	t.route = r
	return nil
}

// OnDetach handles game detachment: pauses active run and clears state.
func (t *RouteTracker) OnDetach() {
	if t.isRunning() {
		t.runner.Pause()
	}
	t.route = nil
	t.runner = nil
	t.running = false
	t.resetOnDetach()
}

// isRunning returns true if the runner exists and is actively tracking a run.
func (t *RouteTracker) isRunning() bool {
	return t.runner != nil && t.runner.IsActive()
}

// Tick performs one monitoring cycle with route tracking.
func (t *RouteTracker) Tick(reader *memreader.GameReader) (DisplayUpdate, error) {
	_, err := t.detectSave(reader)
	if errors.Is(err, ErrSaveChanged) {
		t.handleSaveChanged(reader)
	}

	if !t.isRunning() {
		t.startRouteRun(reader)
	}

	if t.running && t.isRunning() {
		return t.tickRun(reader)
	}

	return t.buildUpdate(), nil
}

func (t *RouteTracker) handleSaveChanged(reader *memreader.GameReader) {
	if t.isRunning() {
		t.runner.Pause()
	}
	t.running = false
	t.startRouteRun(reader)
}

func (t *RouteTracker) tickRun(reader *memreader.GameReader) (DisplayUpdate, error) {
	_, err := t.runner.Tick(reader)
	if err != nil {
		return t.buildUpdate(), err
	}
	t.recordDeathIfChanged(t.runner.LastDeathCount())
	return t.buildUpdate(), nil
}

func (t *RouteTracker) startRouteRun(reader *memreader.GameReader) {
	if t.route == nil {
		return
	}

	t.runner = route.NewRunner(t.route, t.repo, nil)

	// Try to find the latest run for this route+save
	runID, status, err := t.repo.FindLatestRun(t.route.ID, t.currentSaveID)
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		log.Printf("[Route] Failed to find latest run: %v", err)
	}
	if err == nil && (status == string(route.RunNotStarted) || status == string(route.RunInProgress) || status == string(route.RunPaused)) {
		if err := t.runner.Resume(runID, 0); err != nil {
			log.Printf("[Route] Failed to resume run %d: %v", runID, err)
		} else {
			log.Printf("[Route] Resumed route: %s (run %d)", t.route.Name, runID)
			if err := t.runner.CatchUp(reader); err == nil {
				t.running = true
			} else {
				t.runner = nil
			}
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
		t.running = true
	} else {
		t.runner = nil
	}
}

func (t *RouteTracker) statusText() string {
	if t.running && t.isRunning() {
		return "Tracking route"
	}
	return PhaseLoaded.StatusText()
}

func (t *RouteTracker) buildUpdate() DisplayUpdate {
	update := DisplayUpdate{
		GameName:      t.gameLabel(),
		Status:        t.statusText(),
		DeathCount:    t.lastCount,
		CharacterName: t.currentCharName,
		SaveSlotIndex: t.currentSlotIdx,
	}

	if t.isRunning() {
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
	}

	return update
}
