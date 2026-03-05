package observer

import (
	"bufio"
	"fmt"
	"os/exec"
	"sync"
)

// Tail captures the stdout/stderr of a long-running process into a ring buffer.
type Tail struct {
	id      string
	command string
	ring    *Ring
	cmd     *exec.Cmd
	cancel  func()
	done    chan struct{}
	mu      sync.Mutex
}

// NewTail creates a tail observer for the given shell command.
func NewTail(id, command string) *Tail {
	return &Tail{
		id:      id,
		command: command,
		ring:    NewRing(500), // keep last 500 lines
		done:    make(chan struct{}),
	}
}

// Start launches the tailed process.
func (t *Tail) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cmd = exec.Command("sh", "-c", t.command)

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	t.cmd.Stderr = t.cmd.Stdout // merge stderr into stdout

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	go func() {
		defer close(t.done)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			t.ring.Write(scanner.Text())
		}
		t.cmd.Wait()
	}()

	return nil
}

// Stop kills the tailed process.
func (t *Tail) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		<-t.done
	}
	return nil
}

// ReadAll returns all buffered lines.
func (t *Tail) ReadAll() []string {
	return t.ring.ReadAll()
}

// ReadN returns the last n buffered lines.
func (t *Tail) ReadN(n int) []string {
	return t.ring.ReadN(n)
}

// Drain returns all buffered lines and clears the buffer.
func (t *Tail) Drain() []string {
	return t.ring.Drain()
}

// ID returns the tail identifier.
func (t *Tail) ID() string {
	return t.id
}
