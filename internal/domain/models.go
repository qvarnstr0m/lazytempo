// Package domain contains the core models and business rules for lazytempo.
//
// It is deliberately free of any dependency on the UI, the HTTP clients or the
// cache so that the rules (time parsing, the mandatory-account rule, account
// preselection) can be unit tested in isolation.
package domain

import "time"

// Issue is a Jira issue, reduced to what the TUI needs.
//
// Tempo works with the numeric IssueID and ProjectID rather than the human
// readable Key, so both are kept.
type Issue struct {
	ID         string // numeric Jira issue id, e.g. "10991"
	Key        string // human key, e.g. "PROJ-123"
	Summary    string
	ProjectID  string // numeric project id
	ProjectKey string
	Status     string
	Pinned     bool
}

// Account is a Tempo account that time can be booked against.
type Account struct {
	Key    string // the account key used in worklog attributes, e.g. "CORE-2024"
	ID     int
	Name   string
	Status string // "OPEN", "CLOSED", "ARCHIVED"
}

// IsSelectable reports whether the account may be chosen for a new worklog.
// Only open accounts are selectable.
func (a Account) IsSelectable() bool {
	return a.Status == "" || a.Status == "OPEN"
}

// AccountLink ties an account to a project and records whether it is the
// project's default account.
//
// Tempo's account-links endpoint identifies the account by its numeric id
// (not its key), so the link carries AccountID and the key is resolved against
// the account list when needed.
type AccountLink struct {
	AccountID int
	ProjectID string
	Default   bool
}

// WorkAttribute describes a Tempo work attribute. The account attribute has
// type "ACCOUNT"; its key is read dynamically rather than hardcoded.
type WorkAttribute struct {
	Key  string
	Name string
	Type string // e.g. "ACCOUNT"
}

// Worklog is a single Tempo worklog entry.
type Worklog struct {
	ID               int
	IssueID          string
	IssueKey         string
	AuthorAccountID  string
	TimeSpentSeconds int
	StartDate        string // "YYYY-MM-DD"
	StartTime        string // "HH:MM:SS"
	Description      string
	AccountKey       string // the ACCOUNT attribute value; empty means "no account"
}

// HasAccount reports whether the worklog has a Tempo account attached.
// A worklog without an account must never be submitted (see Validate) and is
// highlighted in the week view.
func (w Worklog) HasAccount() bool {
	return w.AccountKey != ""
}

// Day groups the worklogs logged on a single calendar day.
type Day struct {
	Date     time.Time
	Worklogs []Worklog
}

// TotalSeconds returns the sum of time logged on the day.
func (d Day) TotalSeconds() int {
	total := 0
	for _, w := range d.Worklogs {
		total += w.TimeSpentSeconds
	}
	return total
}

// AccountAttributeKey finds the key of the ACCOUNT-typed work attribute.
// Tempo commonly uses "_Account_" but installations can differ, so the key is
// resolved from the work attribute list rather than hardcoded. Returns an empty
// string when no ACCOUNT attribute exists.
func AccountAttributeKey(attrs []WorkAttribute) string {
	for _, a := range attrs {
		if a.Type == "ACCOUNT" {
			return a.Key
		}
	}
	return ""
}
