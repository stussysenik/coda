package worktree

import (
	"fmt"
	"sync"
)

// Pool manages parallel Claude Code sessions across worktrees.
type Pool struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	repoRoot string
}

// NewPool creates a session pool for the given repository.
func NewPool(repoRoot string) *Pool {
	return &Pool{
		sessions: make(map[string]*Session),
		repoRoot: repoRoot,
	}
}

// Spawn creates a worktree and launches a parallel Claude session in it.
func (p *Pool) Spawn(name, prompt string) (*Session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if session already exists
	if existing, ok := p.sessions[name]; ok {
		if existing.Status == StatusRunning {
			return nil, fmt.Errorf("session %q is already running", name)
		}
		// Session completed/failed — clean up old worktree
		Remove(p.repoRoot, name)
	}

	// Create worktree
	wtPath, err := Create(p.repoRoot, name)
	if err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	// Create and start session
	session := NewSession(name, prompt, wtPath)
	if err := session.Start(); err != nil {
		Remove(p.repoRoot, name)
		return nil, fmt.Errorf("start session: %w", err)
	}

	p.sessions[name] = session
	return session, nil
}

// Status returns info for all sessions.
func (p *Pool) Status() []SessionInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(p.sessions))
	for _, session := range p.sessions {
		infos = append(infos, session.Info())
	}
	return infos
}

// Get returns info for a specific session.
func (p *Pool) Get(name string) (SessionInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session, ok := p.sessions[name]
	if !ok {
		return SessionInfo{}, false
	}
	return session.Info(), true
}

// WaitAll blocks until all sessions complete.
func (p *Pool) WaitAll() {
	p.mu.RLock()
	sessions := make([]*Session, 0, len(p.sessions))
	for _, s := range p.sessions {
		sessions = append(sessions, s)
	}
	p.mu.RUnlock()

	for _, s := range sessions {
		s.Wait()
	}
}

// Cleanup removes all worktrees for completed sessions.
func (p *Pool) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, session := range p.sessions {
		if session.Status != StatusRunning {
			Remove(p.repoRoot, name)
			delete(p.sessions, name)
		}
	}
}
