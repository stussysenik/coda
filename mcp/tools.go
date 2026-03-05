package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s3nik/coda/observer"
)

// registerTools wires all coda MCP tools into the server.
func (s *Server) registerTools() {
	s.registerTailServer()
	s.registerReadTail()
	s.registerGetObservations()
	s.registerRunTests()
	s.registerCheckGate()
	s.registerSetMode()
	s.registerScreenshot()
	s.registerWorktreeSpawn()
	s.registerWorktreeStatus()
}

// --- tail_server: Start/stop/read a tailed process ---

type tailServerInput struct {
	Action  string `json:"action" jsonschema:"'start', 'stop', or 'read'. Start begins tailing a command's output, stop kills it, read returns recent lines."`
	Command string `json:"command,omitempty" jsonschema:"Shell command to tail (required for 'start')"`
	ID      string `json:"id,omitempty" jsonschema:"Identifier for this tail (defaults to 'default')"`
}

func (s *Server) registerTailServer() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "tail_server",
		Description: "Start, stop, or read a tailed process. Use 'start' with a command to begin capturing output, 'read' to get recent lines, 'stop' to kill it.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[tailServerInput]) (*mcp.CallToolResult, error) {
		input := params.Arguments
		id := input.ID
		if id == "" {
			id = "default"
		}

		switch input.Action {
		case "start":
			if input.Command == "" {
				return errResult("'command' is required for 'start'"), nil
			}
			err := s.observers.StartTail(id, input.Command)
			if err != nil {
				return errResult(fmt.Sprintf("failed to start tail: %v", err)), nil
			}
			return textResult(fmt.Sprintf("Tailing started: [%s] %s", id, input.Command)), nil

		case "stop":
			err := s.observers.StopTail(id)
			if err != nil {
				return errResult(fmt.Sprintf("failed to stop tail: %v", err)), nil
			}
			return textResult(fmt.Sprintf("Tail stopped: [%s]", id)), nil

		case "read":
			lines, err := s.observers.ReadTail(id)
			if err != nil {
				return errResult(fmt.Sprintf("failed to read tail: %v", err)), nil
			}
			if len(lines) == 0 {
				return textResult("[no output]"), nil
			}
			return textResult(strings.Join(lines, "\n")), nil

		default:
			return errResult("action must be 'start', 'stop', or 'read'"), nil
		}
	})
}

// --- read_tail: Get last N lines from a tailed process ---

type readTailInput struct {
	ID    string `json:"id,omitempty" jsonschema:"Tail identifier (defaults to 'default')"`
	Lines int    `json:"lines,omitempty" jsonschema:"Number of recent lines to return (defaults to 50)"`
}

func (s *Server) registerReadTail() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "read_tail",
		Description: "Get the last N lines from a tailed process. Defaults to 50 lines.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[readTailInput]) (*mcp.CallToolResult, error) {
		input := params.Arguments
		id := input.ID
		if id == "" {
			id = "default"
		}
		n := input.Lines
		if n <= 0 {
			n = 50
		}

		lines, err := s.observers.ReadTailN(id, n)
		if err != nil {
			return errResult(fmt.Sprintf("failed to read tail: %v", err)), nil
		}
		if len(lines) == 0 {
			return textResult("[no output]"), nil
		}
		return textResult(strings.Join(lines, "\n")), nil
	})
}

// --- get_observations: Drain all pending observations ---

type emptyInput struct{}

func (s *Server) registerGetObservations() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_observations",
		Description: "Drain all pending observations from all active observers. Returns any new output, file changes, or errors detected since last drain.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[emptyInput]) (*mcp.CallToolResult, error) {
		obs := s.observers.Drain()
		if len(obs) == 0 {
			return textResult("[no new observations]"), nil
		}
		return textResult(strings.Join(obs, "\n---\n")), nil
	})
}

// --- run_tests: Execute test command ---

type runTestsInput struct {
	Command string `json:"command,omitempty" jsonschema:"Test command to run (defaults to 'mix test')"`
}

