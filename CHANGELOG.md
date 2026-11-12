# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- All panel text now uses the theme palette instead of falling through to the
  terminal's default foreground (e.g. day labels, week total, worklog rows and
  detail values); worklog start–end times use the pink accent.
- The worklog form (create/edit) now sizes itself to the popup box, so the
  description and other fields use the full width instead of wrapping early.
  The form also resizes with the terminal window.

### Changed
- The issue search (`/`) is now multi-word free text: every word must appear in
  an issue's key, summary or project key (e.g. `proj login`). The panel
  title shows the match count and an empty result shows a clear message. All
  characters stay typeable; press `↓`, `tab` or `enter` to move focus from the
  search box into the results (keeping the filter), where `j`/`k` and the
  command keys (`a`/`e`/`d`/`c`/`p`) act on the highlighted issue immediately.
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
