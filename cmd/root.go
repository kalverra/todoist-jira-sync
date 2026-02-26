// Package cmd contains command execution logic.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/kalverra/todoist-jira-sync/config"
)

var (
	cfg    *config.Config
	logger zerolog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "todoist-jira-sync",
	Short: "Bidirectional sync between Todoist and Jira",
	Long: `A CLI tool that synchronizes tasks, comments, ` +
		`statuses, and due dates between Todoist and Jira Cloud.`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		cfg, err = config.Load(config.WithFlags(cmd.PersistentFlags()))
		if err != nil {
			return err
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		logFilePath := filepath.Join(home, "Library", "Logs", "todoist-jira-sync.log.jsonl")
		//nolint:gosec // Log file is fine
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}

		multiWriter := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}, logFile)
		lvl, err := zerolog.ParseLevel(cfg.LogLevel)
		if err != nil {
			lvl = zerolog.InfoLevel
			cfg.LogLevel = "info"
			logger.Warn().Err(err).Msg("invalid log level, setting to info")
		}
		zerolog.SetGlobalLevel(lvl)
		logger = zerolog.New(multiWriter).Level(lvl).With().Timestamp().Logger()

		logger.Info().
			Str("todoist_project", cfg.TodoistProject).
			Str("jira_url", cfg.JiraURL).
			Str("jira_email", cfg.JiraEmail).
			Str("jira_project", cfg.JiraProject).
			Strs("jira_issue_types", cfg.JiraIssueTypes).
			Str("interval", cfg.Interval.String()).
			Str("log_level", cfg.LogLevel).
			Str("log_file_path", cfg.LogFilePath).
			Msg("config")

		return nil
	},
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.String("todoist-token", "", "Todoist API token (env: TODOIST_TOKEN)")
	flags.String("todoist-project", config.DefaultTodoistProject, "Todoist project name to sync (env: TODOIST_PROJECT)")
	flags.String("jira-url", "", "Jira Cloud base URL (env: JIRA_URL)")
	flags.String("jira-email", "", "Jira account email (env: JIRA_EMAIL)")
	flags.String("jira-token", "", "Jira API token (env: JIRA_TOKEN)")
	flags.String("jira-project", config.DefaultJiraProject, "Jira project key (env: JIRA_PROJECT)")
	flags.StringSlice(
		"jira-issue-types",
		config.DefaultJiraIssueTypes,
		"Jira issue types to sync, e.g. Story,Task,Bug (env: JIRA_ISSUE_TYPES)",
	)
	flags.Duration("interval", config.DefaultInterval, "Polling interval for watch mode (env: SYNC_INTERVAL)")
	flags.String("log-level", config.DefaultLogLevel, "Log level: trace, debug, info, warn, error (env: LOG_LEVEL)")
	flags.String("log-file-path", config.DefaultLogFilePath, "Log file path (env: LOG_FILE_PATH)")
}

// Execute runs the root command.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
