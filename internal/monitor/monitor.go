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
	ErrGameDetached     = errors.New("game detached")
	ErrSaveNotSupported = errors.New("save detection not supported")
	ErrSavePending      = errors.New("save detection pending")
	ErrSaveChanged           = errors.New("save identity changed")
	ErrAttachedGameMismatch = errors.New("attached game mismatch")
)

// Monitor is the interface tray.App uses to drive the monitoring lifecycle.
type Monitor interface {
	Start()
	Stop()
	DisplayUpdates() <-chan DisplayUpdate
}

// TickMonitor is implemented by sub-monitors for the tick loop.
type TickMonitor interface {
	Tick(reader *memreader.GameReader) error
	OnAttach(gameID string) error
	OnDetach()
}

// DisplayUpdate is the common display state consumed by tray.
type DisplayUpdate struct {
	GameName      string
	Status        string
	DeathCount    uint32
	CharacterName string
	SaveSlotIndex int
	Route         *RouteDisplay // nil when no route is active (e.g. DeathCounterMonitor)
}

// Displayable is the constraint for State types — they must know
// how to convert themselves into a DisplayUpdate for the tray.
type Displayable interface {
	ToDisplayUpdate() DisplayUpdate
}

// GameMonitor is the generic base that manages the game attach/detach
// lifecycle, death count reading, and pushes typed state updates.
type GameMonitor[S Displayable] struct {
	ops     memreader.ProcessOps
	gameID  string // target game (immutable, set at construction)
	Reader  *memreader.GameReader
	Tracker *stats.Tracker

	updates   chan S
	displayCh chan DisplayUpdate

	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once

	LastCount      uint32
	Phase          MonitorPhase
	attachedGameID string

	// Save slot tracking
	CurrentSaveID   int64
	CurrentSlotIdx  int
	CurrentCharName string

	// Log spam prevention (reset on attach/detach)
	saveLoggedOnce bool
	loadLoggedOnce bool
}

// NewGameMonitor initializes a GameMonitor with buffered channels.
func NewGameMonitor[S Displayable](gameID string, ops memreader.ProcessOps, tracker *stats.Tracker) GameMonitor[S] {
	return GameMonitor[S]{
		ops:       ops,
		gameID:    gameID,
		Tracker:   tracker,
		updates:   make(chan S, 1),
		displayCh: make(chan DisplayUpdate, 1),
		Phase:     PhaseDetached,
	}
}

// DisplayUpdates returns the common display channel for tray.
func (m *GameMonitor[S]) DisplayUpdates() <-chan DisplayUpdate {
	return m.displayCh
}

// Updates returns the typed state channel.
func (m *GameMonitor[S]) Updates() <-chan S {
	return m.updates
}

// GameID returns the target game ID.
func (m *GameMonitor[S]) GameID() string {
	return m.gameID
}

// GameLabel returns the display label for the target game.
func (m *GameMonitor[S]) GameLabel() string {
	return memreader.GetGameLabel(m.gameID)
}

// StatusText returns the current status string derived from the Phase.
func (m *GameMonitor[S]) StatusText() string {
	return m.Phase.StatusText()
}

// Attach attempts to attach to the target game process.
// Returns the GameReader and nil on success, or ErrNoGame if the process is not running.
// If already attached, returns the existing reader.
func (m *GameMonitor[S]) Attach() (*memreader.GameReader, error) {
	if m.Reader != nil {
		return m.Reader, nil
	}
	cfg, proc, err := memreader.FindGame(m.ops, m.gameID)
	if err != nil {
		if m.Phase > PhaseDetached {
			m.Detach()
			return nil, ErrGameDetached
		}
		return nil, ErrNoGame
	}
	m.Reader = memreader.NewGameReader(m.ops, cfg, proc)
	m.attachedGameID = cfg.ID
	log.Printf("Attached to: %s (%s)", cfg.Label, cfg.ID)
	m.Phase = PhaseAttached
	m.LastCount = 0
	m.CurrentSaveID = 0
	m.CurrentSlotIdx = 0
	m.CurrentCharName = ""
	m.saveLoggedOnce = false
	m.loadLoggedOnce = false
	return m.Reader, nil
}

