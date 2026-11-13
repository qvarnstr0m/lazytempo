package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"lazytempo/internal/cache"
	"lazytempo/internal/config"
	"lazytempo/internal/domain"
)

type panel int

const (
	panelIssues panel = iota
	panelWeek
	panelDetails
	panelCount
)

// sortMode controls the ordering of the issue list.
type sortMode int

const (
	sortDefault sortMode = iota // JQL order (recently updated), worklog issues appended
	sortRecent                  // most recently logged on
	sortMost                    // most time logged on
)

func (s sortMode) label() string {
	switch s {
	case sortRecent:
		return "recently logged"
	case sortMost:
		return "most logged"
	default:
		return "default"
	}
}

// searchDebounceMsg fires after a pause in typing to trigger a fallback Jira
// search; gen lets stale ticks be ignored.
type searchDebounceMsg struct {
	gen   int
	query string
}

// pendingSelect records where the worklog cursor should land once a newly
// requested week's worklogs have loaded (used when up/down crosses a week).
// Days are shown chronologically, so "top of week" is the earliest day
// (Monday) and "bottom of week" is the latest (Sunday).
type pendingSelect int

const (
	selectNone         pendingSelect = iota
	selectTopOfWeek                  // earliest day (Monday), first worklog (top of the list)
	selectBottomOfWeek               // latest day (Sunday), last worklog (bottom of the list)
)

// formRequest records a pending request to open the worklog form once account
// links for the issue's project are available.
type formRequest struct {
	issue    domain.Issue
	existing *domain.Worklog
	// accountOnly means "change account" — same as edit but conceptually focused.
	accountOnly bool
}

// Model is the root Bubble Tea model. It routes messages to the focused panel
// and owns all shared state.
type Model struct {
	svc   *Service
	cache *cache.Cache
	cfg   config.Config
	theme Theme
	keys  KeyMap

	help    help.Model
	spinner spinner.Model
	search  textinput.Model

	width, height int
	focused       panel
	showHelp      bool

	// Reference data.
	accountID      string
	accountAttrKey string
	userName       string
	lastSync       time.Time

	// Issues panel.
	issues      []domain.Issue
	issueCursor int
	searching   bool
	// Fallback Jira search when nothing loaded matches the query.
	remoteResults   []domain.Issue
	remoteQuery     string
	remoteSearching bool
	searchGen       int // debounce generation, ignores stale ticks
	// Issue sort order and the per-issue worklog stats it can use.
	sortMode     sortMode
	worklogStats map[string]worklogStat

	// Week panel.
	weekStart      time.Time
	selectedDay    int
	days           []domain.Day
	worklogCursor  int
	pendingWorklog pendingSelect // where to put the cursor after a cross-week load

	// Overlays.
	form             *worklogForm
	pending          *formRequest
	confirmingDelete bool

	// Status bar.
	status    string
	statusErr bool
	loading   int
}

// New builds the root model.
func New(svc *Service, c *cache.Cache, cfg config.Config) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipglossAccent()

	si := textinput.New()
	si.Prompt = "/ "
	si.Placeholder = "filter issues or type a key like PROJ-123"

	h := help.New()

	now := time.Now()
	m := Model{
		svc:         svc,
		cache:       c,
		cfg:         cfg,
		theme:       NewTheme(),
		keys:        DefaultKeyMap(),
		help:        h,
		spinner:     sp,
		search:      si,
		focused:     panelIssues,
		weekStart:   weekStart(now),
		selectedDay: todayIndex(now),
		issues:      c.Issues(),
	}
	m.rebuildDays(nil)
	return m
}

func todayIndex(now time.Time) int {
	idx := (int(now.Weekday()) + 6) % 7
	return idx
}

// Init starts background loads and the spinner.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startLoad(m.svc.LoadReference()),
		m.startLoad(m.svc.LoadIssues()),
	)
}

