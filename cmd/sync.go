package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/syncer"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Run a single sync cycle between Todoist and Jira",
	RunE: func(cmd *cobra.Command, _ []string) error {
		bindEnv(&cfg)
		if err := cfg.Validate(); err != nil {
			return err
		}

		todoistClient := todoist.NewClient(cfg.TodoistToken, logger)
		jiraClient := jira.NewClient(
			cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken, logger,
		)
		engine := syncer.NewEngine(
			todoistClient, jiraClient, &cfg, logger,
		)

		return engine.Run(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
