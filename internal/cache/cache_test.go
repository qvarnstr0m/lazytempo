package cache

import (
	"os"
	"path/filepath"
	"testing"

	"lazytempo/internal/domain"
)

func TestVersionMismatchDropsStaleData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Write a v1-style cache: stale account links plus user data to preserve.
	stale := `{
		"version": 1,
		"account_links": {"13932": [{"ProjectID":"13932","Default":true}]},
		"pinned": {"PROJ-1": true},
		"last_account_by_issue": {"62487": "CORE"}
	}`
	path := filepath.Join(dir, "lazytempo", "cache.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	// Stale, wrong-shape account links must be dropped so they get re-fetched.
	if got := c.AccountLinks("13932"); len(got) != 0 {
		t.Errorf("expected stale account links dropped, got %+v", got)
	}
	// User data survives the format change.
	if c.LastAccountForIssue("62487") != "CORE" {
		t.Error("last-used account should survive a version bump")
	}
	if c.Issues(); len(c.Issues()) != 0 {
		t.Error("expected no cached issues after version bump")
	}
}

func TestLastAccountRoundTrip(t *testing.T) {
	c := &Cache{data: Data{LastAccountByIssue: map[string]string{}, Pinned: map[string]bool{}, AccountLinks: map[string][]domain.AccountLink{}}}
	c.SetLastAccountForIssue("10991", "CORE")
	if got := c.LastAccountForIssue("10991"); got != "CORE" {
		t.Errorf("LastAccountForIssue = %q, want CORE", got)
	}
	// Empty values are ignored.
	c.SetLastAccountForIssue("", "X")
	c.SetLastAccountForIssue("10992", "")
	if c.LastAccountForIssue("10992") != "" {
		t.Error("empty account should not be stored")
	}
}

func TestTogglePin(t *testing.T) {
	c := &Cache{data: Data{LastAccountByIssue: map[string]string{}, Pinned: map[string]bool{}, AccountLinks: map[string][]domain.AccountLink{}}}
	if !c.TogglePin("PROJ-1") {
		t.Error("first toggle should pin")
	}
	if c.TogglePin("PROJ-1") {
		t.Error("second toggle should unpin")
	}
}

func TestIssuesApplyPins(t *testing.T) {
	c := &Cache{data: Data{
		Issues:             []domain.Issue{{Key: "PROJ-1"}, {Key: "PROJ-2"}},
		Pinned:             map[string]bool{"PROJ-2": true},
		LastAccountByIssue: map[string]string{},
		AccountLinks:       map[string][]domain.AccountLink{},
	}}
	issues := c.Issues()
	if issues[0].Pinned {
		t.Error("PROJ-1 should not be pinned")
	}
	if !issues[1].Pinned {
		t.Error("PROJ-2 should be pinned")
	}
}
