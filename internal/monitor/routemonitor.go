package monitor

import (
	"errors"
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
	route       *route.Route
	backupMgr   *backup.Manager
	backupCount int
}

// NewRouteMonitor creates a new route monitor.
func NewRouteMonitor(reader *memreader.GameReader, tracker *stats.Tracker, r *route.Route, backupMgr *backup.Manager) *RouteMonitor {
	return &RouteMonitor{
		GameMonitor: InitGameMonitor[RouteMonitorState](reader, tracker),
		route:       r,
		backupMgr:   backupMgr,
	}
}

// Start begins the monitoring tick loop.
func (m *RouteMonitor) Start() {
	m.StartLoop(m.Tick)
}

// Tick performs one monitoring cycle with route tracking.
func (m *RouteMonitor) Tick() error {
	if _, err := m.Attach(); errors.Is(err, ErrNoGame) {
		m.abandonRun()
		return err
	}

	if m.Phase == PhaseConnected {
		m.startRun()
		return nil
	}

	_, err := m.DetectSave()
	if errors.Is(err, ErrSaveChanged) {
		m.handleSaveChanged()
	}

	if m.Phase == PhaseLoaded && m.runner != nil && m.runner.IsActive() {
		m.catchUpRun()
	}

	count, readErr := m.Reader.ReadDeathCount()
	if readErr != nil {
		m.handleReadError(readErr)
		return readErr
	}

	m.RecordDeathIfChanged(count)
	m.readHollowing()

	if m.Phase == PhaseRouteRunning && m.runner != nil && m.runner.IsActive() {
		m.tickRun()
	}

	m.publishRouteState()
	return nil
}

func (m *RouteMonitor) abandonRun() {
	if m.runner != nil && m.runner.IsActive() {
		m.runner.Abandon()
	}
	m.publishRouteState()
}

func (m *RouteMonitor) startRun() {
	_, err := m.DetectSave()
	if err == nil || errors.Is(err, ErrSaveNotSupported) {
		m.Phase = PhaseLoaded
		m.startRouteRun()
	}
	m.publishRouteState()
}

func (m *RouteMonitor) handleSaveChanged() {
	if m.runner != nil && m.runner.IsActive() {
		m.runner.Abandon()
	}
	m.Tracker.EndCurrentSession()
	m.startRouteRun()
}

func (m *RouteMonitor) catchUpRun() {
	if err := m.runner.CatchUp(); err == nil {
		m.Phase = PhaseRouteRunning
	}
}

func (m *RouteMonitor) handleReadError(readErr error) {
	if errors.Is(readErr, memreader.ErrNullPointer) {
		if !m.loadLoggedOnce {
			log.Printf("[%s] Waiting for game to fully load...", m.Reader.GetCurrentGame())
			m.loadLoggedOnce = true
		}
	} else {
		log.Printf("[%s] Disconnected: %v", m.Reader.GetCurrentGame(), readErr)
		m.Reader.Detach()
		m.Phase = PhaseDisconnected
		m.LastGame = ""
	}
	m.publishRouteState()
}

func (m *RouteMonitor) readHollowing() {
	val, err := m.Reader.ReadHollowing()
	if err != nil {
		m.CurrentHollowing = 0
	} else {
		m.CurrentHollowing = val
	}
}

func (m *RouteMonitor) tickRun() {
	events, err := m.runner.Tick()
	if err != nil {
		log.Printf("Route tracking error: %v", err)
	}
	for _, evt := range events {
		log.Printf("[Route] Checkpoint: %s (IGT: %dms, Deaths: %d)",
			evt.Checkpoint.Name, evt.IGT, evt.Deaths)
	}
	m.backupCount += m.countBackups(events)
}

func (m *RouteMonitor) startRouteRun() {
	if m.route == nil || m.route.Game != m.GameName() {
		return
	}
	m.runner = route.NewRunner(m.route, m.Tracker, m.backupMgr, m.Reader)
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
				if err := m.runner.CatchUp(); err == nil {
					m.Phase = PhaseRouteRunning
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
	// Attempt CatchUp immediately; if it fails, Phase stays PhaseLoaded
	// and Tick will retry on subsequent cycles
	if err := m.runner.CatchUp(); err == nil {
		m.Phase = PhaseRouteRunning
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