// startLoad increments the in-flight counter and returns the command. Because
// Model is passed by value through Bubble Tea, the counter is adjusted here and
// decremented when the corresponding message arrives.
func (m *Model) startLoad(cmd tea.Cmd) tea.Cmd {
	m.loading++
	return cmd
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		// Keep an open form sized to the box so fields (notably the description)
		// wrap at the right width.
		if m.form != nil {
			m.form.form = m.form.form.WithWidth(formContentWidth(msg.Width)).WithHeight(formContentHeight(msg.Height))
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case referenceLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("auth/reference failed: %v", msg.err))
			return m, nil
		}
		m.accountID = msg.accountID
		m.accountAttrKey = msg.attrKey
		m.userName = msg.displayName
		m.lastSync = time.Now()
		m.svc.SetAccountID(msg.accountID)
		to := time.Now().Format(dateLayout)
		from := time.Now().AddDate(0, 0, -30).Format(dateLayout)
		return m, tea.Batch(
			m.startLoad(m.svc.LoadWorklogs(m.weekStart)),
			m.startLoad(m.svc.SeedIssuesFromWorklogs(from, to)),
		)

	case issuesLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("load issues failed: %v", msg.err))
			return m, nil
		}
		m.issues = msg.issues
		m.lastSync = time.Now()
		m.clampIssueCursor()
		return m, m.ensureLinksForSelectedIssue()

	case searchDebounceMsg:
		// Only act on the latest tick, and only if still needed.
		if msg.gen != m.searchGen || strings.TrimSpace(m.search.Value()) != msg.query {
			return m, nil
		}
		if len(m.localMatches()) > 0 {
			return m, nil
		}
		m.remoteSearching = true
		return m, m.svc.SearchRemote(msg.query)

	case remoteIssuesLoadedMsg:
		m.remoteSearching = false
		if msg.err != nil {
			m.setStatus("Jira search: no match")
			return m, nil
		}
		if strings.TrimSpace(m.search.Value()) == msg.query {
			m.remoteResults = msg.issues
			m.remoteQuery = msg.query
			m.clampIssueCursor()
		}
		return m, nil

	case worklogsLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("load worklogs failed: %v", msg.err))
			return m, nil
		}
		m.lastSync = time.Now()
		if msg.from.Equal(m.weekStart) {
			m.rebuildDays(msg.worklogs)
			m.applyPendingWorklog()
		}
		// Resolve any logged issues not already in the list (e.g. when viewing a
		// week outside the initial 30-day seed window).
		if missing := m.missingIssueIDs(msg.worklogs); len(missing) > 0 {
			return m, m.startLoad(m.svc.ResolveIssues(missing))
		}
		return m, nil

	case worklogIssuesLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("resolve logged issues failed: %v", msg.err))
			return m, nil
		}
		if msg.stats != nil {
			m.worklogStats = msg.stats
		}
		if len(msg.issues) > 0 {
			m.issues = m.cache.Issues()
			m.clampIssueCursor()
		}
		return m, nil

	case accountLinksLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("load accounts for project failed: %v", msg.err))
		}
		// If a form was waiting on these links, open it now.
		if m.pending != nil && m.pending.issue.ProjectID == msg.projectID {
			return m, m.openPendingForm()
		}
		return m, nil

	case worklogSavedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("save failed: %v", msg.err))
			return m, nil
		}
		m.setStatus("worklog saved")
		return m, m.startLoad(m.svc.LoadWorklogs(m.weekStart))

	case worklogDeletedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("delete failed: %v", msg.err))
			return m, nil
		}
		m.setStatus("worklog deleted")
		return m, m.startLoad(m.svc.LoadWorklogs(m.weekStart))

	case statusMsg:
		if msg.isError {
			m.setError(msg.text)
		} else {
			m.setStatus(msg.text)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Route to the active form if present.
	if m.form != nil {
		return m.updateForm(msg)
	}
	return m, nil
}

// handleKey dispatches key presses based on the current mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Form captures all keys while open.
	if m.form != nil {
		return m.updateForm(msg)
	}

	// Delete confirmation captures y/n.
	if m.confirmingDelete {
		switch msg.String() {
		case "y", "Y":
			m.confirmingDelete = false
			wl := m.selectedWorklog()
			if wl == nil {
				return m, nil
			}
			return m, m.startLoad(m.svc.DeleteWorklog(wl.ID))
		default:
			m.confirmingDelete = false
			m.setStatus("delete cancelled")
			return m, nil
		}
	}

	// Search input captures keys while active.
	if m.searching {
		return m.updateSearch(msg)
	}

	// Help overlay is modal: esc, q or ? closes it and returns to the main view;
	// ctrl+c still quits the app.
	if m.showHelp {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "?", "esc", "q":
			m.showHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(msg, m.keys.NextPanel):
		m.focused = (m.focused + 1) % panelCount
		return m, tea.Batch(m.ensureLinksForSelectedIssue(), m.resumeSearchIfFiltered())
	case key.Matches(msg, m.keys.Panel1):
		m.focused = panelIssues
		return m, m.resumeSearchIfFiltered()
	case key.Matches(msg, m.keys.Panel2):
		m.focused = panelWeek
		return m, nil
	case key.Matches(msg, m.keys.Panel3):
		m.focused = panelDetails
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		to := time.Now().Format(dateLayout)
		from := time.Now().AddDate(0, 0, -30).Format(dateLayout)
		return m, tea.Batch(
			m.startLoad(m.svc.LoadIssues()),
			m.startLoad(m.svc.LoadWorklogs(m.weekStart)),
			m.startLoad(m.svc.SeedIssuesFromWorklogs(from, to)),
		)
	case key.Matches(msg, m.keys.Search):
		m.searching = true
		m.focused = panelIssues
		m.search.Focus()
		return m, textinput.Blink
	}

	switch m.focused {
	case panelIssues:
		return m.handleIssuesKey(msg)
	case panelWeek:
		return m.handleWeekKey(msg)
	}
	return m, nil
}

