package witness

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/util"
)

// Common errors
var (
	ErrNotRunning     = errors.New("witness not running")
	ErrAlreadyRunning = errors.New("witness already running")
)

// Manager handles witness lifecycle and monitoring operations.
type Manager struct {
	rig     *rig.Rig
	workDir string
}

// NewManager creates a new witness manager for a rig.
func NewManager(r *rig.Rig) *Manager {
	return &Manager{
		rig:     r,
		workDir: r.Path,
	}
}

// stateFile returns the path to the witness state file.
func (m *Manager) stateFile() string {
	return filepath.Join(m.rig.Path, ".runtime", "witness.json")
}

// loadState loads witness state from disk.
func (m *Manager) loadState() (*Witness, error) {
	data, err := os.ReadFile(m.stateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return &Witness{
				RigName: m.rig.Name,
				State:   StateStopped,
			}, nil
		}
		return nil, err
	}

	var w Witness
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}

	return &w, nil
}

// saveState persists witness state to disk using atomic write.
func (m *Manager) saveState(w *Witness) error {
	dir := filepath.Dir(m.stateFile())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return util.AtomicWriteJSON(m.stateFile(), w)
}

// Status returns the current witness status.
// ZFC-compliant: trusts agent-reported state, no PID inference.
// The daemon reads agent bead state for liveness checks.
func (m *Manager) Status() (*Witness, error) {
	w, err := m.loadState()
	if err != nil {
		return nil, err
	}

	// Update monitored polecats list (still useful for display)
	w.MonitoredPolecats = m.rig.Polecats

	return w, nil
}

// Start starts the witness (marks it as running).
// Patrol logic is now handled by mol-witness-patrol molecule executed by Claude.
func (m *Manager) Start() error {
	w, err := m.loadState()
	if err != nil {
		return err
	}

	if w.State == StateRunning && w.PID > 0 && util.ProcessExists(w.PID) {
		return ErrAlreadyRunning
	}

	now := time.Now()
	w.State = StateRunning
	w.StartedAt = &now
	w.PID = os.Getpid()
	w.MonitoredPolecats = m.rig.Polecats

	return m.saveState(w)
}

// Stop stops the witness.
func (m *Manager) Stop() error {
	w, err := m.loadState()
	if err != nil {
		return err
	}

	if w.State != StateRunning {
		return ErrNotRunning
	}

	// If we have a PID, try to stop it gracefully
	if w.PID > 0 && w.PID != os.Getpid() {
		// Send SIGTERM (best-effort graceful stop)
		if proc, err := os.FindProcess(w.PID); err == nil {
			_ = proc.Signal(os.Interrupt)
		}
	}

	w.State = StateStopped
	w.PID = 0

	return m.saveState(w)
}

