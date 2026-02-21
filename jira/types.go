// Package jira provides a client for the Jira Cloud REST API.
package jira

import "time"

// Issue represents a Jira issue.
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields holds the standard fields of a Jira issue.
type IssueFields struct {
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Status      *Status       `json:"status,omitempty"`
	DueDate     string        `json:"duedate,omitempty"`
	Updated     string        `json:"updated,omitempty"`
	Created     string        `json:"created,omitempty"`
	Labels      []string      `json:"labels,omitempty"`
	IssueType   *IssueType    `json:"issuetype,omitempty"`
	Project     *IssueProject `json:"project,omitempty"`
	Comment     *CommentPage  `json:"comment,omitempty"`
}

// Status represents a Jira issue status.
type Status struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IssueType represents a Jira issue type.
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IssueProject represents the project reference in an issue.
type IssueProject struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

// Comment represents a Jira issue comment.
type Comment struct {
	ID      string `json:"id,omitempty"`
	Body    string `json:"body"`
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// CommentPage is the paginated response for issue comments.
type CommentPage struct {
	Comments   []Comment `json:"comments"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
}

// Transition represents a Jira workflow transition.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

// TransitionsResponse wraps the list of available transitions.
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// SearchResponse is the JQL search result.
type SearchResponse struct {
	Issues     []Issue `json:"issues"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	StartAt    int     `json:"startAt"`
}

// CreateIssueRequest is the payload for creating a Jira issue.
type CreateIssueRequest struct {
	Fields CreateIssueFields `json:"fields"`
}

// CreateIssueFields holds the fields for creating a Jira issue.
type CreateIssueFields struct {
	Project     IssueProject `json:"project"`
	Summary     string       `json:"summary"`
	Description string       `json:"description,omitempty"`
	IssueType   IssueType    `json:"issuetype"`
	DueDate     string       `json:"duedate,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
}

// CreatedIssue is the response from creating an issue.
type CreatedIssue struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// UpdateIssueRequest is the payload for updating a Jira issue.
type UpdateIssueRequest struct {
	Fields map[string]any `json:"fields"`
}

// TransitionRequest is the payload for performing a Jira transition.
type TransitionRequest struct {
	Transition TransitionRef `json:"transition"`
}

// TransitionRef identifies the transition to perform.
type TransitionRef struct {
	ID string `json:"id"`
}

// ParseUpdated parses the issue's updated timestamp.
func (i *Issue) ParseUpdated() (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000-0700", i.Fields.Updated)
}
