package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ClockLayout is the 24-hour European clock format used in the UI (e.g. 16:05).
const ClockLayout = "15:04"

// ISODate is the canonical date layout used for storage and the Tempo API.
const ISODate = "2006-01-02"

func dateGoLayout(us bool) string {
	if us {
		return "01/02/2006" // MM/DD/YYYY
	}
	return "02-01-2006" // DD-MM-YYYY
}

// DateHint returns a human placeholder for the display date format.
func DateHint(us bool) string {
	if us {
		return "MM/DD/YYYY"
	}
	return "DD-MM-YYYY"
}

// FormatDateDisplay renders a canonical ISO date ("2006-01-02") in the display
// format (US: MM/DD/YYYY, EU: DD/MM/YYYY). On a parse failure it returns input.
func FormatDateDisplay(iso string, us bool) string {
	t, err := time.Parse(ISODate, iso)
	if err != nil {
		return iso
	}
	return t.Format(dateGoLayout(us))
}

// ParseDateDisplay parses a display-format date back to canonical ISO.
func ParseDateDisplay(s string, us bool) (string, error) {
	t, err := time.Parse(dateGoLayout(us), strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("use format %s", DateHint(us))
	}
	return t.Format(ISODate), nil
}

// TimeOptions returns the clock labels of a day at the given step in minutes,
// from 00:00 up to (but not including) 24:00, in "HH:MM" 24-hour format.
// For step 5 this yields 00:00, 00:05, … 23:55 (288 entries).
func TimeOptions(stepMinutes int) []string {
	if stepMinutes <= 0 {
		stepMinutes = 5
	}
	var out []string
	for m := 0; m < 24*60; m += stepMinutes {
		out = append(out, formatClock(m))
	}
	return out
}

func formatClock(minutesOfDay int) string {
	h := minutesOfDay / 60
	m := minutesOfDay % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

// FormatClockDisplay renders a canonical "HH:MM" (24-hour) clock for display.
// hour12 selects 12-hour US style ("4:05 PM") over 24-hour EU style ("16:05").
// On a parse failure it returns the input unchanged.
func FormatClockDisplay(hhmm string, hour12 bool) string {
	m, err := ParseClock(hhmm)
	if err != nil {
		return hhmm
	}
	return formatClockDisplay(m, hour12)
}

func formatClockDisplay(minutesOfDay int, hour12 bool) string {
	h := minutesOfDay / 60
	m := minutesOfDay % 60
	if !hour12 {
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	ampm := "AM"
	if h >= 12 {
		ampm = "PM"
	}
	hh := h
	if h == 0 {
		hh = 12
	} else if h > 12 {
		hh = h - 12
	}
	return fmt.Sprintf("%d:%02d %s", hh, m, ampm)
}

// ParseClock parses an "HH:MM" 24-hour time into minutes since midnight.
func ParseClock(s string) (int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h*60 + m, nil
}

// SecondsBetween returns the number of seconds between start and end clock
// times (both "HH:MM"). The end must be strictly after the start — worklogs
// may not span midnight.
func SecondsBetween(start, end string) (int, error) {
	s, err := ParseClock(start)
	if err != nil {
		return 0, err
	}
	e, err := ParseClock(end)
	if err != nil {
		return 0, err
	}
	if e <= s {
		return 0, fmt.Errorf("end time %s must be after start time %s", end, start)
	}
	return (e - s) * 60, nil
}

// RoundClock rounds a minute-of-day value to the nearest step and returns the
// matching "HH:MM" clock label, clamped to the last slot of the day so it
// always corresponds to a valid dropdown option.
func RoundClock(minutesOfDay, step int) string {
	if step <= 0 {
		step = 5
	}
	rounded := ((minutesOfDay + step/2) / step) * step
	if last := (24*60/step - 1) * step; rounded > last {
		rounded = last
	}
	if rounded < 0 {
		rounded = 0
	}
	return formatClock(rounded)
}

// AddClock returns the clock time minutesToAdd after the given clock time,
// clamped to 23:59 so it never wraps past midnight.
func AddClock(start string, minutesToAdd int) string {
	s, err := ParseClock(start)
	if err != nil {
		return start
	}
	total := s + minutesToAdd
	if total > 24*60-1 {
		total = 24*60 - 1
	}
	return formatClock(total)
}
