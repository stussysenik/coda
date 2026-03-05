package lua

import (
	"testing"

	"github.com/s3nik/coda/observer"
)

func TestDefineMode(t *testing.T) {
	obs := observer.NewManager()
	engine := NewEngine(obs)
	defer engine.Close()

	err := engine.vm.DoString(`
		local coda = require("coda")
		coda.define_mode({
			name = "test",
			system_prompt = "You are in test mode.",
			tools = {"run_tests", "check_gate"},
			observers = {
				{type = "tail", command = "echo hello", id = "test-server"},
			},
			gate = function()
				return true, "all good"
			end,
		})
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	modes := engine.ListModes()
	if len(modes) != 1 || modes[0] != "test" {
		t.Fatalf("ListModes = %v, want [test]", modes)
	}
}

func TestGateEvaluation(t *testing.T) {
	obs := observer.NewManager()
	engine := NewEngine(obs)
	defer engine.Close()

	// No active mode — gate passes by default
	pass, reason := engine.EvaluateGate()
	if !pass {
		t.Errorf("no mode: gate pass = false, want true")
	}
	if reason == "" {
		t.Error("expected a reason string")
	}

	// Load a mode with a passing gate
	err := engine.vm.DoString(`
		local coda = require("coda")
		coda.define_mode({
			name = "passing",
			gate = function()
				return true, "everything passes"
			end,
		})
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if err := engine.SetMode("passing"); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	pass, reason = engine.EvaluateGate()
	if !pass {
		t.Errorf("passing mode: gate pass = false, want true (reason: %s)", reason)
	}
	if reason != "everything passes" {
		t.Errorf("reason = %q, want %q", reason, "everything passes")
	}
}

func TestGateFailing(t *testing.T) {
	obs := observer.NewManager()
	engine := NewEngine(obs)
	defer engine.Close()

	err := engine.vm.DoString(`
		local coda = require("coda")
		coda.define_mode({
			name = "failing",
			gate = function()
				return false, "tests are broken"
			end,
		})
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if err := engine.SetMode("failing"); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	pass, reason := engine.EvaluateGate()
	if pass {
		t.Error("failing mode: gate pass = true, want false")
	}
	if reason != "tests are broken" {
		t.Errorf("reason = %q, want %q", reason, "tests are broken")
	}
}

func TestSetModeUnknown(t *testing.T) {
	obs := observer.NewManager()
	engine := NewEngine(obs)
	defer engine.Close()

	err := engine.SetMode("nonexistent")
	if err == nil {
		t.Error("SetMode(nonexistent) should return error")
	}
}

func TestLuaRunCommand(t *testing.T) {
	obs := observer.NewManager()
	engine := NewEngine(obs)
	defer engine.Close()

	err := engine.vm.DoString(`
		local coda = require("coda")
		local result = coda.run("echo hello")
		assert(result.exit_code == 0, "exit_code should be 0")
		assert(result.output:find("hello"), "output should contain hello")
	`)
	if err != nil {
		t.Fatalf("Lua run test failed: %v", err)
	}
}
