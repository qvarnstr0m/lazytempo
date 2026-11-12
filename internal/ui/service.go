package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazytempo/internal/cache"
	"lazytempo/internal/config"
	"lazytempo/internal/domain"
	"lazytempo/internal/jira"
	"lazytempo/internal/tempo"
)

// Service bundles the API clients and cache and exposes Bubble Tea commands.
// The clients are interfaces, so the UI can be tested with mocks.
type Service struct {
	cfg       config.Config
	jira      jira.Client
	tempo     tempo.Client
	cache     *cache.Cache
	accountID string
}

// NewService builds a Service.
func NewService(cfg config.Config, j jira.Client, tp tempo.Client, c *cache.Cache) *Service {
	return &Service{cfg: cfg, jira: j, tempo: tp, cache: c}
}

const apiTimeout = 30 * time.Second

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), apiTimeout)
}

// --- Messages ---

type referenceLoadedMsg struct {
	accountID   string
	displayName string
	attrKey     string
	err         error
}

type issuesLoadedMsg struct {
	issues []domain.Issue
	err    error
}

type worklogsLoadedMsg struct {
	from     time.Time
	worklogs []domain.Worklog
	err      error
}

// worklogIssuesLoadedMsg carries issues resolved from Tempo worklog issue ids.
type worklogIssuesLoadedMsg struct {
	issues []domain.Issue
	err    error
}

type accountLinksLoadedMsg struct {
	projectID string
	err       error
}

type worklogSavedMsg struct {
	issueID    string
	accountKey string
	err        error
}

type worklogDeletedMsg struct {
	err error
}

type statusMsg struct {
	text    string
	isError bool
}

// --- Commands ---

// LoadReference fetches the Jira accountId, work attributes (to resolve the
// ACCOUNT key) and accounts, caching them. It runs once at startup.
func (s *Service) LoadReference() tea.Cmd {
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()

		self, err := s.jira.Myself(c)
		if err != nil {
			return referenceLoadedMsg{err: err}
		}

		attrs, err := s.tempo.WorkAttributes(c)
		if err != nil {
			return referenceLoadedMsg{err: err}
		}
		attrKey := domain.AccountAttributeKey(attrs)
		if attrKey != "" {
			s.tempo.SetAccountAttributeKey(attrKey)
		}

		accounts, err := s.tempo.Accounts(c)
		if err != nil {
			return referenceLoadedMsg{err: err}
		}

		s.cache.SetReference(self.AccountID, attrKey, accounts, attrs)
		return referenceLoadedMsg{accountID: self.AccountID, displayName: self.DisplayName, attrKey: attrKey}
	}
}

// SetAccountID records the resolved account id for later worklog queries.
func (s *Service) SetAccountID(id string) {
	s.accountID = id
}

// LoadIssues fetches issues from Jira using the configured JQL and caches them.
func (s *Service) LoadIssues() tea.Cmd {
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		issues, err := s.jira.SearchIssues(c, s.cfg.MyIssuesJQL, 50)
		if err != nil {
			return issuesLoadedMsg{err: err}
		}
		s.cache.SetIssues(issues)
		return issuesLoadedMsg{issues: s.cache.Issues()}
	}
}

// LoadWorklogs fetches the user's worklogs for the week containing weekStart.
func (s *Service) LoadWorklogs(weekStart time.Time) tea.Cmd {
	return func() tea.Msg {
		if s.accountID == "" {
			return worklogsLoadedMsg{from: weekStart} // nothing to load yet
		}
		c, cancel := ctx()
		defer cancel()
		from := weekStart.Format(dateLayout)
		to := weekStart.AddDate(0, 0, 6).Format(dateLayout)
		worklogs, err := s.tempo.Worklogs(c, s.accountID, from, to)
		if err != nil {
			return worklogsLoadedMsg{from: weekStart, err: err}
		}
		return worklogsLoadedMsg{from: weekStart, worklogs: worklogs}
	}
}

// SeedIssuesFromWorklogs fetches the user's Tempo worklogs over a date range,
// collects the distinct issue ids, resolves them to Jira issues and merges them
// into the issue list. This surfaces issues you have logged time on even when
// Jira JQL cannot find them (e.g. Tempo worklogs not synced to Jira).
func (s *Service) SeedIssuesFromWorklogs(from, to string) tea.Cmd {
	return func() tea.Msg {
		if s.accountID == "" {
			return worklogIssuesLoadedMsg{}
		}
		c, cancel := ctx()
		defer cancel()
		worklogs, err := s.tempo.Worklogs(c, s.accountID, from, to)
		if err != nil {
			return worklogIssuesLoadedMsg{err: err}
		}
		return s.resolveAndMerge(c, distinctIssueIDs(worklogs))
	}
}

