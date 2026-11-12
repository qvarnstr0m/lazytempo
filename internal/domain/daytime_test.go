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