func (m Model) handleIssuesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.issueCursor < len(m.visibleIssues())-1 {
			m.issueCursor++
		}
		return m, m.ensureLinksForSelectedIssue()
	case key.Matches(msg, m.keys.Up):
		// At the top of a filtered list, go back up into the search box (the
		// counterpart to ↓/tab dropping into the list).
		if m.issueCursor == 0 && m.search.Value() != "" {
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		}
		if m.issueCursor > 0 {
			m.issueCursor--
		}
		return m, m.ensureLinksForSelectedIssue()
	case key.Matches(msg, m.keys.Pin):
		if iss := m.selectedIssue(); iss != nil {
			now := m.cache.TogglePin(iss.Key)
			m.issues = m.cache.Issues()
			if now {
				m.setStatus("pinned " + iss.Key)
			} else {
				m.setStatus("unpinned " + iss.Key)
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Sort):
		m.sortMode = (m.sortMode + 1) % 3
		m.issueCursor = 0
		m.setStatus("sorted by " + m.sortMode.label())
		return m, m.ensureLinksForSelectedIssue()
	case key.Matches(msg, m.keys.Add):
		return m.requestAddWorklog()
	}
	return m, nil
}

// weekMoveDown moves one row down the chronological list: down within the day,
// then into the later day below, then — past the latest day — into the top
// (Monday) of the next week.
func (m Model) weekMoveDown() (tea.Model, tea.Cmd) {
	if n := len(m.currentDayWorklogs()); n > 0 && m.worklogCursor < n-1 {
		m.worklogCursor++
		return m, nil
	}
	if nd := m.nextDayWithWorklogs(m.selectedDay); nd >= 0 {
		m.selectedDay = nd
		m.worklogCursor = 0
		return m, nil
	}
	return m.crossWeek(1, selectTopOfWeek)
}

// weekMoveUp moves one row up the chronological list: up within the day, then
// into the earlier day above, then — past the earliest day — into the bottom
// (Sunday) of the previous week.
func (m Model) weekMoveUp() (tea.Model, tea.Cmd) {
	if m.worklogCursor > 0 {
		m.worklogCursor--
		return m, nil
	}
	if pd := m.prevDayWithWorklogs(m.selectedDay); pd >= 0 {
		m.selectedDay = pd
		m.worklogCursor = len(m.days[pd].Worklogs) - 1
		return m, nil
	}
	return m.crossWeek(-1, selectBottomOfWeek)
}

func (m Model) handleWeekKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down): // j / down
		return m.weekMoveDown()
	case key.Matches(msg, m.keys.Up): // k / up
		return m.weekMoveUp()
	case key.Matches(msg, m.keys.NextDay):
		if m.selectedDay < 6 {
			m.selectedDay++
			m.worklogCursor = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.PrevDay):
		if m.selectedDay > 0 {
			m.selectedDay--
			m.worklogCursor = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.NextWeek):
		m.weekStart = m.weekStart.AddDate(0, 0, 7)
		m.worklogCursor = 0
		m.rebuildDays(nil)
		return m, m.startLoad(m.svc.LoadWorklogs(m.weekStart))
	case key.Matches(msg, m.keys.PrevWeek):
		m.weekStart = m.weekStart.AddDate(0, 0, -7)
		m.worklogCursor = 0
		m.rebuildDays(nil)
		return m, m.startLoad(m.svc.LoadWorklogs(m.weekStart))
	case key.Matches(msg, m.keys.Add):
		return m.requestAddWorklog()
	case key.Matches(msg, m.keys.Edit), key.Matches(msg, m.keys.Account):
		return m.requestEditWorklog()
	case key.Matches(msg, m.keys.Delete):
		if m.selectedWorklog() != nil {
			m.confirmingDelete = true
			m.setStatus("delete this worklog? (y/n)")
		}
		return m, nil
	}
	return m, nil
}

