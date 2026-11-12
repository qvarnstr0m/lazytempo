// Command lazytempo is a terminal UI for logging time in Tempo against Jira
// issues, in the style of lazygit. Every worklog must have a Tempo account.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"lazytempo/internal/cache"
	"lazytempo/internal/config"
	"lazytempo/internal/httpx"
	"lazytempo/internal/jira"
	"lazytempo/internal/tempo"
	"lazytempo/internal/ui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			printUsage()
			return
		case "--version", "-v":
			fmt.Printf("lazytempo %s\n", version)
			return
		case "--example-config":
			fmt.Print(config.ExampleTOML())
			return
		case "--config-path":
			p, err := config.Path()
			if err != nil {
				fail(err)
			}
			fmt.Println(p)
			return
		}
	}

	if err := run(); err != nil {
		fail(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		path, _ := config.Path()
		return fmt.Errorf("%w\n\nCreate %s (see `lazytempo --example-config`) or set the LAZYTEMPO_* environment variables", err, path)
	}

	doer := httpx.NewDoer(nil)
	jiraClient := jira.New(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, doer)
	tempoClient := tempo.New(tempo.DefaultBaseURL, cfg.TempoAPIToken, doer)

	c, err := cache.Open()
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}

	svc := ui.NewService(cfg, jiraClient, tempoClient, c)
	model := ui.New(svc, c, cfg)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	// Persist cache (pins, last-used accounts) on exit.
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save cache: %v\n", err)
	}
	return nil
}

func printUsage() {
	fmt.Print(`lazytempo — log time in Tempo against Jira issues (lazygit-style TUI)

Usage:
  lazytempo                 Start the TUI
  lazytempo --example-config  Print an example config.toml
  lazytempo --config-path     Print the config file path
  lazytempo --version         Print the version
  lazytempo --help            Show this help

Configuration is read from ~/.config/lazytempo/config.toml with LAZYTEMPO_*
environment variable overrides. See the README for details.
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
