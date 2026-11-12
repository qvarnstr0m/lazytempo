package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazytempo/internal/domain"
)

// View renders the whole UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}

	// Form overlay takes over the main area.
	header := m.renderHeader()
	statusBar := m.renderStatusBar()
	mainHeight := m.height - lipgloss.Height(statusBar) - lipgloss.Height(header)
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

	return lipgloss.JoinVertical(lipgloss.Left, header, main, statusBar)
}

// renderPanels lays out two columns: a left column with Issues (top ~2/3) over
// Details (bottom ~1/3), and a right column with the Week panel.
func (m Model) renderPanels(height int) string {
	avail := m.width
	// Each box adds 2 (border) to its content width; two columns => -4.
	leftW := clampW(avail * 2 / 5)
	rightW := clampW(avail - leftW - 4)

	// Left column: the two stacked boxes contribute 4 border rows in total.
	contentH := height - 4
	if contentH < 4 {
		contentH = 4
	}
	issuesInnerH := contentH * 2 / 3
	if issuesInnerH < 3 {
		issuesInnerH = 3
	}
	detailsInnerH := contentH - issuesInnerH
	if detailsInnerH < 1 {
		detailsInnerH = 1
	}

	// Right column: a single box contributes 2 border rows.
	weekInnerH := height - 2
	if weekInnerH < 1 {
		weekInnerH = 1
	}

	// Text width inside a box excludes its horizontal padding (1 col each side).
	leftTextW := leftW - 2
	rightTextW := rightW - 2

	issues := m.theme.panelStyle(m.focused == panelIssues, leftW, issuesInnerH).
		Render(clipBox(m.renderIssues(leftTextW, issuesInnerH), leftTextW, issuesInnerH))
	details := m.theme.panelStyle(m.focused == panelDetails, leftW, detailsInnerH).
		Render(clipBox(m.renderDetails(leftTextW, detailsInnerH), leftTextW, detailsInnerH))
	leftCol := lipgloss.JoinVertical(lipgloss.Left, issues, details)

	week := m.theme.panelStyle(m.focused == panelWeek, rightW, weekInnerH).
		Render(clipBox(m.renderWeek(rightTextW, weekInnerH), rightTextW, weekInnerH))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, week)
}

func clampW(w int) int {
	if w < 8 {
		return 8
	}
	return w
}

