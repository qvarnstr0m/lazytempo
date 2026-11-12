# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
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