// Detach closes the reader and resets phase to Detached.
func (m *GameMonitor[S]) Detach() {
	if m.Reader != nil {
		log.Printf("[%s] Detached", m.gameID)
		m.Reader.Detach()
		m.Reader = nil
	}
	m.Phase = PhaseDetached
	m.LastCount = 0
}

// DetectSave attempts to read the save slot identity from game memory.
// Returns the save ID and nil on success (same save or first detection),
// ErrSaveNotSupported if the game doesn't support save detection,
// ErrSavePending for transient failures, ErrSaveChanged when identity changes,
// or a wrapped error for DB failures.
func (m *GameMonitor[S]) DetectSave(reader *memreader.GameReader) (int64, error) {
	charName, nameErr := reader.ReadCharacterName()
	slotIdx, slotErr := reader.ReadSaveSlotIndex()

	// If character name is not supported, skip save detection entirely
	if isUnsupportedErr(nameErr) {
		return 0, ErrSaveNotSupported
	}

	// Null pointer or read error means game data is not yet loaded — retry later
	if nameErr != nil {
		if !m.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: %v", m.gameID, nameErr)
			m.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Empty name means the structure exists but save data isn't populated yet
	if charName == "" {
		if !m.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: empty character name", m.gameID)
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
			log.Printf("[%s] Save detection pending: uninitialized slot (255)", m.gameID)
			m.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Check if save identity changed
	if slotIdx != m.CurrentSlotIdx || charName != m.CurrentCharName {
		saveID, err := m.Tracker.FindOrCreateSave(m.gameID, slotIdx, charName)
		if err != nil {
			log.Printf("[%s] Failed to create save record: %v", m.gameID, err)
			return 0, fmt.Errorf("failed to create save record: %w", err)
		}

		previouslyLoaded := m.Phase >= PhaseLoaded
		m.CurrentSaveID = saveID
		m.CurrentSlotIdx = slotIdx
		m.CurrentCharName = charName

		if !previouslyLoaded {
			log.Printf("[%s] Save detected: %s (Slot %d)", m.gameID, charName, slotIdx)
			log.Printf("[%s] Game loaded successfully", m.gameID)
		} else {
			log.Printf("[%s] Save changed: %s (Slot %d)", m.gameID, charName, slotIdx)
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
		log.Printf("[%s] Death count: %d (previous: %d)", m.gameID, count, m.LastCount)
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

// StartLoop creates a 500ms ticker and runs the TickMonitor handler on each tick.
// It manages the PhaseDetached→PhaseAttached→PhaseLoaded transitions.
// Tick is only called when Phase >= PhaseLoaded.
func (m *GameMonitor[S]) StartLoop(handler TickMonitor) {
	m.stopCh = make(chan struct{})
	m.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-m.ticker.C:
				reader, err := m.Attach()
				if errors.Is(err, ErrGameDetached) {
					handler.OnDetach()
					continue
				}
				if err != nil {
					continue // ErrNoGame — never attached, just wait
				}

				// PhaseAttached → PhaseLoaded (via OnAttach)
				if m.Phase == PhaseAttached {
					if err := handler.OnAttach(m.attachedGameID); err != nil {
						log.Printf("[%s] OnAttach error: %v", m.gameID, err)
						m.Detach()
						handler.OnDetach()
						continue
					}
					m.Phase = PhaseLoaded
					continue
				}

				// PhaseLoaded+: call Tick
				if err := handler.Tick(reader); err != nil {
					if errors.Is(err, memreader.ErrGameRead) {
						m.Detach()
						handler.OnDetach()
					}
					log.Printf("[%s] Tick error: %v", m.gameID, err)
				}
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