// clipBox constrains panel content to its box: each line is truncated (ANSI
// aware, so styling is preserved) to width display columns so it never wraps,
// and at most height lines are kept. Without this, a line wider than the box
// would wrap into extra rows, making the box taller than intended and pushing
// the whole frame off the top of the terminal.
func clipBox(s string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > width {
			lines[i] = ansi.Truncate(l, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

// renderIssues renders the left "My issues" panel.
func (m Model) renderIssues(width, height int) string {
	var b strings.Builder
	vis := m.visibleIssues()

	title := "Issues"
	if m.search.Value() != "" {
		title = fmt.Sprintf("Issues (%d/%d)", len(vis), len(m.issues))
	}
	if m.loading > 0 {
		title = fmt.Sprintf("%s %s", title, m.spinner.View())
	}
	b.WriteString(m.theme.title(m.focused == panelIssues, title))
	b.WriteString("\n")

	if m.searching || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n")
		if m.searching {
			b.WriteString(m.theme.Muted.Render("↓/tab: select · esc: clear") + "\n")
		}
	}

	if len(vis) == 0 {
		msg := "no issues"
		if m.search.Value() != "" {
			msg = "no matches for " + strconv.Quote(m.search.Value())
		}
		b.WriteString(m.theme.Muted.Render(msg))
		return b.String()
	}

	// Rows available for issues = panel height minus the header lines already
	// written (title, and the search box + hint when searching).
	header := 1
	if m.searching || m.search.Value() != "" {
		header++
	}
	if m.searching {
		header++
	}
	rows := height - header
	// Reserve a line for the "more below/above" scroll indicator when the list
	// does not fit.
	if len(vis) > rows {
		rows--
	}
	if rows < 1 {
		rows = 1
	}

	start := scrollStart(m.issueCursor, len(vis), rows)
	for i := start; i < len(vis) && i < start+rows; i++ {
		iss := vis[i]
		marker := "  "
		if iss.Pinned {
			marker = m.theme.Accent.Render("★ ")
		}
		line := fmt.Sprintf("%s %s", iss.Key, iss.Summary)
		line = truncate(line, width-4)
		switch {
		case i == m.issueCursor && m.focused == panelIssues:
			line = m.theme.Selected.Render("› " + line)
		case i == m.issueCursor:
			// Keep the selection visible (dimmer) when the panel is not focused.
			line = m.theme.SelectedBlurred.Render("› " + line)
		default:
			line = "  " + m.theme.Text.Render(line)
		}
		b.WriteString(marker + line + "\n")
	}
	if len(vis) > rows {
		above := start
		below := len(vis) - (start + rows)
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  — %d above, %d below —", above, below)))
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
	const gaugeCells = 12
	for i, d := range m.days {
		total := d.TotalSeconds()
		dayLabel := d.Date.Format("Mon 02")
		weekend := d.Date.Weekday() == time.Saturday || d.Date.Weekday() == time.Sunday

		// Gauge and over/under against the daily target (weekends have no target).
		var gauge, delta string
		totalStyle := m.theme.Muted
		if weekend {
			gauge = m.theme.Muted.Render(strings.Repeat("·", gaugeCells))
			if total > 0 {
				totalStyle = m.theme.Text
			}
		} else {
			gauge = m.renderGauge(total, target, gaugeCells)
			switch diff := total - target; {
			case total == 0:
				totalStyle = m.theme.Muted
				delta = m.theme.Muted.Render("-" + domain.FormatDuration(target))
			case diff >= 0:
				totalStyle = m.theme.Success
				if diff == 0 {
					delta = m.theme.Success.Render("✓")
				} else {
					delta = m.theme.Success.Render("+" + domain.FormatDuration(diff))
				}
			default:
				totalStyle = m.theme.Warning
				delta = m.theme.Muted.Render("-" + domain.FormatDuration(-diff))
			}
		}

		labelStyle := m.theme.Text
		prefix := "  "
		if i == m.selectedDay {
			prefix = m.theme.Selected.Render("› ")
			labelStyle = m.theme.Selected
		}
		row := prefix +
			labelStyle.Render(fmt.Sprintf("%-7s", dayLabel)) + " " +
			gauge + " " +
			totalStyle.Render(fmt.Sprintf("%5s", domain.FormatDuration(total))) + " " +
			delta
		b.WriteString(strings.TrimRight(row, " ") + "\n")
	}

	weekTotal := weekTotalSeconds(m.days)
	weekTarget := target * 5 // working week
	wtStyle := m.theme.Warning
	if weekTotal >= weekTarget {
		wtStyle = m.theme.Success
	}
	b.WriteString("\n")
	b.WriteString("  " + m.theme.Muted.Render(fmt.Sprintf("%-7s", "Week")) + " " +
		m.renderGauge(weekTotal, weekTarget, gaugeCells) + " " +
		wtStyle.Render(fmt.Sprintf("%5s", domain.FormatDuration(weekTotal))) +
		m.theme.Muted.Render(" / "+domain.FormatDuration(weekTarget)) + "\n\n")

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
		if i == m.worklogCursor {
			sel := m.theme.Selected
			if m.focused != panelWeek {
				sel = m.theme.SelectedBlurred
			}
			prefix = sel.Render("› ")
			timeStyle = sel
			bodyStyle = sel
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
	content := m.theme.TitleFocused.Render("Keybindings") + "\n\n" +
		m.help.View(m.keys) + "\n\n" +
		m.theme.Muted.Render("press ? or esc to return")
	box := m.theme.PanelFocused.Width(minInt(m.width-2, 70)).Render(content)
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderHeader renders the top strip: brand, user, the week range, the week
// total against target with a small gauge, and a sync indicator on the right.
func (m Model) renderHeader() string {
	sep := m.theme.Muted.Render(" · ")

	parts := []string{lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("lazytempo")}
	if m.userName != "" {
		parts = append(parts, m.theme.Accent.Render(m.userName))
	}
	parts = append(parts, m.theme.Text.Render(m.weekLabel()))

	total := weekTotalSeconds(m.days)
	target := m.cfg.TargetSecondsPerDay() * 5 // a working week
	gauge := m.renderGauge(total, target, 10)
	totalStr := gauge + " " +
		m.theme.Text.Render(domain.FormatDuration(total)) +
		m.theme.Muted.Render(" / "+domain.FormatDuration(target))
	parts = append(parts, totalStr)

	left := strings.Join(parts, sep)
	right := m.theme.Muted.Render(m.syncLabel())

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return clipBox(left+strings.Repeat(" ", gap)+right, m.width, 1)
}

// weekLabel renders the ISO week number and the Mon–Sun date range.
func (m Model) weekLabel() string {
	start := m.weekStart
	end := start.AddDate(0, 0, 6)
	_, wk := start.ISOWeek()
	var rng string
	if start.Month() == end.Month() {
		rng = fmt.Sprintf("%s %d–%d", start.Format("Jan"), start.Day(), end.Day())
	} else {
		rng = fmt.Sprintf("%s – %s", start.Format("Jan 2"), end.Format("Jan 2"))
	}
	return fmt.Sprintf("Week %d · %s", wk, rng)
}

// syncLabel describes the last successful refresh.
func (m Model) syncLabel() string {
	if m.loading > 0 {
		return m.spinner.View() + " syncing…"
	}
	if m.lastSync.IsZero() {
		return ""
	}
	d := time.Since(m.lastSync)
	switch {
	case d < time.Minute:
		return "synced just now"
	case d < time.Hour:
		return fmt.Sprintf("synced %dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("synced %dh ago", int(d.Hours()))
	}
}

// renderGauge draws a block-character progress bar of done/target over `cells`
// columns: the fill is green when the target is met, otherwise the pink accent.
func (m Model) renderGauge(done, target, cells int) string {
	if cells < 1 {
		cells = 1
	}
	frac := 0.0
	if target > 0 {
		frac = float64(done) / float64(target)
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(cells) + 0.5)
	if done > 0 && filled == 0 {
		filled = 1
	}
	if filled > cells {
		filled = cells
	}
	fill := m.theme.Accent
	if target > 0 && done >= target {
		fill = m.theme.Success
	}
	return fill.Render(strings.Repeat("█", filled)) +
		m.theme.Muted.Render(strings.Repeat("░", cells-filled))
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

	// Clip to a single line each so a narrow terminal can't wrap the status bar
	// (which would change its height and shift the layout).
	bar := clipBox(shortHelp, m.width, 1)
	statusLine := clipBox(status, m.width, 1)
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
