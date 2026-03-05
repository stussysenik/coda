-- dev.lua — Development mode for coda
--
-- This mode tails a development server and runs tests as part of
-- its completion gate. Claude Code won't stop while the server
-- has errors or tests are failing.

local coda = require("coda")

coda.define_mode({
    name = "dev",

    system_prompt = [[You are in dev mode. After every code change:
1. Check the server tail for compilation errors
2. Run the test suite
3. Fix any failures before moving on

Never stop while the gate is failing. Use check_gate to verify.]],

    tools = {"tail_server", "read_tail", "run_tests", "check_gate", "get_observations"},

    observers = {
        {type = "tail", command = "mix phx.server", id = "phoenix"},
    },

    gate = function()
        -- Check 1: Server tail for errors
        local output = coda.get_tail_output("phoenix")
        if output and (output:find("error") or output:find("Error") or output:find("CompileError")) then
            return false, "Server has errors:\n" .. output:sub(1, 500)
        end

        -- Check 2: Test suite
        local result = coda.run("mix test --no-color 2>&1")
        if result.exit_code ~= 0 then
            return false, "Tests failing:\n" .. result.output:sub(-500)
        end

        return true, "All checks pass"
    end,
})
