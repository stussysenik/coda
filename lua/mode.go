package lua

import glua "github.com/yuin/gopher-lua"

// Mode defines a coda operating mode — a bundle of system prompt,
// enabled tools, observers, and a completion gate.
//
// Modes are defined in Lua scripts via coda.define_mode({...}).
type Mode struct {
	Name         string
	SystemPrompt string
	Tools        []string
	Observers    []ObserverConfig
	GateFn       *glua.LFunction // Lua function: () -> (bool, string)
}

// ObserverConfig describes an observer to start when a mode is activated.
type ObserverConfig struct {
	Type    string // "tail" or "filewatch"
	Command string // for tail observers
	Pattern string // for filewatch observers
	ID      string // unique identifier
}
