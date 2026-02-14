package run

import "strings"

// MaskKeys returns a masked representation of a secret, keeping only the first
// and last rune and replacing the middle with '*'.
//
// If s contains multiple secrets separated by the provided separator, each item
// is masked individually and the output preserves the original separators and
// item formatting (including empty items and whitespace).
//
// Examples:
//
//	""         -> ""
//	"a"        -> "*"
//	"ab"       -> "a*"
//	"abcd"     -> "a**d"
//	"abcd,ef"  -> "a**d,e*"
func MaskKeys(s string, separator string) string {
	if s == "" {
		return ""
	}

	if separator != "" && strings.Contains(s, separator) {
		parts := strings.Split(s, separator)
		masked := make([]string, 0, len(parts))
		for _, p := range parts {
			masked = append(masked, MaskKey(p))
		}
		return strings.Join(masked, separator)
	}

	return MaskKey(s)
}

func MaskKey(s string) string {
	rs := []rune(s)
	switch len(rs) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(rs[0]) + "*"
	default:
		return string(rs[0]) + strings.Repeat("*", len(rs)-2) + string(rs[len(rs)-1])
	}
}
