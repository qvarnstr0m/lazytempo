package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazytempo/internal/domain"
)

// View renders the whole UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}

	// Form overlay takes over the main area.
	statusBar := m.renderStatusBar()
	mainHeight := m.height - lipgloss.Height(statusBar)
	if mainHeight < 3 {
		mainHeight = 3
	}

	var main string
	if m.form != nil {
		main = m.renderForm(mainHeight)
	} else if m.showHelp {
		main = m.renderFullHelp(mainHeight)
	} else {
		main = m.renderPanels(mainHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, statusBar)
}

// renderPanels lays out the three panels side by side.
func (m Model) renderPanels(height int) string {
	// Horizontal budget: account for borders/padding of three panels.
	avail := m.width
	leftW := clampW(avail * 30 / 100)
	rightW := clampW(avail * 26 / 100)
	centerW := clampW(avail - leftW - rightW - 6) // 6 ≈ borders/padding

	// Inner content height excludes the two border rows.
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}

	left := m.theme.panelStyle(m.focused == panelIssues, leftW, innerH).
		Render(m.renderIssues(leftW, innerH))
	center := m.theme.panelStyle(m.focused == panelWeek, centerW, innerH).
		Render(m.renderWeek(centerW, innerH))
	right := m.theme.panelStyle(m.focused == panelDetails, rightW, innerH).
		Render(m.renderDetails(rightW, innerH))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func clampW(w int) int {
	if w < 8 {
		return 8
	}
	return w
}

