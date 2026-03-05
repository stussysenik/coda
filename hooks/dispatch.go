// Package hooks implements Claude Code hook handlers.
//
// Hooks are short-lived processes invoked by Claude Code at specific events.
// They communicate with the long-lived `coda serve` process over a Unix
// domain socket to query observer state and evaluate completion gates.
package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// HookInput is the JSON payload Claude Code sends to hook processes via stdin.
type HookInput struct {
	// SessionID is the Claude Code session identifier.
	SessionID string `json:"session_id,omitempty"`

	// TranscriptPath is the path to the conversation transcript.
	TranscriptPath string `json:"transcript_path,omitempty"`

	// ToolName is the tool that was just used (PostToolUse only).
	ToolName string `json:"tool_name,omitempty"`

	// ToolInput is the input that was sent to the tool (PostToolUse only).
	ToolInput json.RawMessage `json:"tool_input,omitempty"`

	// StopReason is why Claude decided to stop (Stop only).
	StopReason string `json:"stop_reason,omitempty"`
}

// ReadHookInput reads and parses the hook JSON payload from stdin.
func ReadHookInput() (*HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return &HookInput{}, nil
	}
	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parse hook input: %w", err)
	}
	return &input, nil
}