// resumeSearchIfFiltered re-focuses the search input when returning to the
// Issues panel while a filter is still active, so the query can be edited or
// cleared immediately without pressing "/" again.
func (m *Model) resumeSearchIfFiltered() tea.Cmd {
	if m.focused == panelIssues && !m.searching && m.search.Value() != "" {
		m.searching = true
		m.search.Focus()
		return textinput.Blink
	}
	return nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Blur()
		m.search.SetValue("")
		m.issueCursor = 0
		return m, nil
	case "enter", "tab", "down", "ctrl+n":
		// Move focus from the search box into the results, keeping the filter.
		// The highlighted issue stays selected, so j/k navigation and the
		// command keys (a/e/d/c/p) act on it immediately — no Enter needed, and
		// every character remains typeable while the box is focused.
		m.searching = false
		m.search.Blur()
		return m, m.ensureLinksForSelectedIssue()
	}

	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	// Only reset the cursor when the query actually changed, so it stays put
	// while you move the text caret.
	if m.search.Value() != prev {
		m.issueCursor = 0
		// The old remote results are for a different query; drop them and
		// schedule a fresh fallback search if nothing loaded matches.
		m.remoteResults = nil
		m.remoteQuery = ""
		if dc := m.scheduleRemoteSearch(); dc != nil {
			return m, tea.Batch(cmd, dc)
		}
	}
	return m, cmd
}

// scheduleRemoteSearch returns a debounced command that triggers a fallback
// Jira search, but only when the query is substantial and nothing loaded
// matches it. It returns nil when no remote search is warranted.
func (m *Model) scheduleRemoteSearch() tea.Cmd {
	q := strings.TrimSpace(m.search.Value())
	if len([]rune(q)) < 2 || len(m.localMatches()) > 0 {
		return nil
	}
	m.searchGen++
	gen := m.searchGen
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return searchDebounceMsg{gen: gen, query: q}
	})
}

// localMatches returns the loaded issues matching the current query (excludes
// remote results).
func (m Model) localMatches() []domain.Issue {
	return filterIssues(m.issues, m.search.Value())
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	// Allow Esc to abort the form.
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		m.form = nil
		m.setStatus("cancelled")
		return m, m.resumeSearchIfFiltered()
	}
	form, cmd := m.form.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form.form = f
	}
	switch m.form.form.State {
	case huh.StateCompleted:
		in, err := m.form.input()
		editID := m.form.editID
		m.form = nil
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		in.AuthorAccountID = m.accountID
		return m, tea.Batch(m.startLoad(m.svc.SaveWorklog(editID, in)), m.resumeSearchIfFiltered())
	case huh.StateAborted:
		m.form = nil
		m.setStatus("cancelled")
		return m, m.resumeSearchIfFiltered()
	}
	return m, cmd
}

// requestAddWorklog starts the flow to add a worklog to the selected issue.
func (m Model) requestAddWorklog() (tea.Model, tea.Cmd) {
	iss := m.selectedIssue()
	if iss == nil {
		m.setError("no issue selected")
		return m, nil
	}
	m.pending = &formRequest{issue: *iss}
	return m, m.ensureLinksAndOpen(iss.ProjectID)
}

// requestEditWorklog starts the flow to edit the selected worklog.
func (m Model) requestEditWorklog() (tea.Model, tea.Cmd) {
	wl := m.selectedWorklog()
	if wl == nil {
		m.setError("no worklog selected")
		return m, nil
	}
	iss := m.issueByID(wl.IssueID)
	if iss == nil {
		m.setError(fmt.Sprintf("issue %s not in list; open it via search first", wl.IssueID))
		return m, nil
	}
	w := *wl
	m.pending = &formRequest{issue: *iss, existing: &w}
	return m, m.ensureLinksAndOpen(iss.ProjectID)
}

