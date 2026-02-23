// Package syncer implements bidirectional synchronization between Todoist and Jira.
package syncer

import (
	"fmt"
	"regexp"
	"strings"
)

// jiraPrefixPattern matches a markdown link like [PROJ-123](https://...) at the start of content.
var jiraPrefixPattern = regexp.MustCompile(`^\[([A-Z][A-Z0-9_]+-\d+)\]\(https?://[^)]+\)\s*`)

// ExtractJiraKey extracts the Jira issue key from a Todoist task's content prefix.
// Returns empty string if no link prefix is found.
func ExtractJiraKey(content string) string {
	matches := jiraPrefixPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// StripJiraPrefix removes the [PROJ-123](url) prefix from content,
// returning only the user-authored title.
func StripJiraPrefix(content string) string {
	return jiraPrefixPattern.ReplaceAllString(content, "")
}

// PrependJiraLink prepends a markdown link to the Jira issue at the start of content.
// If the content already has a Jira prefix, it is replaced.
// jiraBaseURL should be the Jira instance URL without trailing slash, e.g. "https://example.atlassian.net".
func PrependJiraLink(content, jiraKey, jiraBaseURL string) string {
	stripped := StripJiraPrefix(content)
	link := fmt.Sprintf("[%s](%s/browse/%s)", jiraKey, jiraBaseURL, jiraKey)
	if stripped == "" {
		return link
	}
	return link + " " + stripped
}

// NormalizeJiraURL ensures the URL has a scheme and no trailing slash.
func NormalizeJiraURL(rawURL string) string {
	u := strings.TrimRight(rawURL, "/")
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return u
}
