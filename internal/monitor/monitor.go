package monitor

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// Sentinel errors for monitor state transitions.
var (
	ErrNoGame           = errors.New("no game found")
	ErrSaveNotSupported = errors.New("save detection not supported")
	ErrSavePending      = errors.New("save detection pending")
	ErrSaveChanged      = errors.New("save identity changed")
)

// Monitor is the interface tray.App uses to drive the monitoring lifecycle.
type Monitor interface {
	Start()
	Stop()
	DisplayUpdates() <-chan DisplayUpdate
}

// DisplayUpdate is the common display state consumed by tray.
// Fields holds domain-specific key-value data (e.g. route progress fields)
// so the struct stays generic without needing typed sub-structs.
type DisplayUpdate struct {
	GameName      string
	Status        string
	DeathCount    uint32
	CharacterName string
	SaveSlotIndex int
	Hollowing     uint32
	Fields        map[string]any
}

// Displayable is the constraint for State types — they must know
// how to convert themselves into a DisplayUpdate for the tray.
type Displayable interface {
	ToDisplayUpdate() DisplayUpdate
}

// GameMonitor is the generic base that manages the game attach/detach
// lifecycle, death count reading, and pushes typed state updates.
type GameMonitor[S Displayable] struct {
	Reader  *memreader.GameReader
	Tracker *stats.Tracker

	updates   chan S
	displayCh chan DisplayUpdate

	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once

	LastCount uint32
	LastGame  string
	Phase     MonitorPhase

	// Save slot tracking
	CurrentSaveID    int64
	CurrentSlotIdx   int
	CurrentCharName  string
	CurrentHollowing uint32

	// Log spam prevention (reset on attach/detach)
	saveLoggedOnce bool
	loadLoggedOnce bool
}

// InitGameMonitor initializes a GameMonitor with buffered channels.
func InitGameMonitor[S Displayable](reader *memreader.GameReader, tracker *stats.Tracker) GameMonitor[S] {
	return GameMonitor[S]{
		Reader:    reader,
		Tracker:   tracker,
		updates:   make(chan S, 1),
		displayCh: make(chan DisplayUpdate, 1),
		Phase:     PhaseDisconnected,
	}
}

// DisplayUpdates returns the common display channel for tray.
func (m *GameMonitor[S]) DisplayUpdates() <-chan DisplayUpdate {
	return m.displayCh
}

// TypedUpdates returns the typed state channel.
func (m *GameMonitor[S]) TypedUpdates() <-chan S {
	return m.updates
}

// GameName returns the current game ID.
func (m *GameMonitor[S]) GameName() string {
	return m.LastGame
}

// GameLabel returns the display label for the current game.
func (m *GameMonitor[S]) GameLabel() string {
	if m.LastGame == "" {
		return ""
	}
	return memreader.GetGameLabel(m.LastGame)
}

// StatusText returns the current status string derived from the Phase.
func (m *GameMonitor[S]) StatusText() string {
	return m.Phase.StatusText()
}

// IsAttached returns whether a game is currently attached.
func (m *GameMonitor[S]) IsAttached() bool {
	return m.Reader.IsAttached()
}

// Attach attempts to attach to a game process, detects game changes,
// and updates status. Returns the game ID and nil on success, or ErrNoGame
// if no supported game is running.
func (m *GameMonitor[S]) Attach() (string, error) {
	if !m.Reader.IsAttached() {
		if err := m.Reader.Attach(); err != nil {
			if m.LastGame != "" {
				log.Printf("[%s] Game process ended", m.LastGame)
				m.Phase = PhaseDisconnected
				m.LastGame = ""
				m.LastCount = 0
			}
			return "", ErrNoGame
		}
	}

	currentGame := m.Reader.GetCurrentGame()
	if currentGame != m.LastGame {
		log.Printf("Attached to: %s", currentGame)
		m.Phase = PhaseConnected
		m.LastGame = currentGame
		m.LastCount = 0
		m.CurrentSaveID = 0
		m.CurrentSlotIdx = 0
		m.CurrentCharName = ""
		m.saveLoggedOnce = false
		m.loadLoggedOnce = false
	}
	return m.LastGame, nil
}

