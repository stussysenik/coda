package cmd

import (
	"fmt"

	"github.com/s3nik/coda/hooks"
)

// Hook handles Claude Code hook events (Stop, PostToolUse).
// The hook process is short-lived — it connects to the running `coda serve`
// process via Unix socket to query state, then exits with the appropriate code.
func Hook(event string) error {
	switch event {
	case "Stop":
		return hooks.Stop()
	case "PostToolUse":
		return hooks.PostToolUse()
	default:
		return fmt.Errorf("unknown hook event: %s (expected Stop or PostToolUse)", event)
	}
}
