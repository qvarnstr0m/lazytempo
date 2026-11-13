package domain

import "testing"

func TestTimeOptions(t *testing.T) {
	opts := TimeOptions(5)
	if len(opts) != 288 {
		t.Fatalf("expected 288 slots, got %d", len(opts))
	}
	if opts[0] != "00:00" || opts[1] != "00:05" || opts[len(opts)-1] != "23:55" {
		t.Errorf("unexpected boundaries: %q %q %q", opts[0], opts[1], opts[len(opts)-1])
	}
}

func TestParseClock(t *testing.T) {
	ok := map[string]int{"00:00": 0, "16:05": 965, "23:59": 1439, " 9:30 ": 570}
	for in, want := range ok {
		got, err := ParseClock(in)
		if err != nil || got != want {
			t.Errorf("ParseClock(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "16", "24:00", "16:60", "-1:00", "aa:bb", "16:05:00"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) expected error", bad)
		}
	}
}

func TestSecondsBetween(t *testing.T) {
	got, err := SecondsBetween("16:05", "17:35")
	if err != nil || got != 90*60 {
		t.Errorf("SecondsBetween = %d, %v; want %d", got, err, 90*60)
	}
	// End not after start is an error.
	if _, err := SecondsBetween("10:00", "10:00"); err == nil {
		t.Error("equal times should error")
	}
	if _, err := SecondsBetween("10:00", "09:00"); err == nil {
		t.Error("end before start should error")
	}
}

func TestFormatClockDisplay(t *testing.T) {
	eu := map[string]string{"00:00": "00:00", "09:30": "09:30", "16:05": "16:05", "23:59": "23:59"}
	for in, want := range eu {
		if got := FormatClockDisplay(in, false); got != want {
			t.Errorf("EU FormatClockDisplay(%q) = %q, want %q", in, got, want)
		}
	}
	us := map[string]string{
		"00:00": "12:00 AM",
		"09:30": "9:30 AM",
		"12:00": "12:00 PM",
		"13:05": "1:05 PM",
		"16:05": "4:05 PM",
		"23:59": "11:59 PM",
	}
	for in, want := range us {
		if got := FormatClockDisplay(in, true); got != want {
			t.Errorf("US FormatClockDisplay(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDateDisplayRoundTrip(t *testing.T) {
	// EU
	if got := FormatDateDisplay("2026-11-13", false); got != "13/11/2026" {
		t.Errorf("EU format = %q, want 13/11/2026", got)
	}
	if iso, err := ParseDateDisplay("13/11/2026", false); err != nil || iso != "2026-11-13" {
		t.Errorf("EU parse = %q, %v; want 2026-11-13", iso, err)
	}
	// US
	if got := FormatDateDisplay("2026-11-13", true); got != "11/13/2026" {
		t.Errorf("US format = %q, want 11/13/2026", got)
	}
	if iso, err := ParseDateDisplay("11/13/2026", true); err != nil || iso != "2026-11-13" {
		t.Errorf("US parse = %q, %v; want 2026-11-13", iso, err)
	}
	// Invalid input errors with a helpful hint.
	if _, err := ParseDateDisplay("2026-11-13", false); err == nil {
		t.Error("ISO input should not parse as EU display format")
	}
}

func TestRoundClock(t *testing.T) {
	cases := map[int]string{
		16*60 + 3:  "16:05", // 16:03 -> nearest is 16:05
		16*60 + 2:  "16:00", // 16:02 -> nearest is 16:00
		16 * 60:    "16:00", // exact
		23*60 + 58: "23:55", // would round up to 24:00; clamped to last slot
		0:          "00:00",
	}
	for in, want := range cases {
		if got := RoundClock(in, 5); got != want {
			t.Errorf("RoundClock(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAddClock(t *testing.T) {
	if got := AddClock("16:05", 90); got != "17:35" {
		t.Errorf("AddClock = %q, want 17:35", got)
	}
	// Clamped so it never wraps past midnight.
	if got := AddClock("23:30", 120); got != "23:59" {
		t.Errorf("AddClock clamp = %q, want 23:59", got)
	}
}
