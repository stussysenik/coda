// Package mcp implements coda's MCP stdio server.
//
// Claude Code launches `coda serve` as a subprocess and communicates over
// stdin/stdout using the Model Context Protocol. This server exposes tools
// that let Claude observe running processes, run tests, take screenshots,
// evaluate completion gates, and manage parallel worktree sessions.
package mcp

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	codalua "github.com/s3nik/coda/lua"
	"github.com/s3nik/coda/observer"
	"github.com/s3nik/coda/worktree"
)

// Server wraps the MCP server and coda's runtime state.
type Server struct {
	mcpServer *mcp.Server
	observers *observer.Manager
	lua       *codalua.Engine
	pool      *worktree.Pool // may be nil if not in a git repo
}

// New creates a coda MCP server with all tools registered.
func New() *Server {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "coda",
			Version: "0.1.0",
		},
		nil,
	)

	obs := observer.NewManager()

	// Try to initialize worktree pool if in a git repo
	var pool *worktree.Pool
	if root, err := worktree.RepoRoot(); err == nil {
		pool = worktree.NewPool(root)
	}

	s := &Server{
		mcpServer: mcpServer,
		observers: obs,
		lua:       codalua.NewEngine(obs),
		pool:      pool,
	}

	s.registerTools()
	return s
}

// Run starts the MCP server over stdio, blocking until the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	log.SetOutput(log.Writer())
	return s.mcpServer.Run(ctx, mcp.NewStdioTransport())
}

// Observers returns the observer manager for IPC access.
func (s *Server) Observers() *observer.Manager {
	return s.observers
}

// Lua returns the Lua engine for IPC access.
func (s *Server) Lua() *codalua.Engine {
	return s.lua
}

// Pool returns the worktree pool (may be nil).
func (s *Server) Pool() *worktree.Pool {
	return s.pool
}
