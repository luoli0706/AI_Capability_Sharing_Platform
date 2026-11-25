package repository

import "testing"

func TestEscapeLikePattern(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"normal/folder", "normal/folder"},
		{"a_b", `a\_b`},
		{"100%", `100\%`},
		{`a\b`, `a\\b`},
		{`mix_%\_test`, `mix\_\%\\\_test`},
	}
	for _, tc := range cases {
		if got := escapeLikePattern(tc.in); got != tc.want {
			t.Fatalf("escapeLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
