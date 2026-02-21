package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"resty.dev/v3"
)

// Client communicates with the Jira Cloud REST API v2.
// We use v2 because it accepts plain-text descriptions;
// v3 requires Atlassian Document Format.
type Client struct {
	baseURL string
	http    *resty.Client
	logger  zerolog.Logger
}

// NewClient creates a new Jira API client.
func NewClient(
	baseURL, email, token string,
	logger zerolog.Logger,
) *Client {
	l := logger.With().Str("component", "jira").Logger()
	r := resty.New().
		SetBasicAuth(email, token).
		SetHeader("Accept", "application/json").
		AddResponseMiddleware(func(_ *resty.Client, resp *resty.Response) error {
			req := resp.Request
			l.Trace().
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
				Dur("elapsed", resp.Duration()).
				Str("resp_body", resp.String()).
				Msg("http round trip")
			if resp.IsError() {
				return fmt.Errorf(
					"jira API error %d: %s",
					resp.StatusCode(), resp.String(),
				)
			}
			return nil
		})

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    r,
		logger:  l,
	}
}

func (c *Client) apiURL(path string) string {
	return c.baseURL + "/rest/api/2" + path
}

// SearchIssues finds issues using JQL.
func (c *Client) SearchIssues(
	ctx context.Context,
	jql string,
	maxResults int,
) ([]Issue, error) {
	var allIssues []Issue
	startAt := 0

	for {
		var searchResp SearchResponse
		_, err := c.http.R().
			SetContext(ctx).
			SetQueryParams(map[string]string{
				"jql":        jql,
				"maxResults": fmt.Sprintf("%d", maxResults),
				"startAt":    fmt.Sprintf("%d", startAt),
				"fields": "summary,description,status,duedate," +
					"updated,created,labels,issuetype,project,comment",
			}).
			SetResult(&searchResp).
			Get(c.apiURL("/search"))
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, searchResp.Issues...)
		if len(allIssues) >= searchResp.Total {
			break
		}
		startAt = len(allIssues)
	}

	return allIssues, nil
}

// GetIssue returns a single issue by key.
func (c *Client) GetIssue(
	ctx context.Context,
	issueKey string,
) (*Issue, error) {
	var issue Issue
	_, err := c.http.R().
		SetContext(ctx).
		SetResult(&issue).
		Get(c.apiURL("/issue/" + issueKey))
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

// CreateIssue creates a new Jira issue.
func (c *Client) CreateIssue(
	ctx context.Context,
	req CreateIssueRequest,
) (*CreatedIssue, error) {
	var created CreatedIssue
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&created).
		Post(c.apiURL("/issue"))
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateIssue updates an existing Jira issue.
func (c *Client) UpdateIssue(
	ctx context.Context,
	issueKey string,
	req UpdateIssueRequest,
) error {
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(req).
		Put(c.apiURL("/issue/" + issueKey))
	return err
}

// DeleteIssue deletes a Jira issue.
func (c *Client) DeleteIssue(
	ctx context.Context,
	issueKey string,
) error {
	_, err := c.http.R().
		SetContext(ctx).
		Delete(c.apiURL("/issue/" + issueKey))
	return err
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(
	ctx context.Context,
	issueKey string,
) ([]Transition, error) {
	var transResp TransitionsResponse
	_, err := c.http.R().
		SetContext(ctx).
		SetResult(&transResp).
		Get(c.apiURL("/issue/" + issueKey + "/transitions"))
	if err != nil {
		return nil, err
	}
	return transResp.Transitions, nil
}

// TransitionIssue performs a workflow transition on an issue.
func (c *Client) TransitionIssue(
	ctx context.Context,
	issueKey, transitionID string,
) error {
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(TransitionRequest{
			Transition: TransitionRef{ID: transitionID},
		}).
		Post(c.apiURL("/issue/" + issueKey + "/transitions"))
	return err
}

// TransitionIssueTo transitions an issue to the named target status.
// It finds the correct transition by matching the target status name.
func (c *Client) TransitionIssueTo(
	ctx context.Context,
	issueKey, targetStatus string,
) error {
	transitions, err := c.GetTransitions(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("get transitions for %s: %w", issueKey, err)
	}

	for _, t := range transitions {
		if strings.EqualFold(t.To.Name, targetStatus) {
			c.logger.Debug().
				Str("issue", issueKey).
				Str("transition", t.Name).
				Str("target_status", targetStatus).
				Msg("transitioning issue")
			return c.TransitionIssue(ctx, issueKey, t.ID)
		}
	}

	available := make([]string, 0, len(transitions))
	for _, t := range transitions {
		available = append(available, t.To.Name)
	}
	return fmt.Errorf(
		"no transition to status %q for issue %s (available: %v)",
		targetStatus, issueKey, available,
	)
}

// GetComments returns all comments for an issue.
func (c *Client) GetComments(
	ctx context.Context,
	issueKey string,
) ([]Comment, error) {
	var page CommentPage
	_, err := c.http.R().
		SetContext(ctx).
		SetResult(&page).
		Get(c.apiURL("/issue/" + issueKey + "/comment"))
	if err != nil {
		return nil, err
	}
	return page.Comments, nil
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(
	ctx context.Context,
	issueKey, body string,
) (*Comment, error) {
	var comment Comment
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(Comment{Body: body}).
		SetResult(&comment).
		Post(c.apiURL("/issue/" + issueKey + "/comment"))
	if err != nil {
		return nil, err
	}
	return &comment, nil
}
