package ui

import (
	"testing"
	"time"

	"lazytempo/internal/domain"
)

func TestWorklogFormInputFromStartEnd(t *testing.T) {
	issue := domain.Issue{ID: "10119", Key: "CORE-5", ProjectID: "200"}
	accounts := []domain.Account{{ID: 1, Key: "CORE", Name: "Acme", Status: "OPEN"}}
	f := newWorklogForm(issue, accounts, "CORE", nil, time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local), "09:00", true, false)

	// Simulate the user picking 16:05–17:35 in the dropdowns.
	f.startStr = "16:05"
	f.endStr = "17:35"

	in, err := f.input()
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if in.TimeSpentSeconds != 90*60 {
		t.Errorf("duration = %d, want %d", in.TimeSpentSeconds, 90*60)
	}
	if in.StartTime != "16:05:00" {
		t.Errorf("startTime = %q, want 16:05:00", in.StartTime)
	}
	if in.AccountKey != "CORE" || in.IssueID != "10119" || in.StartDate != "2026-11-12" {
		t.Errorf("unexpected input: %+v", in)
	}
}

func TestWorklogFormDefaultsStartToNow(t *testing.T) {
	// A new worklog defaults start to the given time and end to 5 minutes later.
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", nil, time.Now(), "16:05", true, false)
	if f.startStr != "16:05" {
		t.Errorf("start = %q, want 16:05", f.startStr)
	}
	if f.endStr != "16:10" {
		t.Errorf("end = %q, want 16:10", f.endStr)
	}
	// Late in the day, the end clamps to the last slot but stays after start.
	f = newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", nil, time.Now(), "23:40", true, false)
	if _, err := domain.SecondsBetween(f.startStr, f.endStr); err != nil {
		t.Errorf("late-day default produced invalid range %s–%s: %v", f.startStr, f.endStr, err)
	}
}

func TestWorklogFormPreselectedAccountFirst(t *testing.T) {
	// Name-sorted accounts with a mid-list preselection; it must be ordered
	// first so Huh doesn't scroll the earlier ones out of view.
	accts := []domain.Account{
		{ID: 1, Key: "AAA", Name: "Aaa", Status: "OPEN"},
		{ID: 2, Key: "BBB", Name: "Bbb", Status: "OPEN"},
		{ID: 3, Key: "CCC", Name: "Ccc", Status: "OPEN"},
	}
	f := newWorklogForm(domain.Issue{ID: "1"}, accts, "CCC", nil, time.Now(), "09:00", true, false)
	// The account field is the first field; its first option should be CCC.
	if f.accountKey != "CCC" {
		t.Fatalf("preselect = %q, want CCC", f.accountKey)
	}
	_ = f.form.Init()
	v := f.form.View()
	ai := indexOf(v, "Ccc (CCC)")
	bi := indexOf(v, "Aaa (AAA)")
	if ai < 0 || bi < 0 || ai > bi {
		t.Errorf("preselected CCC should appear before AAA in the options")
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestWorklogFormOptionalAccount(t *testing.T) {
	// With requireAccount=false, no accounts and an empty selection are allowed.
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "", nil, time.Now(), "09:00", false, false)
	f.accountKey = "" // "(no account)"
	in, err := f.input()
	if err != nil {
		t.Fatalf("optional account: input() returned error: %v", err)
	}
	if in.AccountKey != "" {
		t.Errorf("expected empty account, got %q", in.AccountKey)
	}
}

func TestWorklogFormRequiresAccountWhenSet(t *testing.T) {
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "", nil, time.Now(), "09:00", true, false)
	f.accountKey = ""
	f.startStr, f.endStr = "09:00", "10:00"
	if _, err := f.input(); err == nil {
		t.Error("expected error for missing account when required")
	}
}

func TestWorklogFormRejectsBadRange(t *testing.T) {
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", nil, time.Now(), "09:00", true, false)
	f.startStr = "10:00"
	f.endStr = "09:00" // end before start
	if _, err := f.input(); err == nil {
		t.Error("expected error for end before start")
	}
}

func TestWorklogFormEditDerivesRange(t *testing.T) {
	existing := &domain.Worklog{
		ID:               42,
		StartTime:        "16:05:00",
		TimeSpentSeconds: 90 * 60,
		StartDate:        "2026-11-10",
		AccountKey:       "CORE",
	}
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", existing, time.Now(), "09:00", true, false)
	if f.editID != 42 {
		t.Errorf("editID = %d, want 42", f.editID)
	}
	if f.startStr != "16:05" || f.endStr != "17:35" {
		t.Errorf("derived range = %s–%s, want 16:05–17:35", f.startStr, f.endStr)
	}
	// The date is shown in EU display format and round-trips back to ISO.
	if f.dateStr != "10-11-2026" {
		t.Errorf("date display = %q, want 10-11-2026", f.dateStr)
	}
	in, err := f.input()
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if in.StartDate != "2026-11-10" {
		t.Errorf("StartDate = %q, want 2026-11-10 (ISO)", in.StartDate)
	}
}

func TestWorklogFormUSDate(t *testing.T) {
	existing := &domain.Worklog{StartTime: "09:00:00", TimeSpentSeconds: 3600, StartDate: "2026-11-13", AccountKey: "CORE"}
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", existing, time.Now(), "09:00", true, true)
	if f.dateStr != "11/13/2026" {
		t.Errorf("US date display = %q, want 11/13/2026", f.dateStr)
	}
	in, err := f.input()
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if in.StartDate != "2026-11-13" {
		t.Errorf("US StartDate = %q, want 2026-11-13 (ISO)", in.StartDate)
	}
}
