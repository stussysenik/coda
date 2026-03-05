package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/s3nik/coda/config"
	"github.com/s3nik/coda/hooks"
)

// Status shows the current state of coda: active mode, observers, gate.
func Status() error {
	sockPath, err := config.SocketPath()
	if err != nil {
		return fmt.Errorf("socket path: %w", err)
	}

	// Check if serve is running by trying to connect
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		fmt.Println("coda serve: not running")
		fmt.Printf("  socket: %s (unreachable)\n", sockPath)
		return nil
	}
	defer conn.Close()

	// Query status
	req := hooks.IPCRequest{Method: "get_status"}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return fmt.Errorf("send status request: %w", err)
	}

	var resp hooks.IPCResponse
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return fmt.Errorf("read status response: %w", err)
	}

	if !resp.OK {
		return fmt.Errorf("status error: %s", resp.Error)
	}

	// Pretty print status
	var status map[string]any
	json.Unmarshal(resp.Result, &status)

	fmt.Println("coda serve: running")
	fmt.Printf("  socket: %s\n", sockPath)

	if mode, ok := status["mode"].(string); ok && mode != "" {
		fmt.Printf("  active mode: %s\n", mode)
	} else {
		fmt.Println("  active mode: (none)")
	}

	if modes, ok := status["modes"].([]any); ok {
		fmt.Printf("  available modes: ")
		for i, m := range modes {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(m)
		}
		fmt.Println()
	}

	if tails, ok := status["tails"].([]any); ok && len(tails) > 0 {
		fmt.Printf("  active tails: ")
		for i, t := range tails {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(t)
		}
		fmt.Println()
	} else {
		fmt.Println("  active tails: (none)")
	}

	// Also evaluate the gate
	conn2, err := net.Dial("unix", sockPath)
	if err == nil {
		defer conn2.Close()
		gateReq := hooks.IPCRequest{Method: "evaluate_gate"}
		enc2 := json.NewEncoder(conn2)
		enc2.Encode(gateReq)

		var gateResp hooks.IPCResponse
		dec2 := json.NewDecoder(conn2)
		if dec2.Decode(&gateResp) == nil && gateResp.OK {
			var gate hooks.GateResult
			json.Unmarshal(gateResp.Result, &gate)
			status := "PASS"
			if !gate.Pass {
				status = "FAIL"
			}
			fmt.Printf("  gate: %s — %s\n", status, gate.Reason)
		}
	}

	fmt.Fprintln(os.Stderr) // blank line
	return nil
}
