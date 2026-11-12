package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrEmptyDuration is returned when an empty string is parsed as a duration.
var ErrEmptyDuration = errors.New("duration is empty")

// ParseDuration parses a human time entry into a number of seconds.
//
// Supported formats:
//
//	"1h30m"  -> 1 hour 30 minutes
//	"1.5h"   -> 1 hour 30 minutes
//	"90m"    -> 90 minutes
//	"45s"    -> 45 seconds
//	"1:30"   -> 1 hour 30 minutes (H:MM)
//	"2"      -> 2 hours (a bare number is interpreted as hours)
//
// The result must be greater than zero.
func ParseDuration(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, ErrEmptyDuration
	}

	var seconds int
	var err error
	switch {
	case strings.Contains(s, ":"):
		seconds, err = parseColon(s)
	case containsUnit(s):
		seconds, err = parseUnits(s)
	default:
		// A bare number is interpreted as hours (e.g. "2" -> 2h, "1.5" -> 1h30m).
		seconds, err = parseHoursFloat(s)
	}
	if err != nil {
		return 0, err
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero: %q", s)
	}
	return seconds, nil
}

func containsUnit(s string) bool {
	return strings.ContainsAny(s, "hms")
}

// parseColon parses "H:MM" (and "H:MM:SS") notation.
func parseColon(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid time %q: expected H:MM or H:MM:SS", s)
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid time %q", s)
		}
		nums[i] = n
	}
	if len(parts) > 1 && nums[1] >= 60 {
		return 0, fmt.Errorf("invalid minutes in %q", s)
	}
	seconds := nums[0]*3600 + nums[1]*60
	if len(parts) == 3 {
		if nums[2] >= 60 {
			return 0, fmt.Errorf("invalid seconds in %q", s)
		}
		seconds += nums[2]
	}
	return seconds, nil
}

// parseUnits parses unit notation such as "1h30m", "1.5h", "90m", "45s".
func parseUnits(s string) (int, error) {
	total := 0.0
	num := strings.Builder{}
	seen := false
	for _, r := range s {
		switch r {
		case 'h', 'm', 's':
			if num.Len() == 0 {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			v, err := strconv.ParseFloat(num.String(), 64)
			if err != nil || v < 0 {
				return 0, fmt.Errorf("invalid number in %q", s)
			}
			switch r {
			case 'h':
				total += v * 3600
			case 'm':
				total += v * 60
			case 's':
				total += v
			}
			num.Reset()
			seen = true
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
			num.WriteRune(r)
		case ' ':
			// tolerate spaces between components, e.g. "1h 30m"
		default:
			return 0, fmt.Errorf("invalid character %q in duration %q", r, s)
		}
	}
	if num.Len() > 0 {
		// Trailing number with no unit, e.g. "1h30" — treat as minutes.
		v, err := strconv.ParseFloat(num.String(), 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid number in %q", s)
		}
		total += v * 60
		seen = true
	}
	if !seen {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return int(total + 0.5), nil
}

func parseHoursFloat(s string) (int, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return int(v*3600 + 0.5), nil
}

// FormatDuration renders a number of seconds as a compact "1h30m" string.
func FormatDuration(seconds int) string {
	if seconds <= 0 {
		return "0m"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
