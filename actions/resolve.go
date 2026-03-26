package actions

import "strconv"

// parseProcessRef parses a process reference that can be either a numeric ID or an alias.
// Returns (id, alias) — one will be zero/empty.
func parseProcessRef(ref string) (int, string) {
	if id, err := strconv.Atoi(ref); err == nil {
		return id, ""
	}
	return 0, ref
}
