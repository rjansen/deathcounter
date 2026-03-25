package monitor

import "testing"

func TestMonitorPhase_String(t *testing.T) {
	tests := []struct {
		phase MonitorPhase
		want  string
	}{
		{PhaseDetached, "Detached"},
		{PhaseAttached, "Attached"},
		{PhaseLoaded, "Loaded"},
	}
	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("MonitorPhase(%d).String() = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestMonitorPhase_StatusText(t *testing.T) {
	tests := []struct {
		phase MonitorPhase
		want  string
	}{
		{PhaseDetached, "Waiting for game..."},
		{PhaseAttached, "Attached"},
		{PhaseLoaded, "Loaded"},
	}
	for _, tt := range tests {
		if got := tt.phase.StatusText(); got != tt.want {
			t.Errorf("MonitorPhase(%d).StatusText() = %q, want %q", tt.phase, got, tt.want)
		}
	}
}
