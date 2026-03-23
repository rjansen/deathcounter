package monitor

import (
	"log"

	"github.com/rjansen/deathcounter/internal/backup"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
)

// RouteMonitor tracks death counts and speedrun route progress.
type RouteMonitor struct {
	GameMonitor[RouteMonitorState]
	runner      *route.Runner
	routes      []*route.Route
	backupMgr   *backup.Manager
	backupCount int
}

// NewRouteMonitor creates a new route monitor.
func NewRouteMonitor(reader *memreader.GameReader, tracker *stats.Tracker, routes []*route.Route, backupMgr *backup.Manager) *RouteMonitor {
	return &RouteMonitor{
		GameMonitor: InitGameMonitor[RouteMonitorState](reader, tracker),
		routes:      routes,
		backupMgr:   backupMgr,
	}
}

// Tick performs one monitoring cycle with route tracking.
func (m *RouteMonitor) Tick() {
	m.TryAttach()

	// Disconnected: abandon any active runner, publish empty state
	if m.Phase == PhaseDisconnected {
		if m.runner != nil && m.runner.IsActive() {
			m.runner.Abandon()
		}
		m.publishRouteState()
		return
	}

	// PhaseConnected: attempt save detection, start route if loaded
	if m.Phase == PhaseConnected {
		m.TryDetectSave()
		if m.Phase == PhaseLoaded {
			m.startMatchingRoute()
		}
		m.publishRouteState()
		return
	}

	// PhaseLoaded or PhaseRouteRunning: check for save changes
	saveChanged, _ := m.TryDetectSave()
	if saveChanged {
		// Save identity changed: abandon active run and restart
		if m.runner != nil && m.runner.IsActive() {
			m.runner.Abandon()
		}
		m.Tracker.EndCurrentSession()
		m.startMatchingRoute()
	}

	// Phase-based CatchUp retry: stay in PhaseLoaded until CatchUp succeeds
	if m.Phase == PhaseLoaded && m.runner != nil && m.runner.IsActive() {
		if m.runner.CatchUp(m.Reader) {
			m.Phase = PhaseRouteRunning
		}
	}

	count, ok := m.ReadDeathCount()
	if !ok {
		m.publishRouteState()
		return
	}

	m.RecordDeathIfChanged(count)
	m.ReadHollowing()

	// Tick route runner if active and CatchUp is done
	if m.Phase == PhaseRouteRunning && m.runner != nil && m.runner.IsActive() {
		events, err := m.runner.Tick(m.Reader, m.LastCount)
		if err != nil {
			log.Printf("Route tracking error: %v", err)
		}
		for _, evt := range events {
			log.Printf("[Route] Checkpoint: %s (IGT: %dms, Deaths: %d)",
				evt.Checkpoint.Name, evt.IGT, evt.Deaths)
		}
		m.backupCount += m.countBackups(events)
	}

	m.publishRouteState()
}

func (m *RouteMonitor) startMatchingRoute() {
	for _, r := range m.routes {
		if r.Game == m.GameName() {
			m.runner = route.NewRunner(r, m.Tracker, m.backupMgr)
			if err := m.runner.Start(0, m.CurrentSaveID); err != nil {
				log.Printf("Failed to start route run: %v", err)
				m.runner = nil
			} else {
				log.Printf("[Route] Started route: %s", r.Name)
				m.backupCount = 0
				// Attempt CatchUp immediately; if it fails, Phase stays PhaseLoaded
				// and Tick will retry on subsequent cycles
				if m.runner.CatchUp(m.Reader) {
					m.Phase = PhaseRouteRunning
				}
			}
			return
		}
	}
}

func (m *RouteMonitor) countBackups(events []route.CheckpointEvent) int {
	// Backups are triggered inside runner.Tick, count checkpoint events
	// that would have triggered backups
	count := 0
	for _, evt := range events {
		if evt.Checkpoint.BackupFlagID == 0 {
			// Kill-based backup
			count++
		}
	}
	return count
}

func (m *RouteMonitor) publishRouteState() {
	state := RouteMonitorState{
		GameName:      m.GameName(),
		Status:        m.StatusText(),
		DeathCount:    m.LastCount,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
		Hollowing:     m.CurrentHollowing,
	}

	if m.runner != nil && m.runner.IsActive() {
		r := m.runner.GetRoute()
		state.RouteName = r.Name
		state.CompletionPercent = m.runner.CompletionPercent()
		state.CompletedCount = m.runner.CompletedCount()
		state.TotalCount = m.runner.TotalCount()
		state.SegmentDeaths = m.runner.SegmentDeaths()
		state.BackupCount = m.backupCount

		cp := m.runner.CurrentCheckpoint()
		if cp != nil {
			state.CurrentCheckpoint = cp.Name
		}
	}

	m.PublishState(state)
}