// renderIssues renders the left "My issues" panel.
func (m Model) renderIssues(width, height int) string {
	var b strings.Builder
	title := "Issues"
	if m.loading > 0 {
		title = fmt.Sprintf("Issues %s", m.spinner.View())
	}
	b.WriteString(m.theme.title(m.focused == panelIssues, title))
	b.WriteString("\n")

	if m.searching || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	vis := m.visibleIssues()
	if len(vis) == 0 {
		b.WriteString(m.theme.Muted.Render("no issues"))
		return b.String()
	}

	rows := height - 2
	start := scrollStart(m.issueCursor, len(vis), rows)
	for i := start; i < len(vis) && i < start+rows; i++ {
		iss := vis[i]
		marker := "  "
		if iss.Pinned {
			marker = m.theme.Accent.Render("★ ")
		}
		line := fmt.Sprintf("%s %s", iss.Key, iss.Summary)
		line = truncate(line, width-4)
		if i == m.issueCursor && m.focused == panelIssues {
			line = m.theme.Selected.Render("› " + line)
		} else {
			line = "  " + m.theme.Text.Render(line)
		}
		b.WriteString(marker + line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderWeek renders the center week panel: a row per day plus the selected
// day's worklogs.
func (m Model) renderWeek(width, height int) string {
	var b strings.Builder
	title := fmt.Sprintf("Week of %s", m.weekStart.Format("Jan 2"))
	b.WriteString(m.theme.title(m.focused == panelWeek, title))
	b.WriteString("\n")

	target := m.cfg.TargetSecondsPerDay()
	for i, d := range m.days {
		total := d.TotalSeconds()
		dayLabel := d.Date.Format("Mon 02")
		totalStr := domain.FormatDuration(total)

		totalStyle := m.theme.Warning
		if total >= target {
			totalStyle = m.theme.Success
		} else if total == 0 {
			totalStyle = m.theme.Muted
		}

		labelStyle := m.theme.Text
		prefix := "  "
		if i == m.selectedDay {
			prefix = m.theme.Selected.Render("› ")
			labelStyle = m.theme.Selected
		}
		row := prefix + labelStyle.Render(fmt.Sprintf("%-8s", dayLabel)) + " " + totalStyle.Render(totalStr)
		b.WriteString(row + "\n")
	}

	weekTotal := weekTotalSeconds(m.days)
	weekTarget := target * 5 // working week
	wtStyle := m.theme.Warning
	if weekTotal >= weekTarget {
		wtStyle = m.theme.Success
	}
	b.WriteString(m.theme.Muted.Render("────────────") + "\n")
	b.WriteString(m.theme.Muted.Render("Week total: ") + wtStyle.Render(domain.FormatDuration(weekTotal)) + "\n\n")

	// Worklogs for the selected day.
	wls := m.currentDayWorklogs()
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("Worklogs — %s", m.days[m.selectedDay].Date.Format("Mon 2 Jan"))) + "\n")
	if len(wls) == 0 {
		b.WriteString(m.theme.Muted.Render("  (none — press 'a' to add)"))
		return b.String()
	}
	for i, w := range wls {
		key := m.issueKeyFor(w.IssueID)

		timeStyle := m.theme.Accent
		bodyStyle := m.theme.Text
		prefix := "  "
		if i == m.worklogCursor && m.focused == panelWeek {
			prefix = m.theme.Selected.Render("› ")
			timeStyle = m.theme.Selected
			bodyStyle = m.theme.Selected
		}

		acct := m.theme.Muted.Render("[" + w.AccountKey + "]")
		if !w.HasAccount() {
			acct = m.theme.Error.Render("[NO ACCOUNT]")
		}

		body := truncate(domain.FormatDuration(w.TimeSpentSeconds)+" "+key, width-16)
		line := prefix +
			timeStyle.Render(worklogClockRange(w)) + " " +
			bodyStyle.Render(body) + " " +
			acct
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderDetails renders the right details panel for the current selection.
func (m Model) renderDetails(width, height int) string {
	var b strings.Builder
	b.WriteString(m.theme.title(m.focused == panelDetails, "Details"))
	b.WriteString("\n")

	if m.focused == panelWeek {
		if wl := m.selectedWorklog(); wl != nil {
			m.writeWorklogDetails(&b, *wl, width)
			return strings.TrimRight(b.String(), "\n")
		}
	}

	iss := m.selectedIssue()
	if iss == nil {
		b.WriteString(m.theme.Muted.Render("no selection"))
		return b.String()
	}
	b.WriteString(m.theme.Accent.Render(iss.Key) + "\n")
	b.WriteString(m.theme.Text.Render(wrap(iss.Summary, width-2)) + "\n\n")
	b.WriteString(m.theme.Muted.Render("Project: ") + m.theme.Text.Render(iss.ProjectKey) + "\n")
	b.WriteString(m.theme.Muted.Render("Status:  ") + m.theme.Text.Render(iss.Status) + "\n")
	if last := m.cache.LastAccountForIssue(iss.ID); last != "" {
		b.WriteString(m.theme.Muted.Render("Last account: ") + m.theme.Text.Render(last) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) writeWorklogDetails(b *strings.Builder, w domain.Worklog, width int) {
	b.WriteString(m.theme.Accent.Render(m.issueKeyFor(w.IssueID)) + "\n")
	b.WriteString(m.theme.Muted.Render("When:    ") + m.theme.Accent.Render(worklogClockRange(w)) + "\n")
	b.WriteString(m.theme.Muted.Render("Time:    ") + m.theme.Text.Render(domain.FormatDuration(w.TimeSpentSeconds)) + "\n")
	b.WriteString(m.theme.Muted.Render("Date:    ") + m.theme.Text.Render(w.StartDate) + "\n")
	if w.HasAccount() {
		b.WriteString(m.theme.Muted.Render("Account: ") + m.theme.Text.Render(w.AccountKey) + "\n")
	} else {
		b.WriteString(m.theme.Muted.Render("Account: ") + m.theme.Error.Render("missing!") + "\n")
	}
	if w.Description != "" {
		b.WriteString("\n" + m.theme.Text.Render(wrap(w.Description, width-2)))
	}
}

// renderForm renders the worklog form inside a bordered box.
func (m Model) renderForm(height int) string {
	title := "New worklog"
	if m.form.editID > 0 {
		title = "Edit worklog"
	}
	boxW := formBoxWidth(m.width)
	header := truncate(fmt.Sprintf("%s — %s %s", title, m.form.issue.Key, m.form.issue.Summary), boxW-4)
	body := m.form.form.View()
	content := m.theme.TitleFocused.Render(header) + "\n\n" + body
	box := m.theme.PanelFocused.Width(boxW).Render(content)
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, box)
}

// formBoxWidth is the outer width of the worklog form box.
func formBoxWidth(total int) int {
	w := total - 4
	if w > 72 {
		w = 72
	}
	if w < 24 {
		w = 24
	}
	return w
}

// formContentWidth is the width available to the form's fields inside the box
// (box minus the rounded border and horizontal padding).
func formContentWidth(total int) int {
	return formBoxWidth(total) - 4
}

// renderFullHelp renders the full keybinding help.
func (m Model) renderFullHelp(height int) string {
	m.help.ShowAll = true
	content := m.theme.TitleFocused.Render("Keybindings") + "\n\n" + m.help.View(m.keys)
	box := m.theme.PanelFocused.Width(minInt(m.width-2, 70)).Render(content)
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderStatusBar renders the bottom status bar with short help and status text.
func (m Model) renderStatusBar() string {
	m.help.ShowAll = false
	shortHelp := m.help.View(m.keys)

	status := m.status
	if status == "" {
		if m.loading > 0 {
			status = m.spinner.View() + " loading…"
		} else {
			status = m.theme.Muted.Render("ready")
		}
	} else if m.statusErr {
		status = m.theme.Error.Render(status)
	} else {
		status = m.theme.Success.Render(status)
	}

	bar := lipgloss.NewStyle().Width(m.width).Render(shortHelp)
	statusLine := lipgloss.NewStyle().Width(m.width).Render(status)
	return lipgloss.JoinVertical(lipgloss.Left, statusLine, bar)
}

// worklogClockRange renders a worklog's time span as "16:05–17:35". When the
// start time is unknown it shows just the duration bounds it can ("–").
func worklogClockRange(w domain.Worklog) string {
	start := ""
	if len(w.StartTime) >= 5 {
		if _, err := domain.ParseClock(w.StartTime[:5]); err == nil {
			start = w.StartTime[:5]
		}
	}
	if start == "" {
		return "--:--"
	}
	end := domain.AddClock(start, w.TimeSpentSeconds/60)
	return start + "–" + end
}

// issueKeyFor resolves an issue id to a human key using the loaded issues,
// falling back to the numeric id.
func (m Model) issueKeyFor(issueID string) string {
	if iss := m.issueByID(issueID); iss != nil {
		return iss.Key
	}
	return "#" + issueID
}

// --- small text helpers ---

func truncate(s string, max int) string {
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func wrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func scrollStart(cursor, total, rows int) int {
	if rows <= 0 || total <= rows {
		return 0
	}
	start := cursor - rows/2
	if start < 0 {
		start = 0
	}
	if start > total-rows {
		start = total - rows
	}
	return start
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
