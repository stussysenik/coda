package cmd

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/s3nik/coda/config"
	"github.com/s3nik/coda/hooks"
	codamcp "github.com/s3nik/coda/mcp"
)

// Serve runs coda as an MCP server over stdio.
// It also starts a Unix domain socket for IPC with hook processes.
func Serve() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	server := codamcp.New()

	// Load Lua scripts from default location and user config
	loadLuaScripts(server)

	// Start IPC socket for hook processes
	sockPath, err := config.SocketPath()
	if err != nil {
		log.Printf("warning: could not determine socket path: %v", err)
	} else {
		os.Remove(sockPath) // clean up stale socket
		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			log.Printf("warning: could not start IPC socket: %v", err)
		} else {
			defer listener.Close()
			defer os.Remove(sockPath)
			go serveIPC(ctx, listener, server)
		}
	}

	return server.Run(ctx)
}

// loadLuaScripts loads mode and recipe scripts from bundled and user directories.
func loadLuaScripts(server *codamcp.Server) {
	engine := server.Lua()

	// Load bundled scripts (next to the binary)
	if execPath, err := os.Executable(); err == nil {
		bundledDir := filepath.Join(filepath.Dir(execPath), "lua.d", "modes")
		if err := engine.LoadDir(bundledDir); err != nil {
			log.Printf("warning: loading bundled modes: %v", err)
		}
	}

	// Also try loading from source tree (for development)
	if _, file, _, ok := runtime.Caller(0); ok {
		srcDir := filepath.Join(filepath.Dir(filepath.Dir(file)), "lua.d", "modes")
		if err := engine.LoadDir(srcDir); err != nil {
			log.Printf("warning: loading source modes: %v", err)
		}
	}

	// Load user scripts from ~/.config/coda/lua/modes/
	if modesDir, err := config.ModesDir(); err == nil {
		if err := engine.LoadDir(modesDir); err != nil {
			log.Printf("warning: loading user modes: %v", err)
		}
	}
}

// serveIPC handles connections from `coda hook` processes over Unix socket.
func serveIPC(ctx context.Context, listener net.Listener, server *codamcp.Server) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("ipc accept error: %v", err)
				continue
			}
		}
		go handleIPCConn(ctx, conn, server)
	}
}

// handleIPCConn processes a single IPC request from a hook process.
func handleIPCConn(ctx context.Context, conn net.Conn, server *codamcp.Server) {
	defer conn.Close()

	// Read request
	var req hooks.IPCRequest
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&req); err != nil {
		writeIPCError(conn, "invalid request")
		return
	}

	// Dispatch by method
	var resp hooks.IPCResponse
	switch req.Method {
	case "evaluate_gate":
		pass, reason := server.Lua().EvaluateGate()
		result := hooks.GateResult{Pass: pass, Reason: reason}
		data, _ := json.Marshal(result)
		resp = hooks.IPCResponse{OK: true, Result: data}

	case "drain_observations":
		obs := server.Observers().Drain()
		result := hooks.ObservationsResult{Observations: obs}
		data, _ := json.Marshal(result)
		resp = hooks.IPCResponse{OK: true, Result: data}

	case "get_mode":
		name := server.Lua().ActiveMode()
		prompt := server.Lua().ActiveSystemPrompt()
		result := hooks.ModeResult{Name: name, SystemPrompt: prompt}
		data, _ := json.Marshal(result)
		resp = hooks.IPCResponse{OK: true, Result: data}

	case "get_status":
		status := map[string]any{
			"mode":  server.Lua().ActiveMode(),
			"modes": server.Lua().ListModes(),
			"tails": server.Observers().ListTails(),
		}
		data, _ := json.Marshal(status)
		resp = hooks.IPCResponse{OK: true, Result: data}

	default:
		resp = hooks.IPCResponse{OK: false, Error: "unknown method: " + req.Method}
	}

	// Send response
	enc := json.NewEncoder(conn)
	enc.Encode(resp)
}

func writeIPCError(conn net.Conn, msg string) {
	resp := hooks.IPCResponse{OK: false, Error: msg}
	enc := json.NewEncoder(conn)
	enc.Encode(resp)
}
