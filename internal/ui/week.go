package ui

import (
	"time"

	"lazytempo/internal/domain"
)

const dateLayout = "2006-01-02"

// weekStart returns the Monday of the week containing t (local date, time
// truncated to midnight).
func weekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	// Go's Weekday has Sunday=0; shift so Monday is the first day.
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// buildDays groups worklogs into the seven days starting at start.
func buildDays(start time.Time, worklogs []domain.Worklog) []domain.Day {
	days := make([]domain.Day, 7)
	index := map[string]int{}
	for i := 0; i < 7; i++ {
		d := start.AddDate(0, 0, i)
		days[i] = domain.Day{Date: d}
		index[d.Format(dateLayout)] = i
	}
	for _, w := range worklogs {
		if i, ok := index[w.StartDate]; ok {
			days[i].Worklogs = append(days[i].Worklogs, w)
		}
	}
	return days
}

// weekTotalSeconds sums all logged time in the week.
func weekTotalSeconds(days []domain.Day) int {
	total := 0
	for _, d := range days {
		total += d.TotalSeconds()
	}
	return total
}
