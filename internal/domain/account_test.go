package domain

import (
	"errors"
	"testing"
)

func TestWorklogValidate(t *testing.T) {
	base := Worklog{TimeSpentSeconds: 3600, StartDate: "2026-11-12", AccountKey: "CORE"}

	if err := base.Validate(true); err != nil {
		t.Fatalf("valid worklog rejected: %v", err)
	}

	noAccount := base
	noAccount.AccountKey = ""
	if err := noAccount.Validate(true); !errors.Is(err, ErrNoAccount) {
		t.Errorf("worklog without account (required): got %v, want ErrNoAccount", err)
	}
	// With the rule relaxed, a missing account is allowed.
	if err := noAccount.Validate(false); err != nil {
		t.Errorf("worklog without account (not required): got %v, want nil", err)
	}

	noTime := base
	noTime.TimeSpentSeconds = 0
	if err := noTime.Validate(true); !errors.Is(err, ErrNoTimeSpent) {
		t.Errorf("worklog without time: got %v, want ErrNoTimeSpent", err)
	}

	noDate := base
	noDate.StartDate = ""
	if err := noDate.Validate(true); !errors.Is(err, ErrNoDate) {
		t.Errorf("worklog without date: got %v, want ErrNoDate", err)
	}
}

func sampleAccounts() []Account {
	return []Account{
		{ID: 1, Key: "CORE", Name: "Core", Status: "OPEN"},
		{ID: 2, Key: "RND", Name: "R&D", Status: "OPEN"},
		{ID: 3, Key: "OLD", Name: "Legacy", Status: "CLOSED"},
		{ID: 4, Key: "OTHER", Name: "Other project", Status: "OPEN"},
	}
}

func sampleLinks() []AccountLink {
	return []AccountLink{
		{AccountID: 1, ProjectID: "100", Default: true},
		{AccountID: 2, ProjectID: "100"},
		{AccountID: 3, ProjectID: "100"},
		{AccountID: 4, ProjectID: "200"},
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
	if got := DefaultAccountKey("100", sampleAccounts(), sampleLinks()); got != "CORE" {
		t.Errorf("DefaultAccountKey = %q, want CORE", got)
	}
	if got := DefaultAccountKey("200", sampleAccounts(), sampleLinks()); got != "" {
		t.Errorf("DefaultAccountKey for project without default = %q, want empty", got)
	}
}

func TestPreselectAccount(t *testing.T) {
	selectable := SelectableAccounts("100", sampleAccounts(), sampleLinks())
	defaultKey := DefaultAccountKey("100", sampleAccounts(), sampleLinks())

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
