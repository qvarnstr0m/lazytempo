package domain

import (
	"errors"
	"sort"
)

// ErrNoAccount is the validation error for a worklog that has no account, used
// when the account rule is enforced (see Validate and require_account).
var ErrNoAccount = errors.New("an account is required for every worklog")

// ErrNoTimeSpent is returned when a worklog has no positive time.
var ErrNoTimeSpent = errors.New("time spent must be greater than zero")

// ErrNoDate is returned when a worklog has no start date.
var ErrNoDate = errors.New("a start date is required")

// Validate enforces the invariants a worklog must satisfy before it is sent to
// Tempo. When requireAccount is true (the default policy), every worklog must
// have a Tempo account; set it false only for instances that don't use accounts.
func (w Worklog) Validate(requireAccount bool) error {
	if requireAccount && !w.HasAccount() {
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
// given project: those linked to the project (by account id), keeping only open
// accounts.
//
// The returned slice is sorted by account name for a stable UI ordering.
func SelectableAccounts(projectID string, accounts []Account, links []AccountLink) []Account {
	linked := map[int]bool{}
	for _, l := range links {
		if l.ProjectID == projectID {
			linked[l.AccountID] = true
		}
	}
	var out []Account
	for _, a := range accounts {
		if linked[a.ID] && a.IsSelectable() {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DefaultAccountKey returns the key of the project's default account link,
// resolving the linked account id against accounts. It returns an empty string
// when the project has no default or the account is unknown.
func DefaultAccountKey(projectID string, accounts []Account, links []AccountLink) string {
	defaultID := -1
	for _, l := range links {
		if l.ProjectID == projectID && l.Default {
			defaultID = l.AccountID
			break
		}
	}
	if defaultID < 0 {
		return ""
	}
	for _, a := range accounts {
		if a.ID == defaultID {
			return a.Key
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
