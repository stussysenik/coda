-- test_and_fix.lua — Recipe: Run tests, if they fail, ask Claude to fix
--
-- This recipe demonstrates Programmed Tool Calling (PTC):
-- a deterministic sequence of tool calls that Claude Code executes.

local coda = require("coda")

-- TODO: Recipe registration API (Phase 3+)
-- For now this serves as a reference for the recipe format.
--
-- coda.define_recipe({
--     name = "test_and_fix",
--     description = "Run tests. If any fail, show the failures to Claude and ask it to fix them.",
--     steps = {
--         {tool = "run_tests", args = {command = "mix test --no-color"}},
--         -- If previous step had failures, Claude will see them and fix
--     },
-- })
