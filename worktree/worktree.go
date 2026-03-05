// Package worktree manages git worktrees for parallel Claude Code sessions.
//
// Each worktree is a separate checkout of the same repository, allowing
// multiple Claude instances to work on different tasks simultaneously
// without stepping on each other's changes.
package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// WorktreeDir is the subdirectory within the repo root for coda worktrees.
	WorktreeDir = ".worktrees"
	// BranchPrefix is prepended to worktree branch names.
	BranchPrefix = "coda/"
)

// Create creates a new git worktree with a dedicated branch.
// Returns the absolute path to the worktree directory.
func Create(repoRoot, name string) (string, error) {
	wtDir := filepath.Join(repoRoot, WorktreeDir, name)
	branch := BranchPrefix + name

	// Ensure the .worktrees directory exists
	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	// Create the worktree with a new branch
	cmd := exec.Command("git", "worktree", "add", wtDir, "-b", branch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If branch already exists, try without -b
		cmd = exec.Command("git", "worktree", "add", wtDir, branch)
		cmd.Dir = repoRoot
		out, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}

	return wtDir, nil
}

// Remove removes a git worktree.
func Remove(repoRoot, name string) error {
	wtDir := filepath.Join(repoRoot, WorktreeDir, name)

	cmd := exec.Command("git", "worktree", "remove", wtDir, "--force")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// List returns all coda worktrees in the repository.
func List(repoRoot string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// Filter to only coda worktrees
	var codaWorktrees []WorktreeInfo
	for _, wt := range worktrees {
		if strings.HasPrefix(wt.Branch, BranchPrefix) {
			wt.Name = strings.TrimPrefix(wt.Branch, BranchPrefix)
			codaWorktrees = append(codaWorktrees, wt)
		}
	}

	return codaWorktrees, nil
}

// RepoRoot finds the git repository root from the current directory.
func RepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// WorktreeInfo describes a git worktree.
type WorktreeInfo struct {
	Name   string // coda name (without prefix)
	Path   string // absolute path
	Branch string // full branch name
}
