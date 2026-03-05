package observer

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileWatch monitors files matching a glob pattern and records changes.
type FileWatch struct {
	id      string
	pattern string
	ring    *Ring
	watcher *fsnotify.Watcher
	done    chan struct{}
	mu      sync.Mutex
}

// NewFileWatch creates a file watcher that monitors paths matching the glob pattern.
func NewFileWatch(id, pattern string) *FileWatch {
	return &FileWatch{
		id:      id,
		pattern: pattern,
		ring:    NewRing(100),
		done:    make(chan struct{}),
	}
}

// Start begins watching for file changes.
func (fw *FileWatch) Start() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	fw.watcher = watcher

	// Resolve the glob to find directories to watch
	matches, err := filepath.Glob(fw.pattern)
	if err != nil {
		watcher.Close()
		return fmt.Errorf("glob %q: %w", fw.pattern, err)
	}

	// Watch each matched path's directory
	dirs := make(map[string]bool)
	for _, match := range matches {
		dir := filepath.Dir(match)
		if !dirs[dir] {
			dirs[dir] = true
			if err := watcher.Add(dir); err != nil {
				// Non-fatal: log and continue
				fw.ring.Write(fmt.Sprintf("[warn] cannot watch %s: %v", dir, err))
			}
		}
	}

	go func() {
		defer close(fw.done)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Only report events for files matching our pattern
				if matched, _ := filepath.Match(fw.pattern, event.Name); matched {
					fw.ring.Write(fmt.Sprintf("[%s] %s %s", fw.id, event.Op, event.Name))
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fw.ring.Write(fmt.Sprintf("[%s] watch error: %v", fw.id, err))
			}
		}
	}()

	return nil
}

// Stop closes the file watcher.
func (fw *FileWatch) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.watcher != nil {
		fw.watcher.Close()
		<-fw.done
	}
	return nil
}

// Drain returns all recorded file events and clears the buffer.
func (fw *FileWatch) Drain() []string {
	return fw.ring.Drain()
}

// ReadAll returns all recorded file events.
func (fw *FileWatch) ReadAll() []string {
	return fw.ring.ReadAll()
}

// ID returns the watcher identifier.
func (fw *FileWatch) ID() string {
	return fw.id
}
