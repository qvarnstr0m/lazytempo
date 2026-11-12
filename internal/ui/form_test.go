package ui

import (
	"testing"
	"time"

	"lazytempo/internal/domain"
)

func TestWorklogFormInputFromStartEnd(t *testing.T) {
	issue := domain.Issue{ID: "10119", Key: "CORE-5", ProjectID: "200"}
	accounts := []domain.Account{{ID: 1, Key: "CORE", Name: "Acme", Status: "OPEN"}}
	f := newWorklogForm(issue, accounts, "CORE", nil, time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local))

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

func TestWorklogFormRejectsBadRange(t *testing.T) {
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", nil, time.Now())
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
	f := newWorklogForm(domain.Issue{ID: "1"}, nil, "CORE", existing, time.Now())
	if f.editID != 42 {
		t.Errorf("editID = %d, want 42", f.editID)
	}
	if f.startStr != "16:05" || f.endStr != "17:35" {
		t.Errorf("derived range = %s–%s, want 16:05–17:35", f.startStr, f.endStr)
	}
	if f.dateStr != "2026-11-10" {
		t.Errorf("date = %q, want 2026-11-10", f.dateStr)
	}
}
