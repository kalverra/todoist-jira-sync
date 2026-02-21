// Package config holds the configuration for the todoist-jira-sync CLI.
package config

import (
	"fmt"
	"time"
)

// Config holds all configuration needed to sync Todoist and Jira.
type Config struct {
	TodoistToken   string
	TodoistProject string
	JiraURL        string
	JiraEmail      string
	JiraToken      string
	JiraProject    string
	Interval       time.Duration
	LogLevel       string
	StatusMap      map[string]string
}

// Validate checks that all required configuration values are present and valid.
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
	if c.JiraEmail == "" {
		return fmt.Errorf("jira_email is required")
	}
	if c.JiraToken == "" {
		return fmt.Errorf("jira_token is required")
	}
	if c.JiraProject == "" {
		return fmt.Errorf("jira_project is required")
	}
	if len(c.StatusMap) == 0 {
		c.StatusMap = map[string]string{
			"To Do":       "To Do",
			"In Progress": "In Progress",
			"Done":        "Done",
		}
	}
	if c.Interval == 0 {
		c.Interval = 5 * time.Minute
	}
	return nil
}

// JiraStatusForSection returns the Jira status name for a Todoist section.
// If no mapping exists, the section name is returned as-is.
func (c *Config) JiraStatusForSection(sectionName string) string {
	if status, ok := c.StatusMap[sectionName]; ok {
		return status
	}
	return sectionName
}

// SectionForJiraStatus returns the Todoist section name for a Jira status.
// If no mapping exists, the status name is returned as-is.
func (c *Config) SectionForJiraStatus(jiraStatus string) string {
	for section, status := range c.StatusMap {
		if status == jiraStatus {
			return section
		}
	}
	return jiraStatus
}
