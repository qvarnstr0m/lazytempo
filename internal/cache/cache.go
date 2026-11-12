// Package cache provides a small on-disk JSON cache so the TUI can show data
// immediately on startup and refresh from the API in the background.
//
// It stores issues, accounts, account links, work attributes and the
// "last used account per issue" map used for account preselection.
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lazytempo/internal/config"
	"lazytempo/internal/domain"
)

// cacheVersion is bumped whenever the on-disk format changes in a way that
// makes older data invalid. A mismatch causes the cache to be discarded and
// rebuilt from the API, so format changes heal automatically.
const cacheVersion = 2

// Data is the serialisable cache payload.
type Data struct {
	Version        int            `json:"version"`
	UpdatedAt      time.Time      `json:"updated_at"`
	AccountID      string         `json:"account_id"`
	AccountAttrKey string         `json:"account_attr_key"`
	Issues         []domain.Issue `json:"issues"`
	// WorklogIssues holds issues discovered from Tempo worklogs (keyed by issue
	// id). They are kept separate from JQL Issues so a JQL refresh does not drop
	// issues you have logged time on but that no JQL surfaces.
	WorklogIssues  map[string]domain.Issue `json:"worklog_issues"`
	Accounts       []domain.Account        `json:"accounts"`
	WorkAttributes []domain.WorkAttribute  `json:"work_attributes"`
	// AccountLinks is keyed by projectID.
	AccountLinks map[string][]domain.AccountLink `json:"account_links"`
	// LastAccountByIssue maps issueID -> last used account key.
	LastAccountByIssue map[string]string `json:"last_account_by_issue"`
	// Pinned is the set of pinned issue keys.
	Pinned map[string]bool `json:"pinned"`
}

// Cache is a goroutine-safe wrapper around Data with disk persistence.
type Cache struct {
	mu   sync.RWMutex
	path string
	data Data
}

// Dir returns the cache directory, honouring XDG_CACHE_HOME.
func Dir() (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, config.AppName), nil
}

// Open loads the cache from disk, creating an empty one when none exists.
func Open() (*Cache, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "cache.json")
	c := &Cache{
		path: path,
		data: Data{
			AccountLinks:       map[string][]domain.AccountLink{},
			WorklogIssues:      map[string]domain.Issue{},
			LastAccountByIssue: map[string]string{},
			Pinned:             map[string]bool{},
		},
	}
	if raw, err := os.ReadFile(path); err == nil {
		var d Data
		if json.Unmarshal(raw, &d) == nil {
			if d.Version == cacheVersion {
				c.data = normalize(d)
			} else {
				// Format changed: drop API-derived data (which may be in an old
				// shape) but keep user data that is still valid.
				c.data.Pinned = nonNilBool(d.Pinned)
				c.data.LastAccountByIssue = nonNilStr(d.LastAccountByIssue)
			}
		}
	}
	return c, nil
}

func nonNilBool(m map[string]bool) map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	return m
}

func nonNilStr(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func normalize(d Data) Data {
	if d.AccountLinks == nil {
		d.AccountLinks = map[string][]domain.AccountLink{}
	}
	if d.WorklogIssues == nil {
		d.WorklogIssues = map[string]domain.Issue{}
	}
	if d.LastAccountByIssue == nil {
		d.LastAccountByIssue = map[string]string{}
	}
	if d.Pinned == nil {
		d.Pinned = map[string]bool{}
	}
	return d
}

// Snapshot returns a copy of the current cached data for read-only use.
func (c *Cache) Snapshot() Data {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// Issues returns the cached issues with pins applied: the JQL issues first,
// followed by any worklog-sourced issues not already present (by id).
func (c *Cache) Issues() []domain.Issue {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]domain.Issue, 0, len(c.data.Issues)+len(c.data.WorklogIssues))
	seen := make(map[string]bool, len(c.data.Issues))
	for _, iss := range c.data.Issues {
		iss.Pinned = c.data.Pinned[iss.Key]
		out = append(out, iss)
		if iss.ID != "" {
			seen[iss.ID] = true
		}
	}
	// Append worklog-sourced issues not already in the JQL list, in a stable
	// (id-sorted) order so the list does not jump around between renders.
	extra := make([]string, 0, len(c.data.WorklogIssues))
	for id := range c.data.WorklogIssues {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		iss := c.data.WorklogIssues[id]
		iss.Pinned = c.data.Pinned[iss.Key]
		out = append(out, iss)
	}
	return out
}

// SetIssues replaces the cached JQL issue list (worklog-sourced issues are
// untouched).
func (c *Cache) SetIssues(issues []domain.Issue) {
	c.mu.Lock()
	c.data.Issues = issues
	c.data.UpdatedAt = time.Now()
	c.mu.Unlock()
}

// MergeWorklogIssues records issues discovered from Tempo worklogs so they
// appear in the list even when no JQL surfaces them.
func (c *Cache) MergeWorklogIssues(issues []domain.Issue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data.WorklogIssues == nil {
		c.data.WorklogIssues = map[string]domain.Issue{}
	}
	for _, iss := range issues {
		if iss.ID != "" {
			c.data.WorklogIssues[iss.ID] = iss
		}
	}
}

// SetReference stores accounts, work attributes and the resolved account id/key.
func (c *Cache) SetReference(accountID, attrKey string, accounts []domain.Account, attrs []domain.WorkAttribute) {
	c.mu.Lock()
	c.data.AccountID = accountID
	c.data.AccountAttrKey = attrKey
	c.data.Accounts = accounts
	c.data.WorkAttributes = attrs
	c.data.UpdatedAt = time.Now()
	c.mu.Unlock()
}

// SetAccountLinks caches the account links for a project.
func (c *Cache) SetAccountLinks(projectID string, links []domain.AccountLink) {
	c.mu.Lock()
	c.data.AccountLinks[projectID] = links
	c.mu.Unlock()
}

// AccountLinks returns the cached links for a project (possibly empty).
func (c *Cache) AccountLinks(projectID string) []domain.AccountLink {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data.AccountLinks[projectID]
}

// Accounts returns the cached accounts.
func (c *Cache) Accounts() []domain.Account {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]domain.Account, len(c.data.Accounts))
	copy(out, c.data.Accounts)
	return out
}

// LastAccountForIssue returns the last account key used for an issue.
func (c *Cache) LastAccountForIssue(issueID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data.LastAccountByIssue[issueID]
}

// SetLastAccountForIssue records the account used for an issue.
func (c *Cache) SetLastAccountForIssue(issueID, accountKey string) {
	if issueID == "" || accountKey == "" {
		return
	}
	c.mu.Lock()
	c.data.LastAccountByIssue[issueID] = accountKey
	c.mu.Unlock()
}

// TogglePin flips the pinned state of an issue key and returns the new state.
func (c *Cache) TogglePin(issueKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := !c.data.Pinned[issueKey]
	if now {
		c.data.Pinned[issueKey] = true
	} else {
		delete(c.data.Pinned, issueKey)
	}
	return now
}

// Save writes the cache to disk atomically.
func (c *Cache) Save() error {
	c.mu.RLock()
	data := c.data
	data.Version = cacheVersion
	path := c.path
	c.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
