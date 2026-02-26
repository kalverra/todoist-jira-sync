// Package jira provides an HTTP client for the Jira Cloud REST API v3.
package jira

import "encoding/json"

// SearchResponse is returned by the JQL search endpoint.
type SearchResponse struct {
	Issues     []Issue `json:"issues"`
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
}

// Issue represents a Jira issue from the v3 API.
type Issue struct {
	ID     string       `json:"id,omitempty"`
	Key    string       `json:"key,omitempty"`
	Self   string       `json:"self,omitempty"`
	Fields *IssueFields `json:"fields,omitempty"`
}

// IssueFields holds the fields of a Jira issue.
// Description and comment Body are ADF (Atlassian Document Format) JSON.
type IssueFields struct {
	Summary     string          `json:"summary,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Status      *Status         `json:"status,omitempty"`
	Priority    *Priority       `json:"priority,omitempty"`
	Resolution  *Resolution     `json:"resolution,omitempty"`
	Updated     string          `json:"updated,omitempty"`
	Duedate     string          `json:"duedate,omitempty"`
	Comment     *CommentPage    `json:"comment,omitempty"`
	Project     *Project        `json:"project,omitempty"`
	IssueType   *IssueType      `json:"issuetype,omitempty"`
	SprintRaw   json.RawMessage `json:"customfield_10020,omitempty"`
}

// Status represents a Jira workflow status.
type Status struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Priority represents a Jira priority level.
type Priority struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Resolution represents a Jira resolution.
type Resolution struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Project represents a Jira project reference.
type Project struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

// IssueType represents a Jira issue type.
type IssueType struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// CommentPage holds a page of comments returned inline with an issue.
type CommentPage struct {
	Comments   []Comment `json:"comments"`
	Total      int       `json:"total"`
	MaxResults int       `json:"maxResults"`
	StartAt    int       `json:"startAt"`
}

// Comment represents a single Jira comment (v3: body is ADF).
type Comment struct {
	ID      string          `json:"id,omitempty"`
	Author  *User           `json:"author,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	Created string          `json:"created,omitempty"`
	Updated string          `json:"updated,omitempty"`
}

// User represents a Jira user.
type User struct {
	AccountID   string `json:"accountId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// Transition represents an available workflow transition.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

// TransitionsResponse is the response from the transitions endpoint.
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// CreateIssueResponse is the response from creating an issue.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// TransitionRequest is the payload for POST /issue/{key}/transitions.
type TransitionRequest struct {
	Transition TransitionID      `json:"transition"`
	Fields     *TransitionFields `json:"fields,omitempty"`
}

// TransitionID identifies a transition by its ID string.
type TransitionID struct {
	ID string `json:"id"`
}

// TransitionFields holds optional fields for a transition (e.g. resolution).
type TransitionFields struct {
	Resolution *Resolution `json:"resolution,omitempty"`
}
