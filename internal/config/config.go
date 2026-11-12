// Package config loads lazytempo configuration from a TOML file with
// environment variable overrides.
//
// Secrets (API tokens) are never logged or printed; the String method and any
// debug output deliberately redact them.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// AppName is used to build config and cache paths.
const AppName = "lazytempo"

// Config holds all runtime configuration.
type Config struct {
	JiraBaseURL   string `toml:"jira_base_url"`
	JiraEmail     string `toml:"jira_email"`
	JiraAPIToken  string `toml:"jira_api_token"`
	TempoAPIToken string `toml:"tempo_api_token"`

	// MyIssuesJQL is the JQL used to populate the "My issues" panel.
	MyIssuesJQL string `toml:"my_issues_jql"`
	// TargetHoursPerDay is the daily target used to colour day totals.
	TargetHoursPerDay float64 `toml:"target_hours_per_day"`
}

// defaultJQL lists issues you are involved with: assigned to you, logged time
// on, reported, recently viewed, or watched — updated within the last 30 days.
const defaultJQL = "(assignee = currentUser() OR worklogAuthor = currentUser() OR reporter = currentUser() OR issuekey IN issueHistory() OR issuekey IN watchedIssues()) AND updated >= -30d ORDER BY updated DESC"

// Default returns a Config pre-populated with sensible defaults.
func Default() Config {
	return Config{
		MyIssuesJQL:       defaultJQL,
		TargetHoursPerDay: 8,
	}
}

// Path returns the path to the config file, honouring XDG_CONFIG_HOME.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, AppName, "config.toml"), nil
}

// Load reads the config file (if present) and applies environment overrides.
// A missing config file is not an error as long as the required values are
// supplied via the environment.
func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}

	cfg.applyEnv()
	cfg.normalize()
	return cfg, cfg.Validate()
}

// applyEnv overrides fields from environment variables when they are set.
func (c *Config) applyEnv() {
	if v := os.Getenv("LAZYTEMPO_JIRA_BASE_URL"); v != "" {
		c.JiraBaseURL = v
	}
	if v := os.Getenv("LAZYTEMPO_JIRA_EMAIL"); v != "" {
		c.JiraEmail = v
	}
	if v := os.Getenv("LAZYTEMPO_JIRA_API_TOKEN"); v != "" {
		c.JiraAPIToken = v
	}
	if v := os.Getenv("LAZYTEMPO_TEMPO_API_TOKEN"); v != "" {
		c.TempoAPIToken = v
	}
	if v := os.Getenv("LAZYTEMPO_MY_ISSUES_JQL"); v != "" {
		c.MyIssuesJQL = v
	}
}

func (c *Config) normalize() {
	c.JiraBaseURL = strings.TrimRight(strings.TrimSpace(c.JiraBaseURL), "/")
	if c.MyIssuesJQL == "" {
		c.MyIssuesJQL = defaultJQL
	}
	if c.TargetHoursPerDay <= 0 {
		c.TargetHoursPerDay = 8
	}
}

// Validate checks that the required fields are present.
func (c Config) Validate() error {
	var missing []string
	if c.JiraBaseURL == "" {
		missing = append(missing, "jira_base_url")
	}
	if c.JiraEmail == "" {
		missing = append(missing, "jira_email")
	}
	if c.JiraAPIToken == "" {
		missing = append(missing, "jira_api_token")
	}
	if c.TempoAPIToken == "" {
		missing = append(missing, "tempo_api_token")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

// TargetSecondsPerDay returns the daily target in seconds.
func (c Config) TargetSecondsPerDay() int {
	return int(c.TargetHoursPerDay * 3600)
}

// String renders the config with secrets redacted. Never print the struct
// directly; use this method.
func (c Config) String() string {
	return fmt.Sprintf(
		"Config{JiraBaseURL:%q, JiraEmail:%q, JiraAPIToken:%s, TempoAPIToken:%s, MyIssuesJQL:%q, TargetHoursPerDay:%.1f}",
		c.JiraBaseURL, c.JiraEmail, redact(c.JiraAPIToken), redact(c.TempoAPIToken), c.MyIssuesJQL, c.TargetHoursPerDay,
	)
}

func redact(secret string) string {
	if secret == "" {
		return "<unset>"
	}
	return "<redacted>"
}

// ExampleTOML returns a commented example config, used by the README and by the
// `--print-config-path` helper.
func ExampleTOML() string {
	return `# ` + AppName + ` configuration
# Location: ~/.config/` + AppName + `/config.toml
# Any value can be overridden with an environment variable, e.g.
#   LAZYTEMPO_TEMPO_API_TOKEN=...

jira_base_url = "https://your-site.atlassian.net"
jira_email    = "you@example.com"
jira_api_token = "your-jira-api-token"

tempo_api_token = "your-tempo-api-token"

# Optional:
# my_issues_jql lists the issues shown in the left panel. The default includes
# issues you are assigned, have logged time on, reported, recently viewed or
# watch, updated within the last 30 days. Override it to taste, e.g. only your
# own assigned issues:
# my_issues_jql = "assignee = currentUser() AND updated >= -14d ORDER BY updated DESC"
# target_hours_per_day = 8
`
}
