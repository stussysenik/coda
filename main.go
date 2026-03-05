package main

import (
	"fmt"
	"os"

	"github.com/s3nik/coda/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = cmd.Serve()
	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: coda hook <event>")
			os.Exit(1)
		}
		err = cmd.Hook(os.Args[2])
	case "mode":
		err = cmd.Mode(os.Args[2:])
	case "install":
		err = cmd.Install()
	case "status":
		err = cmd.Status()
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `coda — Claude Code Autonomous Dev Orchestrator

Usage:
  coda serve       Run as MCP server (stdio)
  coda hook <evt>  Handle a Claude Code hook event (Stop, PostToolUse)
  coda mode <cmd>  Manage modes (set, list)
  coda install     One-command project setup
  coda status      Show current state (observers, gate, mode)`)
}
