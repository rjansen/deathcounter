package monitor

import (
	"errors"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// Monitor is the interface tray.App uses to drive the monitoring lifecycle.
type Monitor interface {
	Tick()
	// DisplayUpdates returns a channel that receives display-ready state
	// each time Tick produces new information.
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

	LastCount      uint32
	LastGame       string
	WaitingForLoad bool
	Status         string

	// Save slot tracking
	CurrentSaveID   int64
	CurrentSlotIdx  int
	CurrentCharName string
	SaveDetected    bool
}

// InitGameMonitor initializes a GameMonitor with buffered channels.
func InitGameMonitor[S Displayable](reader *memreader.GameReader, tracker *stats.Tracker) GameMonitor[S] {
	return GameMonitor[S]{
		Reader:    reader,
		Tracker:   tracker,
		updates:   make(chan S, 1),
		displayCh: make(chan DisplayUpdate, 1),
		Status:    "Waiting for game...",
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

// GameName returns the current game name.
func (m *GameMonitor[S]) GameName() string {
	return m.LastGame
}

// StatusText returns the current status string.
func (m *GameMonitor[S]) StatusText() string {
	return m.Status
}

// IsAttached returns whether a game is currently attached.
func (m *GameMonitor[S]) IsAttached() bool {
	return m.Reader.IsAttached()
}

// TryAttach attempts to attach to a game process, detects game changes,
// and updates status. Returns true if the game changed.
func (m *GameMonitor[S]) TryAttach() (gameChanged bool) {
	if !m.Reader.IsAttached() {
		if err := m.Reader.Attach(); err != nil {
			if m.LastGame != "" {
				log.Printf("[%s] Game process ended", m.LastGame)
				m.Status = "Waiting for game..."
				m.LastGame = ""
				m.LastCount = 0
				m.WaitingForLoad = false
			}
			return false
		}
	}

	currentGame := m.Reader.GetCurrentGame()
	if currentGame != m.LastGame {
		log.Printf("Attached to: %s", currentGame)
		m.Status = "Connected"
		m.LastGame = currentGame
		m.LastCount = 0
		m.WaitingForLoad = false
		m.CurrentSaveID = 0
		m.CurrentSlotIdx = 0
		m.CurrentCharName = ""
		m.SaveDetected = false
		return true
	}
	return false
}

// ReadDeathCount reads the death count, handling transient and fatal errors.
// Returns the count and true on success, or 0 and false on failure.
func (m *GameMonitor[S]) ReadDeathCount() (uint32, bool) {
	count, err := m.Reader.ReadDeathCount()
	if err != nil {
		if errors.Is(err, memreader.ErrNullPointer) {
			if !m.WaitingForLoad {
				log.Printf("[%s] Waiting for game to fully load...", m.Reader.GetCurrentGame())
				m.Status = "Loading..."
				m.WaitingForLoad = true
			}
			return 0, false
		}

		// Fatal error: process likely gone, detach
		log.Printf("[%s] Disconnected: %v", m.Reader.GetCurrentGame(), err)
		m.Reader.Detach()
		m.Status = "Disconnected"
		m.LastGame = ""
		m.WaitingForLoad = false
		return 0, false
	}

	if m.WaitingForLoad {
		log.Printf("[%s] Game loaded successfully", m.Reader.GetCurrentGame())
		m.WaitingForLoad = false
		m.Status = "Connected"
	}

	return count, true
}

// TryDetectSave attempts to read the save slot identity from game memory.
// Returns (changed, ok): changed is true if the save identity changed from a
// previously detected save; ok is true if save detection succeeded or is not
// supported (transparent pass-through for non-DS3 games).
// This method never detaches the reader — callers should treat failures as
// transient and keep ticking.
func (m *GameMonitor[S]) TryDetectSave() (changed bool, ok bool) {
	charName, nameErr := m.Reader.ReadCharacterName()
	slotIdx, slotErr := m.Reader.ReadSaveSlotIndex()

	// If character name is not supported, skip save detection entirely
	// (save slot alone is not enough to identify a character)
	if isUnsupportedErr(nameErr) {
		m.SaveDetected = true
		return false, true
	}

	// Null pointer or read error means game data is not yet loaded — retry later
	if nameErr != nil {
		if !m.SaveDetected {
			log.Printf("[%s] Save detection pending: %v", m.Reader.GetCurrentGame(), nameErr)
		}
		return false, false
	}

	// Empty name means the structure exists but save data isn't populated yet
	if charName == "" {
		if !m.SaveDetected {
			log.Printf("[%s] Save detection pending: empty character name", m.Reader.GetCurrentGame())
		}
		return false, false
	}

	// Save slot is optional — use 0 if unavailable
	if slotErr != nil {
		slotIdx = 0
	}

	// Check if save identity changed
	previouslyHadSave := m.SaveDetected
	if slotIdx != m.CurrentSlotIdx || charName != m.CurrentCharName {
		saveID, err := m.Tracker.FindOrCreateSave(m.Reader.GetCurrentGame(), slotIdx, charName)
		if err != nil {
			log.Printf("[%s] Failed to create save record: %v", m.Reader.GetCurrentGame(), err)
			return false, false
		}
		m.CurrentSaveID = saveID
		m.CurrentSlotIdx = slotIdx
		m.CurrentCharName = charName
		m.SaveDetected = true
		log.Printf("[%s] Save detected: %s (Slot %d)", m.Reader.GetCurrentGame(), charName, slotIdx)
		return previouslyHadSave, true
	}

	return false, true
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
