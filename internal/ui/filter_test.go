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

func TestRenderGauge(t *testing.T) {
	m := Model{theme: NewTheme()}
	// Half of target -> ~5 of 10 cells filled.
	g := countRunes(m.renderGauge(4*3600, 8*3600, 10), '█')
	if g != 5 {
		t.Errorf("half target: %d filled, want 5", g)
	}
	// At/over target -> full.
	if g := countRunes(m.renderGauge(9*3600, 8*3600, 10), '█'); g != 10 {
		t.Errorf("over target: %d filled, want 10", g)
	}
	// A tiny non-zero amount still shows at least one cell.
	if g := countRunes(m.renderGauge(60, 8*3600, 10), '█'); g != 1 {
		t.Errorf("tiny amount: %d filled, want 1", g)
	}
	// Zero -> empty.
	if g := countRunes(m.renderGauge(0, 8*3600, 10), '█'); g != 0 {
		t.Errorf("zero: %d filled, want 0", g)
	}
}

func countRunes(s string, r rune) int {
	n := 0
	for _, c := range s {
		if c == r {
			n++
		}
	}
	return n
}

func TestWeekWorklogCrossDayNav(t *testing.T) {
	// Mon (idx 0) has 2 worklogs, Thu (idx 3) has 1; Tue/Wed are empty.
	days := make([]domain.Day, 7)
	days[0].Worklogs = []domain.Worklog{{ID: 1}, {ID: 2}}
	days[3].Worklogs = []domain.Worklog{{ID: 3}}
	m := Model{keys: DefaultKeyMap(), focused: panelWeek, days: days}

	// Start on the first worklog of Thu; up should jump to the last of Mon.
	m.selectedDay, m.worklogCursor = 3, 0
	res, _ := m.handleWeekKey(tea.KeyMsg{Type: tea.KeyUp})
	mm := res.(Model)
	if mm.selectedDay != 0 || mm.worklogCursor != 1 {
		t.Fatalf("up from Thu[0] -> day %d cursor %d, want day 0 cursor 1", mm.selectedDay, mm.worklogCursor)
	}

	// Down from the last worklog of Mon should jump to the first of Thu.
	mm.selectedDay, mm.worklogCursor = 0, 1
	res, _ = mm.handleWeekKey(tea.KeyMsg{Type: tea.KeyDown})
	mm = res.(Model)
	if mm.selectedDay != 3 || mm.worklogCursor != 0 {
		t.Fatalf("down from Mon[last] -> day %d cursor %d, want day 3 cursor 0", mm.selectedDay, mm.worklogCursor)
	}

	// At the very top (Mon[0]), up stays put.
	mm.selectedDay, mm.worklogCursor = 0, 0
	res, _ = mm.handleWeekKey(tea.KeyMsg{Type: tea.KeyUp})
	mm = res.(Model)
	if mm.selectedDay != 0 || mm.worklogCursor != 0 {
		t.Errorf("up at top moved to day %d cursor %d, want day 0 cursor 0", mm.selectedDay, mm.worklogCursor)
	}

	// At the very bottom (Thu[0], its only one), down stays put.
	mm.selectedDay, mm.worklogCursor = 3, 0
	res, _ = mm.handleWeekKey(tea.KeyMsg{Type: tea.KeyDown})
	mm = res.(Model)
	if mm.selectedDay != 3 || mm.worklogCursor != 0 {
		t.Errorf("down at bottom moved to day %d cursor %d, want day 3 cursor 0", mm.selectedDay, mm.worklogCursor)
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
