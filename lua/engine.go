// Package lua provides the Lua scripting engine for coda.
//
// The engine loads mode and recipe scripts from ~/.config/coda/lua/,
// registers the "coda" Lua module with Go-backed functions, and evaluates
// gate functions that determine whether Claude Code should stop or continue.
package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	glua "github.com/yuin/gopher-lua"

	"github.com/s3nik/coda/observer"
)

// Engine manages the Lua VM, loaded modes, and gate evaluation.
type Engine struct {
	mu          sync.Mutex
	vm          *glua.LState
	activeMode  *Mode
	modes       map[string]*Mode
	observers   *observer.Manager
	testCommand string // default test command for gate evaluation
}

// NewEngine creates a Lua engine with the coda module registered.
func NewEngine(observers *observer.Manager) *Engine {
	e := &Engine{
		modes:       make(map[string]*Mode),
		observers:   observers,
		testCommand: "mix test",
	}
	e.initVM()
	return e
}

// initVM creates a fresh Lua VM with the coda module pre-loaded.
func (e *Engine) initVM() {
	e.vm = glua.NewState(glua.Options{
		SkipOpenLibs: false, // full stdlib available
	})
	e.vm.PreloadModule("coda", e.codaModuleLoader)
}

// Close shuts down the Lua VM.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.vm != nil {
		e.vm.Close()
	}
}

// LoadFile executes a Lua script file in the VM.
func (e *Engine) LoadFile(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.vm.DoFile(path)
}

// LoadDir loads all .lua files from a directory.
func (e *Engine) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no scripts yet
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lua" {
			continue
		}
		if err := e.LoadFile(filepath.Join(dir, entry.Name())); err != nil {
			return fmt.Errorf("load %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// SetMode activates a named mode. Returns an error if the mode doesn't exist.
func (e *Engine) SetMode(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	mode, ok := e.modes[name]
	if !ok {
		return fmt.Errorf("unknown mode: %q (available: %v)", name, e.modeNames())
	}

	// Stop observers from previous mode
	if e.activeMode != nil {
		e.stopModeObservers(e.activeMode)
	}

	e.activeMode = mode

	// Start observers for new mode
	for _, obs := range mode.Observers {
		switch obs.Type {
		case "tail":
			if err := e.observers.StartTail(obs.ID, obs.Command); err != nil {
				return fmt.Errorf("start tail %q: %w", obs.ID, err)
			}
		}
	}

	return nil
}

// ActiveMode returns the name of the active mode, or empty string.
func (e *Engine) ActiveMode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeMode == nil {
		return ""
	}
	return e.activeMode.Name
}

// ActiveSystemPrompt returns the system prompt of the active mode.
func (e *Engine) ActiveSystemPrompt() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeMode == nil {
		return ""
	}
	return e.activeMode.SystemPrompt
}

// EvaluateGate runs the active mode's gate function.
// Returns (pass bool, reason string).
func (e *Engine) EvaluateGate() (bool, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.activeMode == nil {
		return true, "no active mode — gate passes by default"
	}

	if e.activeMode.GateFn == nil {
		return true, "mode has no gate function"
	}

	// Call the Lua gate function
	if err := e.vm.CallByParam(glua.P{
		Fn:      e.activeMode.GateFn,
		NRet:    2,
		Protect: true,
	}); err != nil {
		return false, fmt.Sprintf("gate error: %v", err)
	}

	// Get return values: (bool, string)
	pass := e.vm.Get(-2)
	reason := e.vm.Get(-1)
	e.vm.Pop(2)

	passVal := pass == glua.LTrue
	reasonStr := "gate evaluated"
	if reason != glua.LNil {
		reasonStr = reason.String()
	}

	return passVal, reasonStr
}

// ListModes returns the names of all registered modes.
func (e *Engine) ListModes() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modeNames()
}

func (e *Engine) modeNames() []string {
	names := make([]string, 0, len(e.modes))
	for name := range e.modes {
		names = append(names, name)
	}
	return names
}

func (e *Engine) stopModeObservers(mode *Mode) {
	for _, obs := range mode.Observers {
		switch obs.Type {
		case "tail":
			e.observers.StopTail(obs.ID)
		}
	}
}

// registerMode is called from Lua via coda.define_mode().
func (e *Engine) registerMode(mode *Mode) {
	e.modes[mode.Name] = mode
}

// codaModuleLoader is the Lua module loader for require("coda").
func (e *Engine) codaModuleLoader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"define_mode":     e.luaDefineMode,
		"get_tail_output": e.luaGetTailOutput,
		"run":             e.luaRun,
	})
	L.Push(mod)
	return 1
}

// luaDefineMode handles coda.define_mode({...}) from Lua.
func (e *Engine) luaDefineMode(L *glua.LState) int {
	tbl := L.CheckTable(1)

	mode := &Mode{}

	// Extract name
	if name := tbl.RawGetString("name"); name != glua.LNil {
		mode.Name = name.String()
	}
	if mode.Name == "" {
		L.ArgError(1, "mode requires 'name' field")
		return 0
	}

	// Extract system_prompt
	if sp := tbl.RawGetString("system_prompt"); sp != glua.LNil {
		mode.SystemPrompt = sp.String()
	}

	// Extract tools list
	if tools := tbl.RawGetString("tools"); tools != glua.LNil {
		if toolsTbl, ok := tools.(*glua.LTable); ok {
			toolsTbl.ForEach(func(_, v glua.LValue) {
				mode.Tools = append(mode.Tools, v.String())
			})
		}
	}

	// Extract observers list
	if observers := tbl.RawGetString("observers"); observers != glua.LNil {
		if obsTbl, ok := observers.(*glua.LTable); ok {
			obsTbl.ForEach(func(_, v glua.LValue) {
				if vTbl, ok := v.(*glua.LTable); ok {
					obs := ObserverConfig{}
					if t := vTbl.RawGetString("type"); t != glua.LNil {
						obs.Type = t.String()
					}
					if cmd := vTbl.RawGetString("command"); cmd != glua.LNil {
						obs.Command = cmd.String()
					}
					if id := vTbl.RawGetString("id"); id != glua.LNil {
						obs.ID = id.String()
					}
					if pattern := vTbl.RawGetString("pattern"); pattern != glua.LNil {
						obs.Pattern = pattern.String()
					}
					mode.Observers = append(mode.Observers, obs)
				}
			})
		}
	}

	// Extract gate function
	if gate := tbl.RawGetString("gate"); gate != glua.LNil {
		if fn, ok := gate.(*glua.LFunction); ok {
			mode.GateFn = fn
		}
	}

	e.registerMode(mode)
	return 0
}

// luaGetTailOutput handles coda.get_tail_output(id) from Lua.
func (e *Engine) luaGetTailOutput(L *glua.LState) int {
	id := L.CheckString(1)
	lines, err := e.observers.ReadTail(id)
	if err != nil {
		L.Push(glua.LNil)
		return 1
	}
	// Join all lines into a single string
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	L.Push(glua.LString(result))
	return 1
}

// luaRun handles coda.run(command) from Lua — runs a shell command synchronously.
func (e *Engine) luaRun(L *glua.LState) int {
	command := L.CheckString(1)
	result := e.observers.RunCommand(command)

	// Return a table with exit_code and output
	tbl := L.NewTable()
	L.SetField(tbl, "exit_code", glua.LNumber(result.ExitCode))
	L.SetField(tbl, "output", glua.LString(result.Output))
	L.Push(tbl)
	return 1
}
