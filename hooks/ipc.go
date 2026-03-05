package hooks

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/s3nik/coda/config"
)

// IPCRequest is a command sent from hook processes to the coda serve process.
type IPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// IPCResponse is the response from the coda serve process.
type IPCResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// GateResult is the structured response from gate evaluation.
type GateResult struct {
	Pass   bool   `json:"pass"`
	Reason string `json:"reason"`
}

// ObservationsResult is the structured response from draining observations.
type ObservationsResult struct {
	Observations []string `json:"observations"`
}

// ModeResult is the structured response from get_mode.
type ModeResult struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
}

// ipcCall sends a request to the coda serve process and returns the response.
func ipcCall(method string, params any) (*IPCResponse, error) {
	sockPath, err := config.SocketPath()
	if err != nil {
		return nil, fmt.Errorf("socket path: %w", err)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connect to coda serve: %w (is `coda serve` running?)", err)
	}
	defer conn.Close()

	// Build request
	req := IPCRequest{Method: method}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	// Send request
	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response
	var resp IPCResponse
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &resp, nil
}