func (s *Server) registerRunTests() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "run_tests",
		Description: "Execute a test command and return structured pass/fail results with output.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[runTestsInput]) (*mcp.CallToolResult, error) {
		cmd := params.Arguments.Command
		if cmd == "" {
			cmd = "mix test"
		}
		result := s.observers.RunCommand(cmd)
		status := "PASS"
		if result.ExitCode != 0 {
			status = "FAIL"
		}
		text := fmt.Sprintf("[%s] exit=%d\n\n%s", status, result.ExitCode, result.Output)
		return textResult(text), nil
	})
}

// --- check_gate: Evaluate the active mode's completion gate ---

func (s *Server) registerCheckGate() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "check_gate",
		Description: "Evaluate the active mode's completion gate. Returns whether Claude should stop (gate passes) or continue working (gate fails with reason).",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[emptyInput]) (*mcp.CallToolResult, error) {
		pass, reason := s.lua.EvaluateGate()
		status := "PASS"
		if !pass {
			status = "FAIL"
		}
		return textResult(fmt.Sprintf("[gate:%s] %s", status, reason)), nil
	})
}

// --- set_mode: Switch the active Lua mode ---

type setModeInput struct {
	Mode string `json:"mode" jsonschema:"Name of the mode to activate (e.g. 'dev')"`
}

func (s *Server) registerSetMode() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "set_mode",
		Description: "Switch the active Lua mode. Modes define which observers to run, system prompts, and completion gates.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[setModeInput]) (*mcp.CallToolResult, error) {
		if err := s.lua.SetMode(params.Arguments.Mode); err != nil {
			return errResult(fmt.Sprintf("failed to set mode: %v", err)), nil
		}
		prompt := s.lua.ActiveSystemPrompt()
		text := fmt.Sprintf("Mode activated: %s", params.Arguments.Mode)
		if prompt != "" {
			text += "\n\nSystem prompt:\n" + prompt
		}
		return textResult(text), nil
	})
}

// --- screenshot: CDP screenshot ---

type screenshotInput struct {
	URL string `json:"url,omitempty" jsonschema:"URL to screenshot (uses current page if omitted)"`
}

func (s *Server) registerScreenshot() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "screenshot",
		Description: "Take a CDP screenshot of a browser page. Returns base64-encoded image.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[screenshotInput]) (*mcp.CallToolResult, error) {
		url := params.Arguments.URL
		if url == "" {
			url = "http://localhost:4000"
		}
		sc := &observer.Screenshot{}
		data, err := sc.CaptureBytes(url)
		if err != nil {
			return errResult(fmt.Sprintf("screenshot failed: %v", err)), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ImageContent{
					Data:     data,
					MIMEType: "image/png",
				},
			},
		}, nil
	})
}

// --- worktree_spawn: Create worktree + launch parallel Claude session ---

type worktreeSpawnInput struct {
	Name   string `json:"name" jsonschema:"Name for the worktree and branch"`
	Prompt string `json:"prompt" jsonschema:"Task prompt for the parallel Claude session"`
}

func (s *Server) registerWorktreeSpawn() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "worktree_spawn",
		Description: "Create a git worktree and launch a parallel Claude session in it. The session works independently on the given task.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[worktreeSpawnInput]) (*mcp.CallToolResult, error) {
		if s.pool == nil {
			return errResult("not in a git repository — worktrees require git"), nil
		}
		input := params.Arguments
		session, err := s.pool.Spawn(input.Name, input.Prompt)
		if err != nil {
			return errResult(fmt.Sprintf("failed to spawn worktree: %v", err)), nil
		}
		return textResult(fmt.Sprintf("Spawned parallel session '%s' in %s\nPrompt: %s", session.Name, session.Worktree, session.Prompt)), nil
	})
}

// --- worktree_status: Check status of all parallel sessions ---

func (s *Server) registerWorktreeStatus() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "worktree_status",
		Description: "Check the status of all parallel worktree sessions.",
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[emptyInput]) (*mcp.CallToolResult, error) {
		if s.pool == nil {
			return errResult("not in a git repository — worktrees require git"), nil
		}
		infos := s.pool.Status()
		if len(infos) == 0 {
			return textResult("[worktree] no active sessions"), nil
		}
		var lines []string
		for _, info := range infos {
			lines = append(lines, info.FormatStatus())
		}
		return textResult(strings.Join(lines, "\n")), nil
	})
}

// --- helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func errResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
		IsError: true,
	}
}
