// Package config holds the configuration for the todoist-jira-sync CLI.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration needed to sync Todoist and Jira.
type Config struct {
	TodoistToken   string            `mapstructure:"todoist_token"`
	TodoistProject string            `mapstructure:"todoist_project"`
	JiraURL        string            `mapstructure:"jira_url"`
	JiraEmail      string            `mapstructure:"jira_email"`
	JiraToken      string            `mapstructure:"jira_token"`
	JiraProject    string            `mapstructure:"jira_project"`
	JiraIssueTypes []string          `mapstructure:"jira_issue_types"` // issue type names to sync (e.g. Story, Task, Bug); set via flag/env or default
	Interval       time.Duration     `mapstructure:"interval"`
	LogLevel       string            `mapstructure:"log_level"`
	StatusMap      map[string]string `mapstructure:"status_map"`
}

const (
	// DefaultTodoistProject project to sync.
	DefaultTodoistProject = "Work"
	// DefaultJiraProject key to sync.
	DefaultJiraProject = "DX"
	// DefaultInterval polling interval.
	DefaultInterval = 5 * time.Minute
	// DefaultLogLevel log level.
	DefaultLogLevel = "info"
)

var (
	// DefaultStatusMap maps Todoist statuses to Jira statuses.
	DefaultStatusMap = map[string]string{ // todoist status -> jira status
		"To Do":       "To Do",
		"In Progress": "In Progress",
		"In Review":   "In Review",
		"Done":        "Done",
		"Blocked":     "Blocked",
	}
	// DefaultJiraIssueTypes Jira issue types to sync.
	DefaultJiraIssueTypes = []string{"Story", "Task", "Bug", "Sub-task"}
)

// LoadOption is a function that can be used to load configuration.
type LoadOption func(*viper.Viper) error

// WithFlags loads configuration from command line flags.
func WithFlags(flags *pflag.FlagSet) LoadOption {
	return func(v *viper.Viper) error {
		if err := v.BindPFlags(flags); err != nil {
			return err
		}
		return nil
	}
}

// Load loads configuration from environment variables and configuration files.
func Load(opts ...LoadOption) (*Config, error) {
	v := viper.New()

	v.SetDefault("todoist_project", DefaultTodoistProject)
	v.SetDefault("jira_project", DefaultJiraProject)
	v.SetDefault("jira_issue_types", DefaultJiraIssueTypes)
	v.SetDefault("interval", DefaultInterval)
	v.SetDefault("log_level", DefaultLogLevel)
	v.SetDefault("status_map", DefaultStatusMap)

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	v.AutomaticEnv()

	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		fmt.Println("no config file found")
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.TodoistToken == "" {
		return fmt.Errorf("todoist_token is required")
	}
	if c.TodoistProject == "" {
		return fmt.Errorf("todoist_project is required")
	}
	if c.JiraURL == "" {
		return fmt.Errorf("jira_url is required")
	}
	c.JiraURL = strings.TrimRight(c.JiraURL, "/")
	if !strings.HasPrefix(c.JiraURL, "http://") && !strings.HasPrefix(c.JiraURL, "https://") {
		c.JiraURL = "https://" + c.JiraURL
	}
	if c.JiraEmail == "" {
		return fmt.Errorf("jira_email is required")
	}
	if c.JiraToken == "" {
		return fmt.Errorf("jira_token is required")
	}
	if c.JiraProject == "" {
		return fmt.Errorf("jira_project is required")
	}
	return nil
}

// JiraStatusForSection returns the Jira status name for a Todoist section.
// It uses the status map to map the Todoist section name to the Jira status name.
// If no mapping exists, the section name is returned as-is.
func (c *Config) JiraStatusForSection(sectionName string) string {
	if status, ok := c.StatusMap[sectionName]; ok {
		return status
	}
	return sectionName
}

// SectionForJiraStatus returns the Todoist section name for a Jira status.
// It uses the status map to map the Jira status name to the Todoist section name.
// If no mapping exists, the status name is returned as-is.
func (c *Config) SectionForJiraStatus(jiraStatus string) string {
	for section, status := range c.StatusMap {
		if status == jiraStatus {
			return section
		}
	}
	return jiraStatus
}

// JiraIssueTypesJQL returns a JQL fragment for filtering by configured issue types.
// e.g. `issuetype IN (Story, Task, Bug)`. Returns empty string if no types are configured.
func (c *Config) JiraIssueTypesJQL() string {
	if len(c.JiraIssueTypes) == 0 {
		return ""
	}
	quoted := make([]string, len(c.JiraIssueTypes))
	for i, t := range c.JiraIssueTypes {
		t = strings.TrimSpace(t)
		if strings.ContainsRune(t, ' ') || strings.ContainsRune(t, ',') {
			quoted[i] = `"` + strings.ReplaceAll(t, `"`, `\"`) + `"`
		} else {
			quoted[i] = t
		}
	}
	return "issuetype IN (" + strings.Join(quoted, ", ") + ")"
}
