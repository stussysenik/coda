package hooks

import (
	"encoding/json"
	"fmt"
	"os"
)

// Stop evaluates the active mode's completion gate.
//
// This is the critical hook that closes the feedback loop:
// - Gate passes (exit 0): Claude Code stops normally
// - Gate fails (exit 2): Claude Code continues working with reason injected
//
// Exit code 2 is the magic number: Claude Code interprets it as
// "the hook blocked the stop, here's why" and feeds stderr back
// to the model so it can self-correct.
func Stop() error {
	// Read the hook input from stdin
	_, err := ReadHookInput()
	if err != nil {
		// If we can't read input, let Claude stop (don't block on errors)
		return nil
	}

	// Call the coda serve process to evaluate the gate
	resp, err := ipcCall("evaluate_gate", nil)
	if err != nil {
		// If we can't reach coda serve, let Claude stop
		// (don't block on infrastructure issues)
		fmt.Fprintf(os.Stderr, "[coda] warning: could not reach coda serve: %v\n", err)
		return nil
	}

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "[coda] gate error: %s\n", resp.Error)
		return nil // let Claude stop on errors
	}

	// Parse the gate result
	var gate GateResult
	if err := json.Unmarshal(resp.Result, &gate); err != nil {
		fmt.Fprintf(os.Stderr, "[coda] warning: could not parse gate result: %v\n", err)
		return nil
	}

	if gate.Pass {
		// Gate passes — Claude stops normally
		return nil
	}

	// Gate fails — block the stop!
	// Write reason to stderr so Claude Code sees it and continues working.
	fmt.Fprintf(os.Stderr, "[coda/gate] %s\n", gate.Reason)
	os.Exit(2)
	return nil // unreachable, but satisfies compiler
}
