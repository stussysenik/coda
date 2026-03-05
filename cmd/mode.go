package cmd

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/s3nik/coda/config"
	"github.com/s3nik/coda/hooks"
)

// Mode manages Lua modes (set, list).
func Mode(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: coda mode <set|list> [name]")
	}

	switch args[0] {
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: coda mode set <name>")
		}
		return modeSet(args[1])
	case "list":
		return modeList()
	default:
		return fmt.Errorf("unknown mode command: %s (expected set or list)", args[0])
	}
}

func modeSet(name string) error {
	resp, err := modeIPC("set_mode", map[string]string{"name": name})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("set mode: %s", resp.Error)
	}
	fmt.Printf("Mode set to: %s\n", name)
	return nil
}

func modeList() error {
	resp, err := modeIPC("get_status", nil)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("list modes: %s", resp.Error)
	}

	var status map[string]any
	json.Unmarshal(resp.Result, &status)

	if modes, ok := status["modes"].([]any); ok {
		active := ""
		if m, ok := status["mode"].(string); ok {
			active = m
		}
		fmt.Println("Available modes:")
		for _, m := range modes {
			name := fmt.Sprint(m)
			marker := "  "
			if name == active {
				marker = "* "
			}
			fmt.Printf("  %s%s\n", marker, name)
		}
	} else {
		fmt.Println("No modes available. Load Lua scripts first.")
	}

	return nil
}

func modeIPC(method string, params any) (*hooks.IPCResponse, error) {
	sockPath, err := config.SocketPath()
	if err != nil {
		return nil, fmt.Errorf("socket path: %w", err)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("coda serve not running (start it first)")
	}
	defer conn.Close()

	req := hooks.IPCRequest{Method: method}
	if params != nil {
		data, _ := json.Marshal(params)
		req.Params = data
	}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	var resp hooks.IPCResponse
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	return &resp, nil
}
