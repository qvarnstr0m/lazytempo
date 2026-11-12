# lazytempo

A terminal UI for logging time in [Tempo](https://www.tempo.io/) against
[Jira](https://www.atlassian.com/software/jira) issues, in the style of
[lazygit](https://github.com/jesseduffield/lazygit).

**Every worklog must have a Tempo account attached.** A worklog without an
account is never submitted, and existing account-less worklogs are highlighted
in red in the week view.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Huh](https://github.com/charmbracelet/huh). Single static binary, no external
runtime dependencies.

## Layout

```
┌─ Issues ───────┐┌─ Week of Nov 10 ──────────┐┌─ Details ──────┐
│ › PROJ-123 Fix ││ › Mon 10   8h  (green)     ││ PROJ-123       │
│   PROJ-124 Add ││   Tue 11   6h  (yellow)   ││ Fix login bug  │
│ ★ PROJ-99  Pin ││   ...                     ││ Project: PROJ  │
│                ││   Week total: 32h         ││ Status: In Dev │
│                ││   Worklogs — Mon 10 Nov   ││                │
│                ││   › 2h PROJ-123 [CORE]    ││                │
│                ││     1h PROJ-99 [NO ACCOUNT]│                │
└────────────────┘└───────────────────────────┘└────────────────┘
 1h30m, 1.5h, …          tab switch · a add · e edit · d delete · q quit
```

- **Left — Issues:** issues you are involved with — assigned, logged time on,
  reported, recently viewed or watched (configurable JQL) — plus pinned
  favorites.
- **Center — Week:** worklogs per day, totals per day/week against a target
  (default 8h/day). Days at or above target are green, below target yellow.
- **Right — Details:** the selected issue or worklog.
- **Bottom — Status bar:** keybindings, errors and confirmations.

## Install

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
```

Environment overrides:

| Variable | Overrides |
| --- | --- |
| `LAZYTEMPO_JIRA_BASE_URL` | `jira_base_url` |
| `LAZYTEMPO_JIRA_EMAIL` | `jira_email` |
| `LAZYTEMPO_JIRA_API_TOKEN` | `jira_api_token` |
| `LAZYTEMPO_TEMPO_API_TOKEN` | `tempo_api_token` |
| `LAZYTEMPO_MY_ISSUES_JQL` | `my_issues_jql` |

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
| `/` | search issues (also by key, e.g. `PROJ-123`) |
| `p` | pin / unpin issue |
| `r` | refresh from the API |
| `?` | toggle full help |
| `q`, `ctrl+c` | quit |

## Time input

The worklog form accepts: `1h30m`, `1.5h`, `90m`, `45m`, `1:30`, `45s`, or a
bare number read as hours (`2` = 2h). The date defaults to the day selected in
the week view.

## Account rules

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

MVP: config + auth, three-panel layout, issue listing, week view, and
create/edit/delete worklogs with the mandatory-account form. Planned next:
start/stop timer, "fill the day" to target, copy yesterday, and rounding.

## License

MIT — see [LICENSE](LICENSE).
