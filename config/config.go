// Package config holds the configuration for the todoist-jira-sync CLI.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config holds all configuration needed to sync Todoist and Jira.
type Config struct {
	TodoistToken      string
	TodoistProject    string
	JiraURL           string
	JiraEmail         string
	JiraToken         string
	JiraProject       string
	JiraIssueTypes    []string // issue type names to sync (e.g. Story, Task, Bug); set via flag/env or default
	JiraIssueTypesStr string   // comma-separated for flag/env, e.g. "Story,Task,Bug"
	Interval          time.Duration
	LogLevel          string
	StatusMap         map[string]string
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
	if len(c.StatusMap) == 0 {
		c.StatusMap = map[string]string{
			"To Do":       "To Do",
			"In Progress": "In Progress",
			"Done":        "Done",
		}
	}
	if len(c.JiraIssueTypes) == 0 {
		if c.JiraIssueTypesStr != "" {
			for s := range strings.SplitSeq(c.JiraIssueTypesStr, ",") {
				if t := strings.TrimSpace(s); t != "" {
					c.JiraIssueTypes = append(c.JiraIssueTypes, t)
				}
			}
		}
	}
	if len(c.JiraIssueTypes) == 0 {
		c.JiraIssueTypes = []string{"Story", "Task", "Bug"}
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

// JiraIssueTypesJQL returns a JQL fragment for filtering by configured issue types,
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
