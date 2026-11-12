package domain

import (
	"errors"
	"sort"
)

// ErrNoAccount is the validation error for a worklog that has no account.
// Every worklog MUST have a Tempo account attached; one without must never be
// submitted.
var ErrNoAccount = errors.New("an account is required for every worklog")

// ErrNoTimeSpent is returned when a worklog has no positive time.
var ErrNoTimeSpent = errors.New("time spent must be greater than zero")

// ErrNoDate is returned when a worklog has no start date.
var ErrNoDate = errors.New("a start date is required")

// Validate enforces the invariants a worklog must satisfy before it is sent to
// Tempo. The account rule is the central one.
func (w Worklog) Validate() error {
	if !w.HasAccount() {
		return ErrNoAccount
	}
	if w.TimeSpentSeconds <= 0 {
		return ErrNoTimeSpent
	}
	if w.StartDate == "" {
		return ErrNoDate
	}
	return nil
}

// SelectableAccounts returns the accounts that may be chosen for an issue in the
// given project: those linked to the project, keeping only open accounts.
//
// The returned slice is sorted by account name for a stable UI ordering.
func SelectableAccounts(projectID string, accounts []Account, links []AccountLink) []Account {
	linked := map[string]bool{}
	for _, l := range links {
		if l.ProjectID == projectID {
			linked[l.AccountKey] = true
		}
	}
	var out []Account
	for _, a := range accounts {
		if linked[a.Key] && a.IsSelectable() {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DefaultAccountKey returns the key of the project's default account link, or an
// empty string when the project has no default.
func DefaultAccountKey(projectID string, links []AccountLink) string {
	for _, l := range links {
		if l.ProjectID == projectID && l.Default {
			return l.AccountKey
		}
	}
	return ""
}

// PreselectAccount implements the account preselection rule, in order:
//
//  1. The last account used for that specific issue (lastUsedKey, from cache).
//  2. The project's default account link.
//  3. Otherwise no preselection — the user must choose.
//
// A candidate is only honored when it is among the currently selectable
// accounts; a stale "last used" account that is no longer valid is skipped so
// the next rule can apply. It returns the chosen account key, or "" when there
// is no valid preselection.
func PreselectAccount(selectable []Account, lastUsedKey, defaultKey string) string {
	valid := map[string]bool{}
	for _, a := range selectable {
		valid[a.Key] = true
	}
	if lastUsedKey != "" && valid[lastUsedKey] {
		return lastUsedKey
	}
	if defaultKey != "" && valid[defaultKey] {
		return defaultKey
	}
	return ""
}
