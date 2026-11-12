package ui

import (
	"testing"

	"lazytempo/internal/domain"
)

func sampleIssues() []domain.Issue {
	return []domain.Issue{
		{ID: "1", Key: "PROJ-790", Summary: "Password rotation in API", ProjectKey: "PROJ"},
		{ID: "2", Key: "PROJ-794", Summary: "Reset password link returns invalid token", ProjectKey: "PROJ"},
		{ID: "3", Key: "CORE-5", Summary: "Internal training and support", ProjectKey: "CORE"},
		{ID: "4", Key: "CORE-603", Summary: "Internal meetings", ProjectKey: "CORE"},
	}
}

func TestFilterIssuesSingleTerm(t *testing.T) {
	got := filterIssues(sampleIssues(), "password")
	if len(got) != 2 {
		t.Fatalf("expected 2 password matches, got %d: %+v", len(got), got)
	}
}

func TestFilterIssuesMultiTermAnd(t *testing.T) {
	// Both terms must match (across key + summary + project).
	got := filterIssues(sampleIssues(), "proj reset")
	if len(got) != 1 || got[0].Key != "PROJ-794" {
		t.Fatalf("expected only PROJ-794, got %+v", got)
	}
}

func TestFilterIssuesByProjectKey(t *testing.T) {
	got := filterIssues(sampleIssues(), "core")
	if len(got) != 2 {
		t.Fatalf("expected 2 CORE issues, got %d: %+v", len(got), got)
	}
}

func TestFilterIssuesByRawKey(t *testing.T) {
	got := filterIssues(sampleIssues(), "core-603")
	if len(got) != 1 || got[0].Key != "CORE-603" {
		t.Fatalf("expected CORE-603, got %+v", got)
	}
}

func TestFilterIssuesEmptyReturnsAll(t *testing.T) {
	if got := filterIssues(sampleIssues(), "   "); len(got) != 4 {
		t.Errorf("empty query should return all, got %d", len(got))
	}
}

func TestFilterIssuesNoMatch(t *testing.T) {
	if got := filterIssues(sampleIssues(), "zzz nomatch"); len(got) != 0 {
		t.Errorf("expected no matches, got %+v", got)
	}
}
