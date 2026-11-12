package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lazytempo/internal/domain"
)

func searchingModel(value string) Model {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Focus()
	return Model{search: ti, searching: true, focused: panelIssues,
		issues: sampleIssues()}
}

func TestSearchDownFocusesList(t *testing.T) {
	// ↓ / tab / enter move focus into the list, keeping the filter, so command
	// keys act on the highlighted issue without an extra step.
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyTab},
		{Type: tea.KeyEnter},
	} {
		m := searchingModel("core")
		res, _ := m.updateSearch(key)
		mm := res.(Model)
		if mm.searching {
			t.Errorf("%v: expected searching=false (focus moved to list)", key)
		}
		if mm.search.Focused() {
			t.Errorf("%v: expected search input blurred", key)
		}
		if mm.search.Value() != "core" {
			t.Errorf("%v: filter should be kept, got %q", key, mm.search.Value())
		}
	}
}

func TestUpAtTopReturnsToSearch(t *testing.T) {
	// In a filtered list, pressing up on the top issue returns to the search box.
	ti := textinput.New()
	ti.SetValue("core")
	m := Model{search: ti, searching: false, focused: panelIssues,
		issues: sampleIssues(), issueCursor: 0, keys: DefaultKeyMap()}

	res, _ := m.handleIssuesKey(tea.KeyMsg{Type: tea.KeyUp})
	mm := res.(Model)
	if !mm.searching || !mm.search.Focused() {
		t.Error("up on top of a filtered list should re-focus the search box")
	}

	// With a non-zero cursor it just moves up, staying in the list.
	m.issueCursor = 2
	res, _ = m.handleIssuesKey(tea.KeyMsg{Type: tea.KeyUp})
	mm = res.(Model)
	if mm.searching {
		t.Error("up from a lower row should not jump to search")
	}
	if mm.issueCursor != 1 {
		t.Errorf("cursor = %d, want 1", mm.issueCursor)
	}

	// With no active filter, up on top does nothing special.
	m2 := Model{search: textinput.New(), focused: panelIssues, issues: sampleIssues(), issueCursor: 0, keys: DefaultKeyMap()}
	res, _ = m2.handleIssuesKey(tea.KeyMsg{Type: tea.KeyUp})
	if res.(Model).searching {
		t.Error("up with no filter should not open search")
	}
}

func TestHelpOverlayCloses(t *testing.T) {
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'?'}},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	} {
		m := Model{showHelp: true, keys: DefaultKeyMap()}
		res, _ := m.handleKey(k)
		if res.(Model).showHelp {
			t.Errorf("%v should close the help overlay", k)
		}
	}
}

func TestSearchLettersStillType(t *testing.T) {
	// A command letter like 'a' is typed into the query while searching, not
	// treated as a command.
	m := searchingModel("p")
	res, _ := m.updateSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm := res.(Model)
	if !mm.searching {
		t.Error("typing a letter should stay in search mode")
	}
	if mm.search.Value() != "pa" {
		t.Errorf("expected query 'pa', got %q", mm.search.Value())
	}
}

func TestResumeSearchIfFiltered(t *testing.T) {
	m := &Model{search: textinput.New(), focused: panelIssues}

	// No query: nothing happens.
	if cmd := m.resumeSearchIfFiltered(); cmd != nil || m.searching {
		t.Fatal("should not resume search when query is empty")
	}

	// Active query, returned to Issues panel, not currently searching: re-focus.
	m.search.SetValue("password")
	m.searching = false
	m.resumeSearchIfFiltered()
	if !m.searching || !m.search.Focused() {
		t.Error("expected search to be re-focused on return to Issues panel")
	}

	// Not on the Issues panel: do not steal focus.
	m2 := &Model{search: textinput.New(), focused: panelWeek}
	m2.search.SetValue("password")
	if cmd := m2.resumeSearchIfFiltered(); cmd != nil || m2.searching {
		t.Error("should not resume search when another panel is focused")
	}
}

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
