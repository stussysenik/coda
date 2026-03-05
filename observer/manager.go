package observer

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"
)

// CommandResult holds the output and exit code of a command run.
type CommandResult struct {
	Output   string
	ExitCode int
}

// Manager coordinates all active observers (tails, file watchers, etc.).
type Manager struct {
	mu    sync.RWMutex
	tails map[string]*Tail
	// pending observations waiting to be drained
	pendingMu sync.Mutex
	pending   []string
}

// NewManager creates an observer manager.
func NewManager() *Manager {
	return &Manager{
		tails: make(map[string]*Tail),
	}
}

// StartTail starts a new process tail observer.
func (m *Manager) StartTail(id, command string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing tail with same ID
	if existing, ok := m.tails[id]; ok {
		existing.Stop()
	}

	tail := NewTail(id, command)
	if err := tail.Start(); err != nil {
		return err
	}
	m.tails[id] = tail
	return nil
}

// StopTail stops a running tail observer.
func (m *Manager) StopTail(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tail, ok := m.tails[id]
	if !ok {
		return fmt.Errorf("no tail with id %q", id)
	}
	err := tail.Stop()
	delete(m.tails, id)
	return err
}

// ReadTail returns all buffered lines from a tail.
func (m *Manager) ReadTail(id string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tail, ok := m.tails[id]
	if !ok {
		return nil, fmt.Errorf("no tail with id %q", id)
	}
	return tail.ReadAll(), nil
}

// ReadTailN returns the last n lines from a tail.
func (m *Manager) ReadTailN(id string, n int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tail, ok := m.tails[id]
	if !ok {
		return nil, fmt.Errorf("no tail with id %q", id)
	}
	return tail.ReadN(n), nil
}

// Drain returns and clears all pending observations across all observers.
func (m *Manager) Drain() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []string

	// Drain pending observations
	m.pendingMu.Lock()
	if len(m.pending) > 0 {
		all = append(all, m.pending...)
		m.pending = nil
	}
	m.pendingMu.Unlock()

	// Drain tail buffers
	for id, tail := range m.tails {
		lines := tail.Drain()
		if len(lines) > 0 {
			header := fmt.Sprintf("[coda/%s]", id)
			for _, line := range lines {
				all = append(all, header+" "+line)
			}
		}
	}

	return all
}

// AddObservation adds a pending observation to be drained later.
func (m *Manager) AddObservation(obs string) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	m.pending = append(m.pending, obs)
}

// RunCommand executes a shell command synchronously and returns the result.
func (m *Manager) RunCommand(command string) CommandResult {
	cmd := exec.Command("sh", "-c", command)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return CommandResult{
		Output:   buf.String(),
		ExitCode: exitCode,
	}
}

// StopAll stops all active observers.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, tail := range m.tails {
		tail.Stop()
		delete(m.tails, id)
	}
}

// ListTails returns the IDs of all active tails.
func (m *Manager) ListTails() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.tails))
	for id := range m.tails {
		ids = append(ids, id)
	}
	return ids
}
