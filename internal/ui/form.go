package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"

	"lazytempo/internal/domain"
	"lazytempo/internal/tempo"
)

// clockStepMinutes is the granularity of the start/end time dropdowns.
const clockStepMinutes = 5

// accountSelectHeight is the number of account rows visible before the dropdown
// scrolls.
const accountSelectHeight = 8

// worklogForm wraps a Huh form for creating or editing a worklog. The account
// field is required unless require_account is disabled.
//
// Time is entered as start and end clock times (5-minute steps, 24-hour
// European format, e.g. 16:05); the duration is derived from them.
type worklogForm struct {
	form   *huh.Form
	issue  domain.Issue
	editID int // >0 when editing an existing worklog

	// Bound values.
	accountKey string
	startStr   string
	endStr     string
	dateStr    string
	desc       string

	accounts       []domain.Account
	requireAccount bool
	usStyle        bool // 12-hour time + MM/DD/YYYY dates
}

// newWorklogForm builds a form for the given issue. selectable is the list of
// accounts valid for the issue's project; preselect is the preselected account
// key (may be empty). existing is non-nil when editing. defaultStart is the
// preselected start time ("HH:MM") for a new worklog; the end defaults to one
// hour later (clamped to the end of the day).
func newWorklogForm(issue domain.Issue, selectable []domain.Account, preselect string, existing *domain.Worklog, defaultDate time.Time, defaultStart string, requireAccount, usStyle bool) *worklogForm {
	// Default to the given start and one step (5 min) later, both snapped to the
	// dropdown grid (a Select cannot display a value absent from its options).
	start := snapToStep(defaultStart)
	end := snapToStep(domain.AddClock(start, clockStepMinutes))
	if _, err := domain.SecondsBetween(start, end); err != nil {
		// Too close to midnight for a later end on the grid; use the last slot.
		start, end = "23:50", "23:55"
	}
	f := &worklogForm{
		issue:      issue,
		accounts:   selectable,
		accountKey: preselect,
		usStyle:    usStyle,
		dateStr:    domain.FormatDateDisplay(defaultDate.Format(dateLayout), usStyle),
		startStr:   start,
		endStr:     end,
	}

	if existing != nil {
		f.editID = existing.ID
		f.accountKey = existing.AccountKey
		if existing.StartDate != "" {
			f.dateStr = domain.FormatDateDisplay(existing.StartDate, usStyle)
		}
		// Derive start/end from the existing worklog's start time + duration,
		// snapping the start to the dropdown grid.
		start := clockFromStartTime(existing.StartTime, "09:00")
		f.startStr = snapToStep(start)
		// Snap the end onto the dropdown grid too (a Select cannot show a value
		// absent from its options); never let it collapse onto the start.
		end := snapToStep(domain.AddClock(f.startStr, existing.TimeSpentSeconds/60))
		if _, err := domain.SecondsBetween(f.startStr, end); err != nil {
			end = domain.AddClock(f.startStr, clockStepMinutes)
		}
		f.endStr = end
		f.desc = existing.Description
	}

	f.requireAccount = requireAccount
	accountOptions := make([]huh.Option[string], 0, len(selectable)+1)
	if !requireAccount {
		// Allow choosing no account when the rule is relaxed.
		accountOptions = append(accountOptions, huh.NewOption("(no account)", ""))
	}
	// Put the preselected account first. Huh scrolls the selected option to the
	// top of its viewport (and does not clamp it on first render), so a
	// mid-list preselection would hide the accounts above it until a key is
	// pressed; leading with it keeps the selected index at the top.
	ordered := make([]domain.Account, 0, len(selectable))
	for _, a := range selectable {
		if a.Key == preselect {
			ordered = append(ordered, a)
		}
	}
	for _, a := range selectable {
		if a.Key != preselect {
			ordered = append(ordered, a)
		}
	}
	for _, a := range ordered {
		label := a.Name
		if label == "" {
			label = a.Key
		}
		accountOptions = append(accountOptions, huh.NewOption(fmt.Sprintf("%s (%s)", label, a.Key), a.Key))
	}

	clockOptions := clockSelectOptions(usStyle)

	accountTitle := "Account (required)"
	accountDesc := "Every worklog must have a Tempo account."
	if !requireAccount {
		accountTitle = "Account (optional)"
		accountDesc = "Choose an account, or (no account)."
	}
	// When the list is longer than the viewport, signal that there are more rows
	// (Huh's Select has no built-in above/below indicator).
	if len(accountOptions) > accountSelectHeight {
		accountDesc = fmt.Sprintf("%d accounts — ↑/↓ to scroll, type to filter", len(accountOptions))
	}
	accountField := huh.NewSelect[string]().
		Key("account").
		Title(accountTitle).
		Description(accountDesc).
		Options(accountOptions...).
		Filtering(true).
		Height(accountSelectHeight).
		Value(&f.accountKey).
		Validate(func(s string) error {
			if requireAccount && s == "" {
				return domain.ErrNoAccount
			}
			return nil
		})

	startField := huh.NewSelect[string]().
		Key("start").
		Title("Start").
		Options(clockOptions...).
		Filtering(true).
		Height(8).
		Value(&f.startStr)

	endField := huh.NewSelect[string]().
		Key("end").
		Title("End").
		Options(clockOptions...).
		Filtering(true).
		Height(8).
		Value(&f.endStr).
		Validate(func(end string) error {
			_, err := domain.SecondsBetween(f.startStr, end)
			return err
		})

	dateField := huh.NewInput().
		Key("date").
		Title("Date").
		Placeholder(domain.DateHint(usStyle)).
		Value(&f.dateStr).
		Validate(func(s string) error {
			_, err := domain.ParseDateDisplay(s, usStyle)
			return err
		})

	descField := huh.NewText().
		Key("description").
		Title("Description").
		Value(&f.desc).
		Lines(2)

	f.form = huh.NewForm(
		huh.NewGroup(accountField, startField, endField, dateField, descField),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(true)

	return f
}

// clockSelectOptions builds the 5-minute dropdown options. The option value is
// always the canonical 24-hour "HH:MM"; only the displayed label follows the
// hour12 (US) / 24-hour (EU) preference.
func clockSelectOptions(hour12 bool) []huh.Option[string] {
	values := domain.TimeOptions(clockStepMinutes)
	opts := make([]huh.Option[string], 0, len(values))
	for _, v := range values {
		opts = append(opts, huh.NewOption(domain.FormatClockDisplay(v, hour12), v))
	}
	return opts
}

// clockFromStartTime takes a Tempo start time ("HH:MM:SS" or "HH:MM") and
// returns an "HH:MM" clock, falling back to fallback on parse failure.
func clockFromStartTime(startTime, fallback string) string {
	if len(startTime) >= 5 {
		if _, err := domain.ParseClock(startTime[:5]); err == nil {
			return startTime[:5]
		}
	}
	return fallback
}

// snapToStep rounds an "HH:MM" clock down to the dropdown grid so it matches an
// existing option (a Select cannot show a value absent from its options).
func snapToStep(clock string) string {
	m, err := domain.ParseClock(clock)
	if err != nil {
		return "09:00"
	}
	m -= m % clockStepMinutes
	return domain.AddClock("00:00", m)
}

// input converts the completed form into a tempo.WorklogInput. The duration is
// computed from the start and end clock times.
func (f *worklogForm) input() (tempo.WorklogInput, error) {
	if f.requireAccount && f.accountKey == "" {
		return tempo.WorklogInput{}, domain.ErrNoAccount
	}
	seconds, err := domain.SecondsBetween(f.startStr, f.endStr)
	if err != nil {
		return tempo.WorklogInput{}, err
	}
	isoDate, err := domain.ParseDateDisplay(f.dateStr, f.usStyle)
	if err != nil {
		return tempo.WorklogInput{}, err
	}
	return tempo.WorklogInput{
		IssueID:                  f.issue.ID,
		TimeSpentSeconds:         seconds,
		StartDate:                isoDate,
		StartTime:                f.startStr + ":00",
		Description:              f.desc,
		AccountKey:               f.accountKey,
		RemainingEstimateSeconds: f.issue.RemainingEstimateSeconds,
	}, nil
}
