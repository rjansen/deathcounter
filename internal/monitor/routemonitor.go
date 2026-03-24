package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
)

// RouteMonitor tracks death counts and speedrun route progress.
type RouteMonitor struct {
	GameMonitor[RouteMonitorState]
	runner      *route.Runner
	route       *route.Route
	routeID     string
	routesDir   string
	backupCount int
}

// NewRouteMonitor creates a new route monitor.
func NewRouteMonitor(gameID, routeID, routesDir string, ops memreader.ProcessOps, tracker *stats.Tracker) *RouteMonitor {
	return &RouteMonitor{
		GameMonitor: NewGameMonitor[RouteMonitorState](gameID, ops, tracker),
		routeID:     routeID,
		routesDir:   routesDir,
	}
}

// Start begins the monitoring tick loop.
func (m *RouteMonitor) Start() {
	m.StartLoop(m)
}

// OnAttach validates the attached game and loads the route definition.
func (m *RouteMonitor) OnAttach(gameID string) error {
	if m.gameID != gameID {
		log.Printf("[Route] Selected game %q does not match attached game %q", m.gameID, gameID)
		return ErrAttachedGameMismatch
	}
	r, err := route.LoadRouteByID(gameID, m.routeID, m.routesDir)
	if err != nil {
		log.Printf("[Route] Failed to load route %q: %v", m.routeID, err)
		return fmt.Errorf("failed to load route %q of the game %q: %w", m.routeID, m.gameID, err)
	}
	m.route = r
	return nil
}

// OnDetach handles game detachment: abandons active run and clears state.
func (m *RouteMonitor) OnDetach() {
	if m.isRunning() {
		m.runner.Abandon()
	}
	m.route = nil
	m.runner = nil
	m.publishRouteState()
}

// isRunning returns true if the runner exists and is actively tracking a run.
func (m *RouteMonitor) isRunning() bool {
	return m.runner != nil && m.runner.IsActive()
}

// Tick performs one monitoring cycle with route tracking.
// Always called in PhaseLoaded or beyond.
func (m *RouteMonitor) Tick(reader *memreader.GameReader) error {
	_, err := m.DetectSave(reader)
	if errors.Is(err, ErrSaveChanged) {
		m.handleSaveChanged(reader)
	}

	if !m.isRunning() {
		m.startRouteRun(reader)
	}

	if m.Phase == PhaseRunning && m.isRunning() {
		return m.tickRun(reader)
	}

	m.publishRouteState()
	return nil
}

func (m *RouteMonitor) handleSaveChanged(reader *memreader.GameReader) {
	if m.isRunning() {
		m.runner.Abandon()
	}
	m.Tracker.EndCurrentSession()
	m.startRouteRun(reader)
}

func (m *RouteMonitor) tickRun(reader *memreader.GameReader) error {
	events, err := m.runner.Tick(reader)
	if err != nil {
		m.publishRouteState()
		return err
	}
	m.RecordDeathIfChanged(m.runner.LastDeathCount())
	for _, evt := range events {
		log.Printf("[Route] Checkpoint: %s (IGT: %dms, Deaths: %d)",
			evt.Checkpoint.Name, evt.IGT, evt.Deaths)
	}
	m.backupCount += m.countBackups(events)
	m.publishRouteState()
	return nil
}

func (m *RouteMonitor) startRouteRun(reader *memreader.GameReader) {
	if m.route == nil {
		return
	}

	m.runner = route.NewRunner(m.route, m.Tracker, nil)
	m.backupCount = 0

	// Check for an existing in-progress run for this route+save
	if m.CurrentSaveID > 0 {
		runID, found, err := m.Tracker.FindInProgressRun(m.route.ID, m.CurrentSaveID)
		if err != nil {
			log.Printf("[Route] Failed to check for in-progress run: %v", err)
		} else if found {
			if err := m.runner.Resume(runID, 0); err != nil {
				log.Printf("[Route] Failed to resume run %d: %v", runID, err)
			} else {
				log.Printf("[Route] Resumed route: %s (run %d)", m.route.Name, runID)
				if err := m.runner.CatchUp(reader); err == nil {
					m.Phase = PhaseRunning
				} else {
					m.runner = nil
				}
				return
			}
		}
	}

	if err := m.runner.Start(0, m.CurrentSaveID); err != nil {
		log.Printf("Failed to start route run: %v", err)
		m.runner = nil
		return
	}
	log.Printf("[Route] Started route: %s", m.route.Name)
	if err := m.runner.CatchUp(reader); err == nil {
		m.Phase = PhaseRunning
	} else {
		m.runner = nil
	}
}

func (m *RouteMonitor) countBackups(events []route.CheckpointEvent) int {
	count := 0
	for _, evt := range events {
		if evt.Checkpoint.BackupFlagCheck == nil {
			count++
		}
	}
	return count
}

func (m *RouteMonitor) publishRouteState() {
	state := RouteMonitorState{
		GameName:      m.GameLabel(),
		Status:        m.StatusText(),
		DeathCount:    m.LastCount,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
	}

	if m.isRunning() {
		r := m.runner.GetRoute()
		cp := m.runner.CurrentCheckpoint()
		cpName := ""
		if cp != nil {
			cpName = cp.Name
		}
		state.Route = &RouteDisplay{
			RouteName:         r.Name,
			CompletionPercent: m.runner.CompletionPercent(),
			CompletedCount:    m.runner.CompletedCount(),
			TotalCount:        m.runner.TotalCount(),
			CurrentCheckpoint: cpName,
			SegmentDeaths:     m.runner.SegmentDeaths(),
			BackupCount:       m.backupCount,
		}
	}

	m.PublishState(state)
}
