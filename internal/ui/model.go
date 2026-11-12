package ui

import (
	"fmt"
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

	// Issues panel.
	issues      []domain.Issue
	issueCursor int
	searching   bool

	// Week panel.
	weekStart     time.Time
	selectedDay   int
	days          []domain.Day
	worklogCursor int

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
			m.form.form = m.form.form.WithWidth(formContentWidth(msg.Width))
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
		m.clampIssueCursor()
		return m, m.ensureLinksForSelectedIssue()

	case worklogsLoadedMsg:
		m.loading--
		if msg.err != nil {
			m.setError(fmt.Sprintf("load worklogs failed: %v", msg.err))
			return m, nil
		}
		if msg.from.Equal(m.weekStart) {
			m.rebuildDays(msg.worklogs)
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

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(msg, m.keys.NextPanel):
		m.focused = (m.focused + 1) % panelCount
		return m, m.ensureLinksForSelectedIssue()
	case key.Matches(msg, m.keys.Panel1):
		m.focused = panelIssues
		return m, nil
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
	case key.Matches(msg, m.keys.Add):
		return m.requestAddWorklog()
	}
	return m, nil
}

func (m Model) handleWeekKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		if n := len(m.currentDayWorklogs()); n > 0 && m.worklogCursor < n-1 {
			m.worklogCursor++
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.worklogCursor > 0 {
			m.worklogCursor--
		}
		return m, nil
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

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Blur()
		m.search.SetValue("")
		m.issueCursor = 0
		return m, nil
	case "enter":
		// Commit the current selection: keep the filter, leave the input so
		// j/k navigation takes over.
		m.searching = false
		m.search.Blur()
		return m, m.ensureLinksForSelectedIssue()
	case "down", "ctrl+n":
		// Move the selection through the filtered results without leaving search.
		if m.issueCursor < len(m.visibleIssues())-1 {
			m.issueCursor++
		}
		return m, m.ensureLinksForSelectedIssue()
	case "up", "ctrl+p":
		if m.issueCursor > 0 {
			m.issueCursor--
		}
		return m, m.ensureLinksForSelectedIssue()
	}

	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	// Only reset the cursor when the query actually changed, so it stays put
	// while you move the text caret.
	if m.search.Value() != prev {
		m.issueCursor = 0
	}
	return m, cmd
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	// Allow Esc to abort the form.
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		m.form = nil
		m.setStatus("cancelled")
		return m, nil
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
		return m, m.startLoad(m.svc.SaveWorklog(editID, in))
	case huh.StateAborted:
		m.form = nil
		m.setStatus("cancelled")
		return m, nil
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
	if len(selectable) == 0 {
		m.setError(fmt.Sprintf("no valid Tempo accounts for project %s — logging blocked", req.issue.ProjectKey))
		return nil
	}

	lastUsed := m.cache.LastAccountForIssue(req.issue.ID)
	defaultKey := domain.DefaultAccountKey(projectID, m.cache.Accounts(), links)
	preselect := domain.PreselectAccount(selectable, lastUsed, defaultKey)

	defaultDate := m.weekStart.AddDate(0, 0, m.selectedDay)
	now := time.Now()
	defaultStart := domain.RoundClock(now.Hour()*60+now.Minute(), clockStepMinutes)
	m.form = newWorklogForm(req.issue, selectable, preselect, req.existing, defaultDate, defaultStart)
	m.form.form = m.form.form.WithWidth(formContentWidth(m.width))
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

// visibleIssues returns pinned-first, filtered-by-search issue list.
func (m Model) visibleIssues() []domain.Issue {
	issues := m.issues
	if m.searching || m.search.Value() != "" {
		issues = filterIssues(issues, m.search.Value())
	}
	return sortIssues(issues)
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
	return nil
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

// sortIssues puts pinned issues first, preserving relative order otherwise.
func sortIssues(issues []domain.Issue) []domain.Issue {
	pinned := make([]domain.Issue, 0, len(issues))
	rest := make([]domain.Issue, 0, len(issues))
	for _, i := range issues {
		if i.Pinned {
			pinned = append(pinned, i)
		} else {
			rest = append(rest, i)
		}
	}
	return append(pinned, rest...)
}
