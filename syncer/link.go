// Package syncer implements bidirectional synchronization between Todoist and Jira.
package syncer

import (
	"regexp"
	"strings"
	"time"
)

const (
	syncSeparator  = "\n\n---\n"
	jiraKeyPrefix  = "synced-jira-key: "
	todoistPrefix  = "synced-todoist-id: "
	syncedAtPrefix = "synced-at: "
)

var (
	jiraKeyPattern   = regexp.MustCompile(`synced-jira-key:\s*([A-Z][A-Z0-9_]+-\d+)`)
	todoistIDPattern = regexp.MustCompile(`synced-todoist-id:\s*(\d+)`)
	syncedAtPattern  = regexp.MustCompile(`synced-at:\s*(.+)`)
)

// ExtractJiraKey extracts the Jira issue key from a Todoist task description.
// Returns empty string if no link is found.
func ExtractJiraKey(description string) string {
	matches := jiraKeyPattern.FindStringSubmatch(description)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// ExtractTodoistID extracts the Todoist task ID from a Jira issue description.
// Returns empty string if no link is found.
func ExtractTodoistID(description string) string {
	matches := todoistIDPattern.FindStringSubmatch(description)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// ExtractSyncedAt extracts the last sync timestamp from a description.
func ExtractSyncedAt(description string) (time.Time, bool) {
	matches := syncedAtPattern.FindStringSubmatch(description)
	if len(matches) < 2 {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(matches[1]))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// UserDescription returns the user-authored portion of a description,
// stripping the sync metadata footer.
func UserDescription(description string) string {
	before, _, ok := strings.Cut(description, syncSeparator)
	if !ok {
		return description
	}
	return strings.TrimRight(before, "\n")
}

// buildFooter constructs the sync metadata footer.
func buildFooter(linkKey, linkValue string, syncTime time.Time) string {
	return syncSeparator + linkKey + linkValue + "\n" + syncedAtPrefix + syncTime.Format(time.RFC3339)
}

// SetJiraKey sets or updates the Jira key in a Todoist task description,
// preserving the user-authored content.
func SetJiraKey(description, jiraKey string, syncTime time.Time) string {
	userDesc := UserDescription(description)
	return userDesc + buildFooter(jiraKeyPrefix, jiraKey, syncTime)
}

// SetTodoistID sets or updates the Todoist ID in a Jira issue description,
// preserving the user-authored content.
func SetTodoistID(description, todoistID string, syncTime time.Time) string {
	userDesc := UserDescription(description)
	return userDesc + buildFooter(todoistPrefix, todoistID, syncTime)
}