// ensureLinksAndOpen loads account links for a project if needed, then opens the
// pending form (immediately when links are already cached).
func (m *Model) ensureLinksAndOpen(projectID string) tea.Cmd {
	if len(m.cache.AccountLinks(projectID)) > 0 {
		return m.openPendingForm()
	}
	return m.startLoad(m.svc.LoadAccountLinks(projectID))
}

// openPendingForm builds and shows the worklog form from m.pending, enforcing
// the account rules (selectable accounts and preselection).
func (m *Model) openPendingForm() tea.Cmd {
	if m.pending == nil {
		return nil
	}
	req := m.pending
	m.pending = nil

	projectID := req.issue.ProjectID
	links := m.cache.AccountLinks(projectID)
	selectable := domain.SelectableAccounts(projectID, m.cache.Accounts(), links)
	// Block only when accounts are required; otherwise allow logging with none.
	if len(selectable) == 0 && m.cfg.RequireAccount {
		m.setError(fmt.Sprintf("no valid Tempo accounts for project %s — logging blocked", req.issue.ProjectKey))
		return nil
	}

	lastUsed := m.cache.LastAccountForIssue(req.issue.ID)
	defaultKey := domain.DefaultAccountKey(projectID, m.cache.Accounts(), links)
	preselect := domain.PreselectAccount(selectable, lastUsed, defaultKey)

	// A new worklog always defaults to today's date and the current time.
	now := time.Now()
	defaultDate := now
	defaultStart := domain.RoundClock(now.Hour()*60+now.Minute(), clockStepMinutes)
	m.form = newWorklogForm(req.issue, selectable, preselect, req.existing, defaultDate, defaultStart, m.cfg.RequireAccount, m.cfg.Hour12())
	m.form.form = m.form.form.WithWidth(formContentWidth(m.width)).WithHeight(formContentHeight(m.height))
	return m.form.form.Init()
}

// ensureLinksForSelectedIssue pre-loads account links for the selected issue so
// the form opens instantly later.
func (m *Model) ensureLinksForSelectedIssue() tea.Cmd {
	iss := m.selectedIssue()
	if iss == nil || iss.ProjectID == "" {
		return nil
	}
	if len(m.cache.AccountLinks(iss.ProjectID)) > 0 {
		return nil
	}
	return m.startLoad(m.svc.LoadAccountLinks(iss.ProjectID))
}

// --- helpers for selection and state ---

func (m *Model) rebuildDays(worklogs []domain.Worklog) {
	m.days = buildDays(m.weekStart, worklogs)
	m.clampWorklogCursor()
}

func (m *Model) clampIssueCursor() {
	n := len(m.visibleIssues())
	if m.issueCursor >= n {
		m.issueCursor = n - 1
	}
	if m.issueCursor < 0 {
		m.issueCursor = 0
	}
}

func (m *Model) clampWorklogCursor() {
	n := len(m.currentDayWorklogs())
	if m.worklogCursor >= n {
		m.worklogCursor = n - 1
	}
	if m.worklogCursor < 0 {
		m.worklogCursor = 0
	}
}

// visibleIssues returns pinned-first, filtered-by-search issue list. While
// searching it also includes fallback results fetched from Jira (deduped).
func (m Model) visibleIssues() []domain.Issue {
	if !(m.searching || m.search.Value() != "") {
		return m.applySort(m.issues)
	}
	pool := m.issues
	if len(m.remoteResults) > 0 {
		seen := make(map[string]bool, len(m.issues))
		for _, i := range m.issues {
			seen[i.ID] = true
		}
		pool = append([]domain.Issue{}, m.issues...)
		for _, r := range m.remoteResults {
			if !seen[r.ID] {
				pool = append(pool, r)
			}
		}
	}
	return m.applySort(filterIssues(pool, m.search.Value()))
}

func (m Model) selectedIssue() *domain.Issue {
	vis := m.visibleIssues()
	if m.issueCursor < 0 || m.issueCursor >= len(vis) {
		return nil
	}
	return &vis[m.issueCursor]
}

// missingIssueIDs returns the distinct issue ids referenced by the worklogs that
// are not yet present in the loaded issue list.
func (m Model) missingIssueIDs(worklogs []domain.Worklog) []string {
	seen := map[string]bool{}
	var missing []string
	for _, w := range worklogs {
		if w.IssueID == "" || seen[w.IssueID] {
			continue
		}
		seen[w.IssueID] = true
		if m.issueByID(w.IssueID) == nil {
			missing = append(missing, w.IssueID)
		}
	}
	return missing
}

