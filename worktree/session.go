package worktree

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SessionStatus tracks the state of a parallel Claude Code session.
type SessionStatus string

const (
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
)

// Session represents a parallel Claude Code session running in a worktree.
type Session struct {
	Name      string
	Prompt    string
	Worktree  string // path to the worktree
	Status    SessionStatus
	StartedAt time.Time
	EndedAt   time.Time
	Output    string // captured output
	ExitCode  int

	cmd  *exec.Cmd
	mu   sync.Mutex
	done chan struct{}
}

// NewSession creates a new session (does not start it).
func NewSession(name, prompt, worktreePath string) *Session {
	return &Session{
		Name:     name,
		Prompt:   prompt,
		Worktree: worktreePath,
		Status:   StatusRunning,
		done:     make(chan struct{}),
	}
}

// Start launches `claude -p` in the worktree directory.
func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.StartedAt = time.Now()

	// Launch claude in print mode with the prompt
	s.cmd = exec.Command("claude", "-p", s.Prompt)
	s.cmd.Dir = s.Worktree

	go func() {
		defer close(s.done)

		out, err := s.cmd.CombinedOutput()

		s.mu.Lock()
		defer s.mu.Unlock()

		s.Output = string(out)
		s.EndedAt = time.Now()

		if err != nil {
			s.Status = StatusFailed
			if exitErr, ok := err.(*exec.ExitError); ok {
				s.ExitCode = exitErr.ExitCode()
			} else {
				s.ExitCode = 1
			}
		} else {
			s.Status = StatusCompleted
			s.ExitCode = 0
		}
	}()

	return nil
}

// Wait blocks until the session completes.
func (s *Session) Wait() {
	<-s.done
}

// Info returns a snapshot of the session state.
func (s *Session) Info() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := SessionInfo{
		Name:      s.Name,
		Prompt:    s.Prompt,
		Worktree:  s.Worktree,
		Status:    string(s.Status),
		StartedAt: s.StartedAt.Format(time.RFC3339),
	}
	if !s.EndedAt.IsZero() {
		info.EndedAt = s.EndedAt.Format(time.RFC3339)
		info.Duration = s.EndedAt.Sub(s.StartedAt).String()
		info.ExitCode = s.ExitCode
		// Truncate output for status display
		if len(s.Output) > 500 {
			info.Output = "..." + s.Output[len(s.Output)-500:]
		} else {
			info.Output = s.Output
		}
	}
	return info
}

// SessionInfo is a serializable snapshot of session state.
type SessionInfo struct {
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	Worktree  string `json:"worktree"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at,omitempty"`
	Duration  string `json:"duration,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Output    string `json:"output,omitempty"`
}

// FormatStatus returns a human-readable status line.
func (si SessionInfo) FormatStatus() string {
	status := fmt.Sprintf("[%s] %s — %s", si.Status, si.Name, si.Prompt)
	if si.Duration != "" {
		status += fmt.Sprintf(" (%s)", si.Duration)
	}
	return status
}

// FormatResult returns a human-readable result string.
func (si SessionInfo) FormatResult() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s ===\n", si.Name)
	fmt.Fprintf(&b, "Status: %s\n", si.Status)
	fmt.Fprintf(&b, "Prompt: %s\n", si.Prompt)
	if si.Duration != "" {
		fmt.Fprintf(&b, "Duration: %s\n", si.Duration)
	}
	if si.Output != "" {
		fmt.Fprintf(&b, "\nOutput:\n%s\n", si.Output)
	}
	return b.String()
}
