package run

import "testing"

func TestNormalizeCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"k1", "k1"},
		{" k1 ", "k1"},
		{" k1, ,k2 ,k3 ,, ", "k1,k2,k3"},
		{",,,", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := NormalizeCSV(tc.in); got != tc.want {
				t.Fatalf("NormalizeCSV(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
