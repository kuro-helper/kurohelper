package kuro

import "strings"

// ParseIDSet parses comma-separated Discord IDs from application configuration.
func ParseIDSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = struct{}{}
		}
	}
	return result
}

func isAllowed(ids map[string]struct{}, id string) bool {
	if len(ids) == 0 {
		return true
	}
	_, ok := ids[id]
	return ok
}
