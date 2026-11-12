package domain

import "testing"

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"1h30m", 5400},
		{"1.5h", 5400},
		{"90m", 5400},
		{"1:30", 5400},
		{"1:30:00", 5400},
		{"2h", 7200},
		{"45m", 2700},
		{"45s", 45},
		{"2", 7200},        // bare number = hours
		{"0.25h", 900},     // quarter hour
		{" 1h 30m ", 5400}, // spaces tolerated
		{"1h30", 5400},     // trailing number = minutes
		{"0:45", 2700},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if err != nil {
			t.Errorf("ParseDuration(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseDurationErrors(t *testing.T) {
	cases := []string{"", "   ", "abc", "1x", "-1h", "0", "0h", "1:90", "1:2:90", "h"}
	for _, c := range cases {
		if _, err := ParseDuration(c); err == nil {
			t.Errorf("ParseDuration(%q) expected error, got nil", c)
		}
	}
}

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
