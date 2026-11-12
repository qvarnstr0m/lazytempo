package domain

import (
	"errors"
	"testing"
)

func TestWorklogValidate(t *testing.T) {
	base := Worklog{TimeSpentSeconds: 3600, StartDate: "2026-11-12", AccountKey: "CORE"}

	if err := base.Validate(); err != nil {
		t.Fatalf("valid worklog rejected: %v", err)
	}

	noAccount := base
	noAccount.AccountKey = ""
	if err := noAccount.Validate(); !errors.Is(err, ErrNoAccount) {
		t.Errorf("worklog without account: got %v, want ErrNoAccount", err)
	}

	noTime := base
	noTime.TimeSpentSeconds = 0
	if err := noTime.Validate(); !errors.Is(err, ErrNoTimeSpent) {
		t.Errorf("worklog without time: got %v, want ErrNoTimeSpent", err)
	}

	noDate := base
	noDate.StartDate = ""
	if err := noDate.Validate(); !errors.Is(err, ErrNoDate) {
		t.Errorf("worklog without date: got %v, want ErrNoDate", err)
	}
}

func sampleAccounts() []Account {
	return []Account{
		{Key: "CORE", Name: "Core", Status: "OPEN"},
		{Key: "RND", Name: "R&D", Status: "OPEN"},
		{Key: "OLD", Name: "Legacy", Status: "CLOSED"},
		{Key: "OTHER", Name: "Other project", Status: "OPEN"},
	}
}

func sampleLinks() []AccountLink {
	return []AccountLink{
		{AccountKey: "CORE", ProjectID: "100", Default: true},
		{AccountKey: "RND", ProjectID: "100"},
		{AccountKey: "OLD", ProjectID: "100"},
		{AccountKey: "OTHER", ProjectID: "200"},
	}
}

func TestSelectableAccounts(t *testing.T) {
	got := SelectableAccounts("100", sampleAccounts(), sampleLinks())
	// CORE and RND are linked to project 100 and open; OLD is closed; OTHER is
	// linked to a different project.
	if len(got) != 2 {
		t.Fatalf("expected 2 selectable accounts, got %d: %+v", len(got), got)
	}
	// Sorted by name: "Core" before "R&D".
	if got[0].Key != "CORE" || got[1].Key != "RND" {
		t.Errorf("unexpected order/content: %+v", got)
	}
}

func TestSelectableAccountsNone(t *testing.T) {
	// A project with no valid accounts yields an empty slice (logging is blocked).
	got := SelectableAccounts("999", sampleAccounts(), sampleLinks())
	if len(got) != 0 {
		t.Errorf("expected no selectable accounts, got %+v", got)
	}
}

func TestDefaultAccountKey(t *testing.T) {
	if got := DefaultAccountKey("100", sampleLinks()); got != "CORE" {
		t.Errorf("DefaultAccountKey = %q, want CORE", got)
	}
	if got := DefaultAccountKey("200", sampleLinks()); got != "" {
		t.Errorf("DefaultAccountKey for project without default = %q, want empty", got)
	}
}

func TestPreselectAccount(t *testing.T) {
	selectable := SelectableAccounts("100", sampleAccounts(), sampleLinks())
	defaultKey := DefaultAccountKey("100", sampleLinks())

	// 1. Last used wins when it is still selectable.
	if got := PreselectAccount(selectable, "RND", defaultKey); got != "RND" {
		t.Errorf("last-used preselection = %q, want RND", got)
	}

	// 1a. A stale last-used account (no longer selectable) is skipped, falling
	// through to the project default.
	if got := PreselectAccount(selectable, "OLD", defaultKey); got != "CORE" {
		t.Errorf("stale last-used should fall back to default, got %q, want CORE", got)
	}

	// 2. Default is used when there is no last-used account.
	if got := PreselectAccount(selectable, "", defaultKey); got != "CORE" {
		t.Errorf("default preselection = %q, want CORE", got)
	}

	// 3. No preselection when neither applies.
	if got := PreselectAccount(selectable, "", ""); got != "" {
		t.Errorf("expected no preselection, got %q", got)
	}
}
