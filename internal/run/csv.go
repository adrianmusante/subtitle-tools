package run

import "strings"

// CommaSeparator is the separator used for comma-separated configuration values
// (e.g. multiple API keys).
const CommaSeparator = ","

// NormalizeCSV trims spaces around each item in a comma-separated list,
// discards empty items, and returns a normalized CSV string (without spaces).
//
// Examples:
//
//	" k1, ,k2 ,k3 ,, " -> "k1,k2,k3"
//	",,," -> ""
func NormalizeCSV(s string) string {
	parts := strings.Split(s, CommaSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, CommaSeparator)
}
