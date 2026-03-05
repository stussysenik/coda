package lua

// Recipe defines an ordered sequence of tool calls (Programmed Tool Calling).
// Recipes let users script multi-step workflows that Claude Code executes.
type Recipe struct {
	Name        string
	Description string
	Steps       []RecipeStep
}

// RecipeStep is a single step in a recipe.
type RecipeStep struct {
	Tool string            // MCP tool name to call
	Args map[string]string // arguments to pass
}
