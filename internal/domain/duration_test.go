package domain

import "testing"

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{5400, "1h30m"},
		{7200, "2h"},
		{2700, "45m"},
		{0, "0m"},
		{-5, "0m"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.in); got != c.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
