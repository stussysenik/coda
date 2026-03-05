package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HookConfig represents a single hook entry in .claude/settings.json.
type HookConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// HookEventConfig is the config for a single hook event.
type HookEventConfig struct {
	Hooks []HookConfig `json:"hooks"`
}

// Install patches .claude/settings.json with the Stop and PostToolUse hooks.
func Install(projectDir string) error {
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Read existing settings or start fresh
	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	} else {
		settings = make(map[string]any)
	}

	// Build hooks config
	hooks := map[string][]HookEventConfig{
		"Stop": {
			{Hooks: []HookConfig{{Type: "command", Command: "coda hook Stop"}}},
		},
		"PostToolUse": {
			{Hooks: []HookConfig{{Type: "command", Command: "coda hook PostToolUse"}}},
		},
	}

	settings["hooks"] = hooks

	// Write back
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}

	fmt.Printf("Installed hooks in %s\n", settingsPath)
	return nil
}
