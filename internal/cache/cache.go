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
	"sync"
	"time"

	"lazytempo/internal/config"
	"lazytempo/internal/domain"
)

// Data is the serialisable cache payload.
type Data struct {
	UpdatedAt      time.Time              `json:"updated_at"`
	AccountID      string                 `json:"account_id"`
	AccountAttrKey string                 `json:"account_attr_key"`
	Issues         []domain.Issue         `json:"issues"`
	Accounts       []domain.Account       `json:"accounts"`
	WorkAttributes []domain.WorkAttribute `json:"work_attributes"`
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
			LastAccountByIssue: map[string]string{},
			Pinned:             map[string]bool{},
		},
	}
	if raw, err := os.ReadFile(path); err == nil {
		var d Data
		if json.Unmarshal(raw, &d) == nil {
			c.data = normalize(d)
		}
	}
	return c, nil
}

func normalize(d Data) Data {
	if d.AccountLinks == nil {
		d.AccountLinks = map[string][]domain.AccountLink{}
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

// Issues returns the cached issues, with pins applied.
func (c *Cache) Issues() []domain.Issue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]domain.Issue, len(c.data.Issues))
	copy(out, c.data.Issues)
	for i := range out {
		out[i].Pinned = c.data.Pinned[out[i].Key]
	}
	return out
}

// SetIssues replaces the cached issue list.
func (c *Cache) SetIssues(issues []domain.Issue) {
	c.mu.Lock()
	c.data.Issues = issues
	c.data.UpdatedAt = time.Now()
	c.mu.Unlock()
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
