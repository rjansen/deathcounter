package monitor

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// Sentinel errors for monitor state transitions.
var (
	ErrNoGame               = errors.New("no game found")
	ErrGameDetached         = errors.New("game detached")
	ErrSaveNotSupported     = errors.New("save detection not supported")
	ErrSavePending          = errors.New("save detection pending")
	ErrSaveChanged          = errors.New("save identity changed")
	ErrAttachedGameMismatch = errors.New("attached game mismatch")
)

// Monitor is the interface tray.App uses to drive the monitoring lifecycle.
type Monitor interface {
	Start() <-chan DisplayUpdate
	Stop()
}

// GameTracker processes game data each tick and returns a display state.
// Implementations must not embed GameMonitor — they are distinct structs.
type GameTracker interface {
	OnAttach(gameID string) error
	OnDetach()
	Tick(reader *memreader.GameReader) (DisplayUpdate, error)
}

// GameMonitor manages the game attach/detach lifecycle, the tick loop,
// and pushes display updates to the tray.
type GameMonitor struct {
	ops     memreader.ProcessOps
	gameID  string
	reader  *memreader.GameReader
	tracker GameTracker

	displayCh chan DisplayUpdate

	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once

	phase          MonitorPhase
	attachedGameID string
}

// NewGameMonitor creates a GameMonitor that delegates tick processing to the tracker.
func NewGameMonitor(gameID string, ops memreader.ProcessOps, tracker GameTracker) *GameMonitor {
	return &GameMonitor{
		ops:     ops,
		gameID:  gameID,
		tracker: tracker,
		phase:   PhaseDetached,
	}
}

// GameID returns the target game ID.
func (m *GameMonitor) GameID() string {
	return m.gameID
}

// Attach attempts to attach to the target game process.
// Returns the GameReader and nil on success, or ErrNoGame / ErrGameDetached.
// If already attached, returns the existing reader.
func (m *GameMonitor) Attach() (*memreader.GameReader, error) {
	if m.reader != nil {
		return m.reader, nil
	}
	cfg, proc, err := memreader.FindGame(m.ops, m.gameID)
	if err != nil {
		if m.phase > PhaseDetached {
			m.Detach()
			return nil, ErrGameDetached
		}
		return nil, ErrNoGame
	}
	m.reader = memreader.NewGameReader(m.ops, cfg, proc)
	m.attachedGameID = cfg.ID
	log.Printf("Attached to: %s (%s)", cfg.Label, cfg.ID)
	m.phase = PhaseAttached
	return m.reader, nil
}

// Detach closes the reader and resets phase to Detached.
func (m *GameMonitor) Detach() {
	if m.reader != nil {
		log.Printf("[%s] Detached", m.gameID)
		m.reader.Detach()
		m.reader = nil
	}
	m.phase = PhaseDetached
}

// Start creates a 500ms ticker, runs the tick loop, and returns the display
// channel. The channel is closed when the monitor stops.
// It manages PhaseDetached → PhaseAttached → PhaseLoaded transitions.
// Tick is only called when Phase == PhaseLoaded.
func (m *GameMonitor) Start() <-chan DisplayUpdate {
	m.displayCh = make(chan DisplayUpdate, 1)
	m.stopCh = make(chan struct{})
	m.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		defer close(m.displayCh)
		for {
			select {
			case <-m.ticker.C:
				reader, err := m.Attach()
				if errors.Is(err, ErrGameDetached) {
					m.tracker.OnDetach()
					m.publishDetached()
					continue
				}
				if err != nil {
					continue // ErrNoGame — not yet attached, just wait
				}

				// PhaseAttached → PhaseLoaded (via OnAttach)
				if m.phase == PhaseAttached {
					if err := m.tracker.OnAttach(m.attachedGameID); err != nil {
						log.Printf("[%s] OnAttach error: %v", m.gameID, err)
						m.Detach()
						m.tracker.OnDetach()
						m.publishDetached()
						continue
					}
					m.phase = PhaseLoaded
					continue
				}

				// PhaseLoaded: call Tick
				update, err := m.tracker.Tick(reader)
				if err != nil {
					if errors.Is(err, memreader.ErrGameRead) {
						m.Detach()
						m.tracker.OnDetach()
						m.publishDetached()
					}
					log.Printf("[%s] Tick error: %v", m.gameID, err)
					continue
				}
				m.publish(update)

			case <-m.stopCh:
				return
			}
		}
	}()
	return m.displayCh
}

// Stop halts the tick loop. The display channel is closed by the goroutine.
func (m *GameMonitor) Stop() {
	m.stopOnce.Do(func() {
		if m.ticker != nil {
			m.ticker.Stop()
		}
		if m.stopCh != nil {
			close(m.stopCh)
		}
	})
}

// publish sends a DisplayUpdate on the display channel.
// Non-blocking: drops old data if not consumed.
func (m *GameMonitor) publish(update DisplayUpdate) {
	select {
	case m.displayCh <- update:
	default:
		select {
		case <-m.displayCh:
		default:
		}
		m.displayCh <- update
	}
}

// publishDetached publishes a minimal state for when no game is connected.
func (m *GameMonitor) publishDetached() {
	m.publish(DisplayUpdate{
		Status: m.phase.StatusText(),
	})
}
