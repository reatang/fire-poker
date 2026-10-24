package toolkit

import (
	"slices"
	"strings"
)

// NormalizeToolIDs removes empty and duplicate IDs while preserving an explicit empty selection.
func NormalizeToolIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		// Resolve the legacy generic ID without enabling any foreign platform tool.
		if id == "shell" && PlatformShellToolID() != "" {
			id = PlatformShellToolID()
		}
		if id != "" && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result
}

// SameToolIDs compares selections as sets without modifying caller-owned slices.
func SameToolIDs(left, right []string) bool {
	l, r := NormalizeToolIDs(left), NormalizeToolIDs(right)
	slices.Sort(l)
	slices.Sort(r)
	return slices.Equal(l, r)
}