func (m Model) issueByID(id string) *domain.Issue {
	for i := range m.issues {
		if m.issues[i].ID == id {
			return &m.issues[i]
		}
	}
	for i := range m.remoteResults {
		if m.remoteResults[i].ID == id {
			return &m.remoteResults[i]
		}
	}
	return nil
}

// applyPendingWorklog places the cursor at the top/bottom of the just-loaded
// week (in chronological order) when a cross-week move requested it. If the week
// has no worklogs the selection stays at the start.
func (m *Model) applyPendingWorklog() {
	switch m.pendingWorklog {
	case selectTopOfWeek: // earliest day (Monday), first worklog
		if d := m.nextDayWithWorklogs(-1); d >= 0 {
			m.selectedDay = d
			m.worklogCursor = 0
		}
	case selectBottomOfWeek: // latest day (Sunday), last worklog
		if d := m.prevDayWithWorklogs(len(m.days)); d >= 0 {
			m.selectedDay = d
			m.worklogCursor = len(m.days[d].Worklogs) - 1
		}
	}
	m.pendingWorklog = selectNone
}

// crossWeek moves to the adjacent week (dir -1/+1), loads its worklogs, and
// records where the cursor should land once they arrive (first/last worklog).
func (m Model) crossWeek(dir int, where pendingSelect) (tea.Model, tea.Cmd) {
	m.weekStart = m.weekStart.AddDate(0, 0, 7*dir)
	m.selectedDay = 0
	m.worklogCursor = 0
	m.pendingWorklog = where
	m.rebuildDays(nil)
	return m, m.startLoad(m.svc.LoadWorklogs(m.weekStart))
}

// nextDayWithWorklogs returns the first day index after `from` that has any
// worklogs, or -1 when there is none.
func (m Model) nextDayWithWorklogs(from int) int {
	for i := from + 1; i < len(m.days); i++ {
		if len(m.days[i].Worklogs) > 0 {
			return i
		}
	}
	return -1
}

// prevDayWithWorklogs returns the last day index before `from` that has any
// worklogs, or -1 when there is none.
func (m Model) prevDayWithWorklogs(from int) int {
	for i := from - 1; i >= 0; i-- {
		if len(m.days[i].Worklogs) > 0 {
			return i
		}
	}
	return -1
}

func (m Model) currentDayWorklogs() []domain.Worklog {
	if m.selectedDay < 0 || m.selectedDay >= len(m.days) {
		return nil
	}
	return m.days[m.selectedDay].Worklogs
}

func (m Model) selectedWorklog() *domain.Worklog {
	wls := m.currentDayWorklogs()
	if m.worklogCursor < 0 || m.worklogCursor >= len(wls) {
		return nil
	}
	return &wls[m.worklogCursor]
}

func (m *Model) setStatus(s string) {
	m.status = s
	m.statusErr = false
}

func (m *Model) setError(s string) {
	m.status = s
	m.statusErr = true
}

// filterIssues keeps issues matching a free-text query. The query is split into
// whitespace-separated terms; an issue matches when every term is a
// case-insensitive substring of its key, summary or project key. This makes
// queries like "proj login" or "core report" work intuitively.
func filterIssues(issues []domain.Issue, query string) []domain.Issue {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return issues
	}
	var out []domain.Issue
	for _, i := range issues {
		haystack := strings.ToLower(i.Key + " " + i.Summary + " " + i.ProjectKey)
		if matchesAllTerms(haystack, terms) {
			out = append(out, i)
		}
	}
	return out
}

func matchesAllTerms(haystack string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

// applySort orders issues pinned-first, then by the active sort mode: by last
// logged date (recently logged), by total logged time (most logged), or the
// incoming order (default). The sort is stable so equal issues keep their order.
func (m Model) applySort(issues []domain.Issue) []domain.Issue {
	out := make([]domain.Issue, len(issues))
	copy(out, issues)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Pinned != b.Pinned {
			return a.Pinned // pinned first
		}
		switch m.sortMode {
		case sortRecent:
			if la, lb := m.worklogStats[a.ID].lastDate, m.worklogStats[b.ID].lastDate; la != lb {
				return la > lb // later date first; no logs ("") sorts last
			}
		case sortMost:
			if ta, tb := m.worklogStats[a.ID].totalSeconds, m.worklogStats[b.ID].totalSeconds; ta != tb {
				return ta > tb
			}
		}
		return false // keep incoming order (stable)
	})
	return out
}
