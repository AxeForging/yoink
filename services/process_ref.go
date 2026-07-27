package services

import "strconv"

// ParseProcessRef splits a user-supplied process reference into the numeric ID
// and alias fields the daemon protocol expects. A ref that parses as an integer
// is an ID; anything else is an alias.
func ParseProcessRef(ref string) (id int, alias string) {
	if n, err := strconv.Atoi(ref); err == nil {
		return n, ""
	}
	return 0, ref
}
