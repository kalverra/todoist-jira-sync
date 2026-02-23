// Package cmd contains command execution logic.
package cmd

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/kalverra/todoist-jira-sync/config"
)

var (
	cfg    config.Config
	logger zerolog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "todoist-jira-sync",
	Short: "Bidirectional sync between Todoist and Jira",
	Long: `A CLI tool that synchronizes tasks, comments, ` +
		`statuses, and due dates between Todoist and Jira Cloud.`,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		lvl, err := zerolog.ParseLevel(cfg.LogLevel)
		if err != nil {
			lvl = zerolog.InfoLevel
		}
		zerolog.SetGlobalLevel(lvl)
		logger = zerolog.New(
			zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: time.RFC3339,
			},
		).With().Timestamp().Logger()
	},
}

func init() {
	_ = godotenv.Load()

	flags := rootCmd.PersistentFlags()
	flags.StringVar(
		&cfg.TodoistToken, "todoist-token", "",
		"Todoist API token (env: TODOIST_API_TOKEN)",
	)
	flags.StringVar(
		&cfg.TodoistProject, "todoist-project", "",
		"Todoist project name to sync (env: TODOIST_PROJECT)",
	)
	flags.StringVar(
		&cfg.JiraURL, "jira-url", "",
		"Jira Cloud base URL (env: JIRA_URL)",
	)
	flags.StringVar(
		&cfg.JiraEmail, "jira-email", "",
		"Jira account email (env: JIRA_EMAIL)",
	)
	flags.StringVar(
		&cfg.JiraToken, "jira-token", "",
		"Jira API token (env: JIRA_API_TOKEN)",
	)
	flags.StringVar(
		&cfg.JiraProject, "jira-project", "",
		"Jira project key (env: JIRA_PROJECT)",
	)
	flags.StringVar(
		&cfg.JiraIssueTypesStr,
		"jira-issue-types",
		"",
		"Comma-separated Jira issue types to sync, e.g. Story,Task,Bug (env: JIRA_ISSUE_TYPES; default: Story,Task,Bug)",
	)
	flags.DurationVar(
		&cfg.Interval, "interval", 5*time.Minute,
		"Polling interval for watch mode (env: SYNC_INTERVAL)",
	)
	flags.StringVar(
		&cfg.LogLevel, "log-level", "info",
		"Log level: trace, debug, info, warn, error (env: LOG_LEVEL)",
	)

	bindEnv(&cfg)
}

// bindEnv reads environment variables into the config,
// only overriding zero-value fields.
func bindEnv(c *config.Config) {
	if v := os.Getenv("TODOIST_API_TOKEN"); v != "" && c.TodoistToken == "" {
		c.TodoistToken = v
	}
	if v := os.Getenv("TODOIST_PROJECT"); v != "" && c.TodoistProject == "" {
		c.TodoistProject = v
	}
	if v := os.Getenv("JIRA_URL"); v != "" && c.JiraURL == "" {
		c.JiraURL = v
	}
	if v := os.Getenv("JIRA_EMAIL"); v != "" && c.JiraEmail == "" {
		c.JiraEmail = v
	}
	if v := os.Getenv("JIRA_API_TOKEN"); v != "" && c.JiraToken == "" {
		c.JiraToken = v
	}
	if v := os.Getenv("JIRA_PROJECT"); v != "" && c.JiraProject == "" {
		c.JiraProject = v
	}
	if v := os.Getenv("JIRA_ISSUE_TYPES"); v != "" && c.JiraIssueTypesStr == "" {
		c.JiraIssueTypesStr = v
	}
	if v := os.Getenv("SYNC_INTERVAL"); v != "" && c.Interval == 0 {
		if d, err := time.ParseDuration(v); err == nil {
			c.Interval = d
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" && c.LogLevel == "" {
		c.LogLevel = v
	}
}

// Execute runs the root command.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
