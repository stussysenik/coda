// Package config provides filesystem path resolution for coda's config,
// state, and Lua script directories.
//
// XDG-like layout:
//   ~/.config/coda/         — user configuration + Lua scripts
//   ~/.local/state/coda/    — runtime state (socket, PID files)
package config

import (
	"os"
	"path/filepath"
)

// ConfigDir returns ~/.config/coda/, creating it if needed.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "coda")
	return dir, os.MkdirAll(dir, 0o755)
}

// StateDir returns ~/.local/state/coda/, creating it if needed.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "state", "coda")
	return dir, os.MkdirAll(dir, 0o755)
}

// SocketPath returns the Unix domain socket path for IPC.
func SocketPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "coda.sock"), nil
}

// LuaDir returns ~/.config/coda/lua/, the directory for user Lua scripts.
func LuaDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	luaDir := filepath.Join(dir, "lua")
	return luaDir, os.MkdirAll(luaDir, 0o755)
}

// ModesDir returns the directory for mode Lua scripts.
func ModesDir() (string, error) {
	luaDir, err := LuaDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(luaDir, "modes")
	return dir, os.MkdirAll(dir, 0o755)
}

// RecipesDir returns the directory for recipe Lua scripts.
func RecipesDir() (string, error) {
	luaDir, err := LuaDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(luaDir, "recipes")
	return dir, os.MkdirAll(dir, 0o755)
}
