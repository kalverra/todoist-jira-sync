// Package jira provides an HTTP client for the Jira Cloud REST API v3.
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"resty.dev/v3"

	"github.com/kalverra/todoist-jira-sync/config"
)

const (
	// SprintInfoField is the custom field name to get sprint info for a Jira issue.
	SprintInfoField = "customfield_10020"
	// EpicLinkField is the custom field name to get epic link for a Jira issue.
	EpicLinkField = "customfield_10014"
)

// Client communicates with the Jira Cloud REST API v3 via Resty.
type Client struct {
	http   *resty.Client
	logger zerolog.Logger
	cfg    *config.Config
}

// NewClient creates a new Jira API v3 client.
func NewClient(cfg *config.Config, logger zerolog.Logger) (*Client, error) {
	l := logger.With().Str("component", "jira").Logger()

	r := resty.New().
		SetBasicAuth(cfg.JiraEmail, cfg.JiraToken).
		SetBaseURL(cfg.JiraURL+"/rest/api/3").
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		AddResponseMiddleware(func(_ *resty.Client, resp *resty.Response) error {
			req := resp.Request
			ev := l.Trace().
				Str("method", req.Method).
				Str("url", req.RawRequest.URL.String()).
				Func(func(e *zerolog.Event) {
					if req.Body != nil {
						if b, err := json.Marshal(req.Body); err == nil {
							e.RawJSON("req_body", b)
						}
					}
				}).
				Int("status", resp.StatusCode()).
				Str("elapsed", resp.Duration().String())
			body := resp.String()
			if len(body) > 2000 {
				ev.Str("resp_body", body[:2000]+"...(truncated)")
			} else {
				ev.Str("resp_body", body)
			}
			ev.Msg("http round trip")
			if resp.IsError() {
				return fmt.Errorf("jira API error %d: %s", resp.StatusCode(), body)
			}
			return nil
		})

	return &Client{http: r, logger: l, cfg: cfg}, nil
}

// SearchIssues searches for issues using JQL (enhanced search endpoint).
func (c *Client) SearchIssues(
	ctx context.Context,
	jql string,
	fields []string,
	maxResults int,
) ([]Issue, error) {
	var result SearchResponse
	req := c.http.R().
		SetContext(ctx).
		SetQueryParam("jql", jql).
		SetQueryParam("maxResults", fmt.Sprintf("%d", maxResults)).
		SetResult(&result)
	if len(fields) > 0 {
		req.SetQueryParam("fields", strings.Join(fields, ","))
	}
	if _, err := req.Get("/search/jql"); err != nil {
		return nil, err
	}

	return result.Issues, nil
}

// CreateIssue creates a new Jira issue.
func (c *Client) CreateIssue(ctx context.Context, issue *Issue) (*CreateIssueResponse, error) {
	var result CreateIssueResponse
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(issue).
		SetResult(&result).
		Post("/issue")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetIssue fetches a single issue by key.
func (c *Client) GetIssue(ctx context.Context, key string, fields []string) (*Issue, error) {
	var result Issue
	req := c.http.R().
		SetContext(ctx).
		SetResult(&result)
	if len(fields) > 0 {
		req.SetQueryParam("fields", strings.Join(fields, ","))
	}
	if _, err := req.Get("/issue/" + key); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateIssue updates an existing issue's fields.
func (c *Client) UpdateIssue(ctx context.Context, key string, issue *Issue) error {
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(issue).
		Put("/issue/" + key)
	return err
}

// DeleteIssue deletes an issue by key.
func (c *Client) DeleteIssue(ctx context.Context, key string) error {
	_, err := c.http.R().
		SetContext(ctx).
		SetQueryParam("deleteSubtasks", "true").
		Delete("/issue/" + key)
	return err
}

// DoTransition transitions an issue to the target status by name.
// It fetches available transitions, finds the match, and POSTs it.
func (c *Client) DoTransition(ctx context.Context, issueKey, targetStatus string) error {
	var tr TransitionsResponse
	_, err := c.http.R().
		SetContext(ctx).
		SetResult(&tr).
		Get("/issue/" + issueKey + "/transitions")
	if err != nil {
		return fmt.Errorf("get transitions: %w", err)
	}

	var transitionID string
	for _, t := range tr.Transitions {
		if strings.EqualFold(t.Name, targetStatus) || strings.EqualFold(t.To.Name, targetStatus) {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		available := make([]string, len(tr.Transitions))
		for i, t := range tr.Transitions {
			available[i] = fmt.Sprintf("%s (-> %s)", t.Name, t.To.Name)
		}
		return fmt.Errorf("no transition found for status %q, available: %v", targetStatus, available)
	}

	payload := TransitionRequest{
		Transition: TransitionID{ID: transitionID},
	}
	if strings.EqualFold(targetStatus, "Closed") {
		payload.Fields = &TransitionFields{
			Resolution: &Resolution{Name: "Done"},
		}
	}

	_, err = c.http.R().
		SetContext(ctx).
		SetBody(payload).
		Post("/issue/" + issueKey + "/transitions")
	if err != nil {
		return fmt.Errorf("do transition: %w", err)
	}
	return nil
}

// AddComment adds a comment to an issue. Body must be ADF JSON.
func (c *Client) AddComment(ctx context.Context, issueKey string, body json.RawMessage) (*Comment, error) {
	var result Comment
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(map[string]json.RawMessage{"body": body}).
		SetResult(&result).
		Post("/issue/" + issueKey + "/comment")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// InCurrentSprint checks if the issue is in an active sprint by inspecting
// the SprintRaw (customfield_10020) field.
func InCurrentSprint(issue *Issue) bool {
	if issue.Fields == nil || len(issue.Fields.SprintRaw) == 0 || string(issue.Fields.SprintRaw) == "null" {
		return false
	}

	var sprints []map[string]any
	if err := json.Unmarshal(issue.Fields.SprintRaw, &sprints); err != nil {
		return false
	}
	for _, s := range sprints {
		if s["state"] == "active" {
			return true
		}
	}
	return false
}

// TodoistPriority converts a Jira priority ID to a Todoist priority level.
func TodoistPriority(jiraPriorityID string) int {
	jiraPriorityID = strings.TrimSpace(jiraPriorityID)
	switch jiraPriorityID {
	case "1":
		return 4
	case "2":
		return 3
	case "3":
		return 2
	}
	return 1
}
