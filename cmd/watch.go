package cmd

import (
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/syncer"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously sync Todoist and Jira on a polling interval",
	RunE: func(cmd *cobra.Command, _ []string) error {
		bindEnv(&cfg)
		if err := cfg.Validate(); err != nil {
			return err
		}

		todoistClient := todoist.NewClient(cfg.TodoistToken, logger)
		jiraClient, err := jira.NewClient(
			cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken, logger,
		)
		if err != nil {
			return err
		}
		engine := syncer.NewEngine(
			todoistClient, jiraClient, &cfg, logger,
		)

		ctx, stop := signal.NotifyContext(
			cmd.Context(), syscall.SIGINT, syscall.SIGTERM,
		)
		defer stop()

		logger.Info().
			Dur("interval", cfg.Interval).
			Msg("starting watch mode")

		if err := engine.Run(ctx); err != nil {
			logger.Error().Err(err).Msg("sync cycle failed")
		}

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info().Msg("shutting down watch mode")
				return nil
			case <-ticker.C:
				if err := engine.Run(ctx); err != nil {
					logger.Error().Err(err).Msg("sync cycle failed")
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
