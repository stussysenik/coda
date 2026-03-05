package hooks

import (
	"encoding/json"
	"fmt"
	"os"
)

// PostToolUse drains pending observations and injects them into Claude's context.
//
// After every tool use, this hook:
// 1. Connects to coda serve
// 2. Drains all pending observations (server errors, file changes, etc.)
// 3. Prints them to stdout so Claude Code sees them
//
// This is how Claude "sees" server errors without explicitly asking —
// the observations appear in its context after every tool call.
func PostToolUse() error {
	// Read the hook input from stdin
	_, err := ReadHookInput()
	if err != nil {
		return nil // non-fatal
	}

	// Drain observations from coda serve
	resp, err := ipcCall("drain_observations", nil)
	if err != nil {
		// Silently skip if serve isn't running
		return nil
	}

	if !resp.OK {
		return nil // skip on errors
	}

	var obs ObservationsResult
	if err := json.Unmarshal(resp.Result, &obs); err != nil {
		return nil
	}

	// Print observations to stdout — Claude Code will see these
	for _, o := range obs.Observations {
		fmt.Fprintln(os.Stdout, o)
	}

	return nil
}
