# Demo recording

This directory regenerates the demo GIF in the project README
([`assets/demo.gif`](../assets/demo.gif)). It records the **real** `lazytempo`
TUI driven entirely by **fictional data** — no real Jira or Tempo account, no
customer data, and nothing leaves the machine.

## How it works

- [`mock.go`](mock.go) is a tiny stand-in for the Jira + Tempo APIs. It listens
  on `:8787` and serves made-up issues, accounts and a full current week of
  worklogs. Created/edited worklogs are kept in memory so the demo can show a
  worklog being added. It is excluded from normal builds (`//go:build ignore`),
  so it never ships with `go install ./...`; run it by naming the file
  explicitly (`go run demo/mock.go`), as `record.sh` does.
- [`demo.tape`](demo.tape) is the [VHS](https://github.com/charmbracelet/vhs)
  script that launches the app and performs the keystrokes.
- The app is pointed at the vanity host `http://hollywood-upstairs.atlassian.net` (which is
  what the header shows) and reaches the mock via `HTTP_PROXY=localhost:8787`.
  This keeps the recording free of `localhost` and local file paths **without**
  editing `/etc/hosts` or needing root.
- The recording shell uses throwaway `XDG_CONFIG_HOME`/`XDG_CACHE_HOME` dirs and
  dummy tokens, so your real config, cache and credentials are never touched.

## Regenerate the GIF

Requires [`vhs`](https://github.com/charmbracelet/vhs) (`brew install vhs`) and
Go.

```sh
./demo/record.sh      # or: make demo
```

This builds the binary, starts the mock, records `demo/demo.tape`, and writes
`assets/demo.gif`. Edit `demo.tape` to change the walkthrough (timings,
keystrokes, window size).