// DetectSave attempts to read the save slot identity from game memory.
// Returns the save ID and nil on success (same save or first detection),
// ErrSaveNotSupported if the game doesn't support save detection,
// ErrSavePending for transient failures, ErrSaveChanged when identity changes,
// or a wrapped error for DB failures.
// Phase transitions (Connected → Loaded) are left to callers.
func (m *GameMonitor[S]) DetectSave() (int64, error) {
	charName, nameErr := m.Reader.ReadCharacterName()
	slotIdx, slotErr := m.Reader.ReadSaveSlotIndex()

	// If character name is not supported, skip save detection entirely
	if isUnsupportedErr(nameErr) {
		return 0, ErrSaveNotSupported
	}

	// Null pointer or read error means game data is not yet loaded — retry later
	if nameErr != nil {
		if !m.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: %v", m.Reader.GetCurrentGame(), nameErr)
			m.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Empty name means the structure exists but save data isn't populated yet
	if charName == "" {
		if !m.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: empty character name", m.Reader.GetCurrentGame())
			m.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Save slot is optional — use 0 if unavailable
	if slotErr != nil {
		slotIdx = 0
	}

	// Slot 255 is uninitialized memory — treat as not yet loaded
	if slotIdx == 255 {
		if !m.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: uninitialized slot (255)", m.Reader.GetCurrentGame())
			m.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Check if save identity changed
	if slotIdx != m.CurrentSlotIdx || charName != m.CurrentCharName {
		saveID, err := m.Tracker.FindOrCreateSave(m.Reader.GetCurrentGame(), slotIdx, charName)
		if err != nil {
			log.Printf("[%s] Failed to create save record: %v", m.Reader.GetCurrentGame(), err)
			return 0, fmt.Errorf("failed to create save record: %w", err)
		}

		previouslyLoaded := m.Phase >= PhaseLoaded
		m.CurrentSaveID = saveID
		m.CurrentSlotIdx = slotIdx
		m.CurrentCharName = charName

		if !previouslyLoaded {
			log.Printf("[%s] Save detected: %s (Slot %d)", m.Reader.GetCurrentGame(), charName, slotIdx)
			log.Printf("[%s] Game loaded successfully", m.Reader.GetCurrentGame())
		} else {
			log.Printf("[%s] Save changed: %s (Slot %d)", m.Reader.GetCurrentGame(), charName, slotIdx)
			return saveID, ErrSaveChanged
		}
		return saveID, nil
	}

	return m.CurrentSaveID, nil
}

// RecordDeathIfChanged checks if the death count changed and records it.
// Returns true if the count changed.
func (m *GameMonitor[S]) RecordDeathIfChanged(count uint32) bool {
	if count != m.LastCount {
		log.Printf("[%s] Death count: %d (previous: %d)", m.Reader.GetCurrentGame(), count, m.LastCount)
		if m.CurrentSaveID > 0 {
			m.Tracker.RecordDeathForSave(count, m.CurrentSaveID)
		} else {
			m.Tracker.RecordDeath(count)
		}
		m.LastCount = count
		return true
	}
	return false
}

// isUnsupportedErr checks if an error indicates the feature is not supported.
func isUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, memreader.ErrNotSupported)
}

// StartLoop creates a 500ms ticker and runs tickFn on each tick in a goroutine.
func (m *GameMonitor[S]) StartLoop(tickFn func()) {
	m.stopCh = make(chan struct{})
	m.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-m.ticker.C:
				tickFn()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop halts the tick loop and closes the display channel.
func (m *GameMonitor[S]) Stop() {
	m.stopOnce.Do(func() {
		if m.ticker != nil {
			m.ticker.Stop()
		}
		if m.stopCh != nil {
			close(m.stopCh)
		}
		close(m.displayCh)
	})
}

// PublishState sends state on both the typed and display channels.
// Uses non-blocking sends to avoid stalling the tick loop.
func (m *GameMonitor[S]) PublishState(state S) {
	display := state.ToDisplayUpdate()

	// Non-blocking send on typed channel (drop old if not consumed)
	select {
	case m.updates <- state:
	default:
		select {
		case <-m.updates:
		default:
		}
		m.updates <- state
	}

	// Non-blocking send on display channel (drop old if not consumed)
	select {
	case m.displayCh <- display:
	default:
		select {
		case <-m.displayCh:
		default:
		}
		m.displayCh <- display
	}
}
