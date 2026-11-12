# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- The create/edit worklog form no longer overflows when a project has many
  accounts: the account dropdown is capped at 8 visible rows (type to filter)
  and the whole form is height-constrained so it scrolls, keeping the top of the
  form reachable.
- Creating or updating a worklog now sends `remainingEstimateSeconds` (the
  issue's current remaining estimate, fetched from Jira), fixing the Tempo 400
  "Remaining estimate must not be empty" on projects with estimate tracking.
  The current estimate is sent unchanged, so logging time does not alter it.
- Panel content is now clipped to its box width (ANSI-aware) as well as its
  height, so long issue summaries are truncated with `…` instead of wrapping.
  Previously, on a small window, wrapped lines made a panel taller than its box
  and pushed the whole frame off the top of the terminal, so the issue and
  worklog panels appeared stuck showing only their bottom rows regardless of
  scrolling. The status bar is likewise clipped so it can't wrap.
- All panel text now uses the theme palette instead of falling through to the
  terminal's default foreground (e.g. day labels, week total, worklog rows and
  detail values); worklog start–end times use the pink accent.
- The worklog form (create/edit) now sizes itself to the popup box, so the
  description and other fields use the full width instead of wrapping early.
  The form also resizes with the terminal window.

### Added
- `time_format` config (default `eu`; env `LAZYTEMPO_TIME_FORMAT`): `eu` shows
  and enters clock times in 24-hour form (`16:05`), `us` in 12-hour form
  (`4:05 PM`). Applies to the worklog form dropdowns and the week/details views;
  worklogs are always stored in 24-hour time.
- `require_account` config (default `true`; env `LAZYTEMPO_REQUIRE_ACCOUNT`).
  When false, the worklog account becomes optional (a `(no account)` choice),
  logging is never blocked for lacking an account, and account-less worklogs are
  shown neutrally instead of in red — for Tempo instances without accounts or to
  log before a relevant account exists. The default keeps the account mandatory.
- Press `s` to cycle the issue list sort: default (recently updated), recently
  logged, or most logged — using per-issue totals and last-logged dates
  aggregated from the recent (30-day) worklog window. Pinned issues stay first
  and the active sort shows in the panel title.
- Issue search falls back to a direct Jira search when nothing in the loaded
  list matches: after a short debounce it queries Jira (key match for a
  key-looking query, otherwise a text search) across all issues the account can
  view, capped at 25 results, and merges them into the list (`+N from Jira`).
- The Week panel now lists the whole week's worklogs grouped by day, not just
  the selected day. The selected day and worklog stay highlighted, empty days
  are omitted (except the selected one), and the list scrolls to keep the
  selected worklog visible. `j`/`k` flows across day boundaries, and past the
  first/last worklog of the week it crosses into the previous/next week (loading
  it and landing on that week's last/first worklog). `h`/`l` still jumps days,
  `H`/`L` jumps weeks.
- Per-day gauges in the Week panel: each weekday shows a progress bar against
  the daily target with its over/under (`+15m`, `-1h`, `✓`), weekends show a
  neutral row, and the week total has a matching gauge.
- A header strip across the top of the UI: app name, your Jira display name, the
  current ISO week and Mon–Sun date range, the week total against target with a
  small gauge, and a last-synced indicator (`synced 2m ago` / `syncing…`). The
  Jira `myself` lookup now also reads the display name.

### Changed
- The layout is now two columns: the left column stacks Issues (top ~2/3,
  scrollable) over Details (bottom ~1/3), and the right column is the Week
  panel. The issue list scrolls with the cursor and shows how many rows are
  hidden above/below; panel content is clipped to its box so it never overflows.
- The issue search (`/`) is now multi-word free text: every word must appear in
  an issue's key, summary or project key (e.g. `proj login`). The panel
  title shows the match count and an empty result shows a clear message. All
  characters stay typeable; press `↓`, `tab` or `enter` to move focus from the
  search box into the results (keeping the filter), where `j`/`k` and the
  command keys (`a`/`e`/`d`/`c`/`p`) act on the highlighted issue immediately.
  Pressing `↑`/`k` on the top result returns focus to the search box.
- The worklog form now records start and end time via 5-minute dropdowns in
  24-hour European format (e.g. 16:05) instead of a free-text duration; the time
  spent is derived from the range (end must be after start). For a new worklog
  the start defaults to the current time rounded to the nearest 5 minutes and
  the end to one hour later. The dropdowns are filterable by typing, and the
  week view shows each worklog as a start–end range.
- The default issue list now includes issues you are involved with — assigned,
  reported, recently viewed (`issueHistory()`) or watching (`watchedIssues()`) —
  updated within the last 30 days, instead of only issues assigned to you.
  Override via `my_issues_jql` / `LAZYTEMPO_MY_ISSUES_JQL`.
- The issue list is additionally seeded from your recent (last 30 days) Tempo
  worklogs: their issue ids are resolved back to Jira issues and merged in, so
  issues you have logged time on always appear — even when Tempo worklogs are
  not synced to Jira worklogs and no JQL can find them. Worklog-sourced issues
  survive a JQL refresh and are cached for instant display next launch. This
  also lets you edit/add time on such issues directly.

### Added
- Initial MVP:
  - TOML configuration with `LAZYTEMPO_*` environment overrides; secrets are
    redacted and never logged.
  - Jira Cloud REST v3 client (`myself`, enhanced JQL search) behind an
    interface.
  - Tempo REST v4 client (worklogs CRUD, work attributes, accounts, account
    links) behind an interface, with the `ACCOUNT` attribute key resolved
    dynamically.
  - Shared HTTP helper with retry/backoff on HTTP 429 and transient 5xx.
  - On-disk JSON cache for issues, accounts, account links and the last-used
    account per issue.
  - Charm-style three-panel TUI (Issues / Week / Details) with a status bar and
    auto-generated help.
  - Worklog form (Huh) that enforces the mandatory-account rule, filters
    selectable accounts to the issue's project, and preselects by last-used →
    project default → none.
  - Create, edit and delete worklogs; pin/unpin issues; issue search.
  - Unit tests for time parsing, account rules and cache; `httptest` tests for
    the Jira and Tempo clients; mock-based tests for the service layer.
