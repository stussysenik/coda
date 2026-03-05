package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/s3nik/coda/config"
	"github.com/s3nik/coda/hooks"
)

// Install performs one-command project setup:
// 1. Register coda as an MCP server with Claude Code
// 2. Patch .claude/settings.json with hooks
// 3. Install /mode skill
// 4. Copy default Lua scripts to user config
func Install() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Find the coda binary path
	codaPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find coda binary: %w", err)
	}

	fmt.Println("coda install — setting up Claude Code integration")
	fmt.Println()

	// Step 1: Register as MCP server
	fmt.Print("1. Registering MCP server... ")
	if err := registerMCPServer(codaPath); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	// Step 2: Install hooks
	fmt.Print("2. Installing hooks... ")
	if err := hooks.Install(cwd); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	// Step 3: Install /mode skill
	fmt.Print("3. Installing /mode skill... ")
	if err := installModeSkill(cwd); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	// Step 4: Copy default Lua scripts
	fmt.Print("4. Copying default Lua scripts... ")
	if err := copyDefaultScripts(); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	fmt.Println()
	fmt.Println("Done! Start Claude Code and use /mode dev to activate dev mode.")
	return nil
}

func registerMCPServer(codaPath string) error {
	cmd := exec.Command("claude", "mcp", "add",
		"--transport", "stdio",
		"--scope", "project",
		"coda", "--",
		codaPath, "serve",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func installModeSkill(projectDir string) error {
	skillDir := filepath.Join(projectDir, ".claude", "skills", "mode")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	content := `# /mode — Switch coda operating mode

When the user uses /mode, use the set_mode MCP tool to switch the active coda mode.

## Usage
- ` + "`/mode dev`" + ` — Activate dev mode (tail server, run tests, gate checks)
- ` + "`/mode list`" + ` — List available modes

## What modes do
Modes configure:
- **Observers**: What processes to tail, what files to watch
- **System prompt**: Instructions injected into your context
- **Completion gate**: Checks that must pass before you can stop

When a mode is active, the Stop hook evaluates the gate function.
If the gate fails (tests failing, server errors), you'll be asked to continue fixing.
`

	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)
}

func copyDefaultScripts() error {
	modesDir, err := config.ModesDir()
	if err != nil {
		return err
	}

	// Find bundled lua.d directory
	var srcDir string

	// Try next to binary first
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "lua.d", "modes")
		if _, err := os.Stat(candidate); err == nil {
			srcDir = candidate
		}
	}

	// Fall back to source tree
	if srcDir == "" {
		if _, file, _, ok := runtime.Caller(0); ok {
			candidate := filepath.Join(filepath.Dir(filepath.Dir(file)), "lua.d", "modes")
			if _, err := os.Stat(candidate); err == nil {
				srcDir = candidate
			}
		}
	}

	if srcDir == "" {
		return fmt.Errorf("could not find bundled lua.d directory")
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(modesDir, entry.Name())

		// Don't overwrite existing user scripts
		if _, err := os.Stat(dst); err == nil {
			continue
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
	}

	return nil
}
