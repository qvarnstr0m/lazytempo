package cache

import (
	"testing"

	"lazytempo/internal/domain"
)

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
