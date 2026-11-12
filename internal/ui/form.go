package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"

	"lazytempo/internal/domain"
	"lazytempo/internal/tempo"
)

// worklogForm wraps a Huh form for creating or editing a worklog. The account
// field is mandatory: the form cannot be submitted without a selected account.
type worklogForm struct {
	form   *huh.Form
	issue  domain.Issue
	editID int // >0 when editing an existing worklog

	// Bound values.
	accountKey string
	timeStr    string
	dateStr    string
	desc       string

	accounts []domain.Account
}

// newWorklogForm builds a form for the given issue. selectable is the list of
// accounts valid for the issue's project; preselect is the preselected account
// key (may be empty). existing is non-nil when editing.
func newWorklogForm(issue domain.Issue, selectable []domain.Account, preselect string, existing *domain.Worklog, defaultDate time.Time) *worklogForm {
	f := &worklogForm{
		issue:      issue,
		accounts:   selectable,
		accountKey: preselect,
		dateStr:    defaultDate.Format(dateLayout),
	}

	if existing != nil {
		f.editID = existing.ID
		f.accountKey = existing.AccountKey
		f.timeStr = domain.FormatDuration(existing.TimeSpentSeconds)
		if existing.StartDate != "" {
			f.dateStr = existing.StartDate
		}
		f.desc = existing.Description
	}

	options := make([]huh.Option[string], 0, len(selectable))
	for _, a := range selectable {
		label := a.Name
		if label == "" {
			label = a.Key
		}
		options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", label, a.Key), a.Key))
	}

	accountField := huh.NewSelect[string]().
		Key("account").
		Title("Account (required)").
		Description("Every worklog must have a Tempo account.").
		Options(options...).
		Value(&f.accountKey).
		Validate(func(s string) error {
			if s == "" {
				return domain.ErrNoAccount
			}
			return nil
		})

	timeField := huh.NewInput().
		Key("time").
		Title("Time spent").
		Placeholder("1h30m, 1.5h, 90m, 1:30").
		Value(&f.timeStr).
		Validate(func(s string) error {
			_, err := domain.ParseDuration(s)
			return err
		})

	dateField := huh.NewInput().
		Key("date").
		Title("Date").
		Placeholder(dateLayout).
		Value(&f.dateStr).
		Validate(func(s string) error {
			if _, err := time.Parse(dateLayout, s); err != nil {
				return fmt.Errorf("use format YYYY-MM-DD")
			}
			return nil
		})

	descField := huh.NewText().
		Key("description").
		Title("Description").
		Value(&f.desc).
		Lines(2)

	f.form = huh.NewForm(
		huh.NewGroup(accountField, timeField, dateField, descField),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(true)

	return f
}

// input converts the completed form into a tempo.WorklogInput.
func (f *worklogForm) input() (tempo.WorklogInput, error) {
	seconds, err := domain.ParseDuration(f.timeStr)
	if err != nil {
		return tempo.WorklogInput{}, err
	}
	if f.accountKey == "" {
		return tempo.WorklogInput{}, domain.ErrNoAccount
	}
	return tempo.WorklogInput{
		IssueID:          f.issue.ID,
		TimeSpentSeconds: seconds,
		StartDate:        f.dateStr,
		Description:      f.desc,
		AccountKey:       f.accountKey,
	}, nil
}
