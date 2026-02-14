package run

import "testing"

func TestMaskKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{" ", "*"},
		{"a", "*"},
		{"ab", "a*"},
		{"abcd", "a**d"},
		{"  abcd  ", " ****** "},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := MaskKey(tc.in); got != tc.want {
				t.Fatalf("MaskKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskKeys_CSV(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		sep  string
		want string
	}{
		{"two_keys", "abcd,efgh", CommaSeparator, "a**d,e**h"},
		// Preserva espacios dentro de cada ítem.
		{"two_keys_spaces_preserved", " abcd , efgh ", CommaSeparator, " **** , **** "},
		{"short_items", "a,bc", CommaSeparator, "*,b*"},
		// Ítems vacíos y whitespace se preservan tal cual.
		{"keeps_empties", "k1, ,k2 ,k3 ,, ", CommaSeparator, "k*,*,k* ,k* ,,*"},
		{"only_separators", ",,,", CommaSeparator, ",,,"},
		{"leading_empty", "  , k1 ,  ", CommaSeparator, " *, ** , *"},
		{"separator_empty_string", "abcd,efgh", "", "a*******h"},
		{"separator_not_present", "abcd", CommaSeparator, "a**d"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskKeys(tc.in, tc.sep); got != tc.want {
				t.Fatalf("MaskKeys(%q, %q) = %q, want %q", tc.in, tc.sep, got, tc.want)
			}
		})
	}
}