// ResolveIssues resolves the given issue ids to Jira issues and merges them.
func (s *Service) ResolveIssues(ids []string) tea.Cmd {
	return func() tea.Msg {
		if len(ids) == 0 {
			return worklogIssuesLoadedMsg{}
		}
		c, cancel := ctx()
		defer cancel()
		return s.resolveAndMerge(c, ids)
	}
}

func (s *Service) resolveAndMerge(c context.Context, ids []string) tea.Msg {
	if len(ids) == 0 {
		return worklogIssuesLoadedMsg{}
	}
	issues, err := s.jira.IssuesByIDs(c, ids)
	if err != nil {
		return worklogIssuesLoadedMsg{err: err}
	}
	s.cache.MergeWorklogIssues(issues)
	return worklogIssuesLoadedMsg{issues: issues}
}

func distinctIssueIDs(worklogs []domain.Worklog) []string {
	seen := map[string]bool{}
	var ids []string
	for _, w := range worklogs {
		if w.IssueID != "" && !seen[w.IssueID] {
			seen[w.IssueID] = true
			ids = append(ids, w.IssueID)
		}
	}
	return ids
}

// remoteIssuesLoadedMsg carries the result of a direct Jira search run when the
// locally loaded issues do not match the search query.
type remoteIssuesLoadedMsg struct {
	query  string
	issues []domain.Issue
	err    error
}

// remoteSearchLimit caps how many issues a fallback Jira search returns.
const remoteSearchLimit = 25

var issueKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[0-9]+$`)

// buildSearchJQL turns a free-text query into JQL. A key-looking query matches
// that issue exactly; anything else is a text search ordered by recency. Jira
// only returns issues the account may view.
func buildSearchJQL(query string) string {
	q := strings.TrimSpace(query)
	if issueKeyRe.MatchString(q) {
		return fmt.Sprintf("issuekey = %q", strings.ToUpper(q))
	}
	return fmt.Sprintf("text ~ %q ORDER BY updated DESC", q)
}

// SearchRemote searches Jira directly for issues matching the query, capped at
// remoteSearchLimit. It is used as a fallback when nothing loaded matches.
func (s *Service) SearchRemote(query string) tea.Cmd {
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		issues, err := s.jira.SearchIssues(c, buildSearchJQL(query), remoteSearchLimit)
		return remoteIssuesLoadedMsg{query: query, issues: issues, err: err}
	}
}

// LoadAccountLinks fetches and caches the account links for a project.
func (s *Service) LoadAccountLinks(projectID string) tea.Cmd {
	return func() tea.Msg {
		if projectID == "" {
			return accountLinksLoadedMsg{projectID: projectID}
		}
		if links := s.cache.AccountLinks(projectID); len(links) > 0 {
			return accountLinksLoadedMsg{projectID: projectID}
		}
		c, cancel := ctx()
		defer cancel()
		links, err := s.tempo.AccountLinks(c, projectID)
		if err != nil {
			return accountLinksLoadedMsg{projectID: projectID, err: err}
		}
		s.cache.SetAccountLinks(projectID, links)
		return accountLinksLoadedMsg{projectID: projectID}
	}
}

// SaveWorklog creates or updates a worklog. The account is mandatory and the
// input is validated before any request is made.
func (s *Service) SaveWorklog(editID int, in tempo.WorklogInput) tea.Cmd {
	return func() tea.Msg {
		wl := domain.Worklog{
			IssueID:          in.IssueID,
			TimeSpentSeconds: in.TimeSpentSeconds,
			StartDate:        in.StartDate,
			AccountKey:       in.AccountKey,
		}
		if err := wl.Validate(); err != nil {
			return worklogSavedMsg{err: err}
		}
		c, cancel := ctx()
		defer cancel()
		var err error
		if editID > 0 {
			_, err = s.tempo.UpdateWorklog(c, editID, in)
		} else {
			_, err = s.tempo.CreateWorklog(c, in)
		}
		if err != nil {
			return worklogSavedMsg{err: err}
		}
		s.cache.SetLastAccountForIssue(in.IssueID, in.AccountKey)
		return worklogSavedMsg{issueID: in.IssueID, accountKey: in.AccountKey}
	}
}

// DeleteWorklog removes a worklog.
func (s *Service) DeleteWorklog(id int) tea.Cmd {
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		if err := s.tempo.DeleteWorklog(c, id); err != nil {
			return worklogDeletedMsg{err: err}
		}
		return worklogDeletedMsg{}
	}
}
