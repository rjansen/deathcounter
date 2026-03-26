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

	state          MonitorState
	attachedGameID string
}

// NewGameMonitor creates a GameMonitor that delegates tick processing to the tracker.
func NewGameMonitor(gameID string, ops memreader.ProcessOps, tracker GameTracker) *GameMonitor {
	return &GameMonitor{
		ops:     ops,
		gameID:  gameID,
		tracker: tracker,
		state:   &detachedState{},
	}
}

// GameID returns the target game ID.
func (m *GameMonitor) GameID() string {
	return m.gameID
}

// setState transitions the monitor to a new state.
func (m *GameMonitor) setState(s MonitorState) {
	m.state = s
}

// detachReader closes the reader without touching state or tracker.
func (m *GameMonitor) detachReader() {
	if m.reader != nil {
		log.Printf("[%s] Detached", m.gameID)
		m.reader.Detach()
		m.reader = nil
	}
}

// Start creates a 500ms ticker, runs the tick loop, and returns the display
// channel. The channel is closed when the monitor stops.
// Each tick delegates to state.Tick() which handles attach, tracker processing,
// error recovery, and publishing internally.
func (m *GameMonitor) Start() <-chan DisplayUpdate {
	m.displayCh = make(chan DisplayUpdate, 1)
	m.stopCh = make(chan struct{})
	m.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		defer close(m.displayCh)
		for {
			select {
			case <-m.ticker.C:
				if err := m.state.Tick(m); err != nil && !errors.Is(err, ErrNoGame) {
					log.Printf("[%s] %v", m.gameID, err)
				}
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
// Non-blocking: drops old data if not consumed. No-op if channel is nil.
func (m *GameMonitor) publish(update DisplayUpdate) {
	if m.displayCh == nil {
		return
	}
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
