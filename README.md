# lazytempo

A terminal UI for logging time in [Tempo](https://www.tempo.io/) against
[Jira](https://www.atlassian.com/software/jira) issues, in the style of
[lazygit](https://github.com/jesseduffield/lazygit).

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Huh](https://github.com/charmbracelet/huh). Single static binary, no external
runtime dependencies.

![lazytempo demo](assets/demo.gif)

> The recording above runs against a local mock API with fictional data — no
> real Jira or Tempo account is involved.

## Layout

```
┌─ Issues (1/48) ─────────┐┌─ Week of Nov 10 ──────────────┐
│ › PROJ-123 Fix login    ││ › Mon 10   8h  (green)        │
│   PROJ-124 Add feature  ││   Tue 11   6h  (yellow)       │
│ ★ PROJ-99  Pinned       ││   ...                         │
│   …                     ││   Week total: 32h             │
│   — 0 above, 11 below — ││                               │
├─ Details ───────────────┤│   Worklogs — Mon 10 Nov       │
│ PROJ-123                ││ › 09:00–11:00 2h PROJ-123 [CORE]
│ Fix login bug           ││   11:00–12:00 1h PROJ-99 [NO ACCOUNT]
│ Project: PROJ           ││                               │
│ Status:  In Dev         ││                               │
└─────────────────────────┘└───────────────────────────────┘
 ready                     tab switch · a add · e edit · q quit
```

A header strip across the top shows the app name, your Jira display name, the
current ISO week and date range, the week total against target with a small
gauge, and a last-synced indicator. Below it the screen is two columns:

- **Left column — Issues (top ~⅔, scrollable) over Details (bottom ~⅓):**
  - *Issues:* issues you are involved with — assigned, reported, recently viewed
    or watched (configurable JQL) — **plus every issue you have logged time on**
    (seeded from your recent Tempo worklogs and resolved back to Jira, so they
    appear even when Tempo worklogs are not synced to Jira) — plus pinned
    favorites. The list scrolls with the cursor and shows how many rows are
    hidden above/below.
  - *Details:* the selected issue (or the selected worklog when the Week panel
    is focused).
- **Right column — Week:** a per-day gauge against the target (default 8h/day,
  green at/above, pink below, with the over/under), the week total, and the
  whole week's worklogs grouped by day — **newest day on top**. `h`/`l` selects
  the day, `j`/`k` moves through worklogs (down = older, up = newer) and rolls
  into the previous/next week at the ends.
- **Bottom — Status bar:** keybindings, errors and confirmations.

## Install

### Homebrew (macOS)

```sh
brew install qvarnstr0m/tap/lazytempo
```

On Linux, download a binary from the [releases](https://github.com/qvarnstr0m/lazytempo/releases)
or use `go install` below.

### From source

```sh
make build     # build to ./bin/lazytempo
make install   # install into $GOBIN (or $GOPATH/bin)
make run       # build and run the TUI
make help      # list all targets

# or with the go toolchain directly:
go build -o lazytempo ./cmd/lazytempo
go install ./cmd/lazytempo
```

## Configuration

Configuration lives at `~/.config/lazytempo/config.toml` (respects
`XDG_CONFIG_HOME`). Every value can be overridden with an environment variable.

```sh
lazytempo --example-config   # print an example config
lazytempo --config-path      # print the resolved config path
```

```toml
jira_base_url  = "https://your-site.atlassian.net"
jira_email     = "you@example.com"
jira_api_token = "your-jira-api-token"

tempo_api_token = "your-tempo-api-token"

# Optional. The default lists issues you are assigned, have logged time on,
# reported, recently viewed or watch (updated within 30 days). Override to taste,
# e.g. only your assigned issues:
# my_issues_jql = "assignee = currentUser() AND updated >= -14d ORDER BY updated DESC"
# target_hours_per_day = 8

# Every worklog must have a Tempo account (default: true). Set false for Tempo
# instances that don't use accounts, or to log before a relevant account exists.
# require_account = true
```

Environment overrides:

| Variable | Overrides |
| --- | --- |
| `LAZYTEMPO_JIRA_BASE_URL` | `jira_base_url` |
| `LAZYTEMPO_JIRA_EMAIL` | `jira_email` |
| `LAZYTEMPO_JIRA_API_TOKEN` | `jira_api_token` |
| `LAZYTEMPO_TEMPO_API_TOKEN` | `tempo_api_token` |
| `LAZYTEMPO_MY_ISSUES_JQL` | `my_issues_jql` |
| `LAZYTEMPO_REQUIRE_ACCOUNT` | `require_account` |
| `LAZYTEMPO_TIME_FORMAT` | `time_format` |

### Getting tokens

- **Jira API token:** <https://id.atlassian.com/manage-profile/security/api-tokens>
  (used with your email for Basic auth against Jira Cloud).
- **Tempo API token:** Tempo → Settings → API integration
  (used as a Bearer token against `https://api.tempo.io/4`).

Tokens are never logged or printed.

## Keybindings

| Key | Action |
| --- | --- |
| `tab`, `1` `2` `3` | switch / jump to panel |
| `j` / `k` | navigate up / down |
| `h` / `l` | previous / next day |
| `H` / `L` | previous / next week |
| `a` | add worklog to selected issue |
| `e` | edit selected worklog |
| `d` | delete worklog (with confirmation) |
| `c` | change account on selected worklog |
| `/` | free-text search issues (see below) |
| `s` | cycle issue sort (default → recently logged → most logged) |
| `p` | pin / unpin issue |
| `r` | refresh from the API |
| `?` | toggle full help |
| `q`, `ctrl+c` | quit |

## Searching issues

Press `/` to filter the issue list as you type. The search is free-text and
case-insensitive: it splits your query into words and keeps issues where **every
word** appears in the key, summary or project key. So `proj login`,
`core report` or a raw key like `CORE-603` all work. The panel title shows the
match count (e.g. `Issues (3/48)`).

Every character is typeable in the query. Press `↓`, `tab` or `enter` to move
focus from the search box into the results (keeping the filter) — the
highlighted issue stays selected, so `j`/`k` navigation and the command keys
(`a`, `e`, `d`, `c`, `p`) act on it immediately without any further step.
Pressing `↑`/`k` on the top result jumps back up into the search box. `esc`
clears the search.

When nothing in the loaded list matches, lazytempo falls back to a direct Jira
search (after a short pause) across all issues your account can view — a
key-looking query (`PROJ-123`) matches that issue, anything else is a text
search — capped at 25 results and merged into the list (shown as `+N from
Jira`). You can log time against those results like any other issue.

## Time input

The worklog form records a **start** and **end** time via dropdowns in
5-minute steps; the duration is derived from the range (end must be after
start). Times display and are entered in the configured `time_format`: `eu`
(24-hour, e.g. `16:05`, the default) or `us` (12-hour, e.g. `4:05 PM`) —
worklogs are stored in 24-hour time either way. A new worklog defaults the start
to the current time rounded to the nearest 5 minutes and the end to one hour
later. Type in a dropdown to filter it (e.g. `16:0` / `4:0`). The date field
uses the same `time_format`: `DD-MM-YYYY` (eu) or `MM/DD/YYYY` (us), stored as
ISO `YYYY-MM-DD`. The
date defaults to the day selected in the week view, and the week view shows each
worklog as its `start–end` range.

## Account rules

By default every worklog must have a Tempo account (the rules below). Set
`require_account = false` (or `LAZYTEMPO_REQUIRE_ACCOUNT=false`) for Tempo
instances that don't use accounts, or to log before a relevant account exists —
the account field becomes optional (with a `(no account)` choice), nothing is
blocked, and account-less worklogs are shown neutrally instead of in red.

With the default (`require_account = true`):

1. The worklog form cannot be submitted without a selected account (enforced by
   Huh field validation **and** re-checked before any API call).
2. Selectable accounts are filtered to those linked to the issue's project
   (open accounts only).
3. Preselection order: (a) last account used for that issue, (b) the project's
   default account link, (c) otherwise none — you must choose.
4. If a project has no valid accounts, logging is blocked with a clear message.
5. Worklogs created elsewhere without an account are flagged `[NO ACCOUNT]` in
   red.

## Architecture

```
cmd/lazytempo        entrypoint
internal/domain      models, time parsing, account rules (pure, unit-tested)
internal/config      TOML config + env overrides (secrets redacted)
internal/cache       on-disk JSON cache (issues, accounts, links, last-used)
internal/httpx       shared JSON HTTP helper with 429/5xx backoff
internal/jira        Jira Cloud REST v3 client (behind an interface)
internal/tempo       Tempo REST v4 client (behind an interface)
internal/ui          Bubble Tea models, one panel each + root router, theme
```

The Jira and Tempo clients sit behind interfaces so the UI and service layer
can be exercised with mocks. The cache lets the TUI render immediately and
refresh in the background.

## API notes

- **Jira Cloud REST v3** — issues are searched with the enhanced JQL endpoint
  `POST /rest/api/3/search/jql`, which also resolves issue key → numeric
  `issueId`/`projectId`. The current user's `accountId` comes from
  `GET /rest/api/3/myself`.
- **Tempo REST v4** — worklogs (`/4/worklogs`, `/4/worklogs/user/{accountId}`),
  work attributes (`/4/work-attributes`, the `ACCOUNT` key is read dynamically),
  accounts (`/4/accounts`) and account links
  (`/4/account-links/project/{projectId}`).

> Endpoints follow the referenced API versions; verify against current Atlassian
> and Tempo documentation if a call behaves unexpectedly.

## Development

```sh
go test ./...        # run unit + httptest client tests
go build ./...       # build everything
go vet ./...
```

Unit tests cover time parsing, the account rules (validation, selectable
accounts, preselection), the cache, and both API clients (against `httptest`
servers). The service layer is tested with client mocks.

## Status

Config + auth, two-column layout, issue listing and search (with a Jira
fallback), week view with per-day gauges and whole-week worklogs, and
create/edit/delete worklogs with an account form (mandatory by default,
configurable). Planned next: start/stop timer, "fill the day" to target, copy
yesterday, and rounding.

## License

MIT — see [LICENSE](LICENSE).

This project depends on third-party Go modules, all under permissive licenses
(MIT, BSD, ISC, Apache-2.0). Their license texts are collected in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md); include that file with any
binary release.
