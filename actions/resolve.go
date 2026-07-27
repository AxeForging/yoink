package actions

import "github.com/AxeForging/yoink/services"

// parseProcessRef parses a process reference that can be either a numeric ID or an alias.
// Returns (id, alias) — one will be zero/empty. Shared with the MCP server so both
// frontends resolve references identically.
func parseProcessRef(ref string) (int, string) {
	return services.ParseProcessRef(ref)
}
