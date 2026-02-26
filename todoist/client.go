package todoist

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"resty.dev/v3"
)

const baseURL = "https://api.todoist.com/api/v1"

// Client communicates with the Todoist API v1.
type Client struct {
	http   *resty.Client
	logger zerolog.Logger
}

// NewClient creates a new Todoist API client.
func NewClient(token string, logger zerolog.Logger) *Client {
	l := logger.With().Str("component", "todoist").Logger()
	r := resty.New().
		SetAuthToken(token).
		AddRequestMiddleware(func(_ *resty.Client, req *resty.Request) error {
			req.SetHeader("X-Request-Id", uuid.New().String())
			return nil
		}).
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
				Str("elapsed", resp.Duration().String()).
				Str("resp_body", resp.String()).
				Msg("http round trip")
			if resp.IsError() {
				return fmt.Errorf(
					"todoist API error %d: %s",
					resp.StatusCode(), resp.String(),
				)
			}
			return nil
		}).SetBaseURL(baseURL)
	return &Client{http: r, logger: l}
}

// GetProjects returns all projects (exhausting pagination).
func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	var all []Project
	var cursor *string
	for {
		var page paginatedResponse[Project]
		req := c.http.R().SetContext(ctx).SetResult(&page)
		if cursor != nil {
			req.SetQueryParam("cursor", *cursor)
		}
		if _, err := req.Get("/projects"); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

// FindProjectByName returns the project matching the given name.
func (c *Client) FindProjectByName(
	ctx context.Context,
	name string,
) (*Project, error) {
	projects, err := c.GetProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("todoist project %q not found", name)
}

// GetSections returns all sections for a project (exhausting pagination).
func (c *Client) GetSections(
	ctx context.Context,
	projectID string,
) ([]Section, error) {
	var all []Section
	var cursor *string
	for {
		var page paginatedResponse[Section]
		req := c.http.R().
			SetContext(ctx).
			SetQueryParam("project_id", projectID).
			SetResult(&page)
		if cursor != nil {
			req.SetQueryParam("cursor", *cursor)
		}
		if _, err := req.Get("/sections"); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

// CreateSection creates a new section in a project.
func (c *Client) CreateSection(
	ctx context.Context,
	projectID, name string,
) (*Section, error) {
	var section Section
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(map[string]string{
			"project_id": projectID,
			"name":       name,
		}).
		SetResult(&section).
		Post("/sections")
	if err != nil {
		return nil, err
	}
	return &section, nil
}

// GetTasks returns all active tasks for a project (exhausting pagination).
func (c *Client) GetTasks(
	ctx context.Context,
	projectID string,
) ([]Task, error) {
	var all []Task
	var cursor *string
	for {
		var page paginatedResponse[Task]
		req := c.http.R().
			SetContext(ctx).
			SetQueryParam("project_id", projectID).
			SetResult(&page)
		if cursor != nil {
			req.SetQueryParam("cursor", *cursor)
		}
		if _, err := req.Get("/tasks"); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

// GetCompletedTasks returns tasks completed between since and until for the
// given project. It exhausts pagination so the caller gets the full set.
func (c *Client) GetCompletedTasks(
	ctx context.Context,
	projectID string,
	since, until string,
) ([]Task, error) {
	var all []Task
	var cursor *string
	for {
		var page completedResponse
		req := c.http.R().
			SetContext(ctx).
			SetQueryParam("project_id", projectID).
			SetQueryParam("since", since).
			SetQueryParam("until", until).
			SetResult(&page)
		if cursor != nil {
			req.SetQueryParam("cursor", *cursor)
		}
		if _, err := req.Get("/tasks/completed/by_completion_date"); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

// GetTask returns a single task by ID.
func (c *Client) GetTask(
	ctx context.Context,
	taskID string,
) (*Task, error) {
	var task Task
	_, err := c.http.R().
		SetContext(ctx).
		SetResult(&task).
		Get("/tasks/" + taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// CreateTask creates a new Todoist task.
func (c *Client) CreateTask(
	ctx context.Context,
	req CreateTaskRequest,
) (*Task, error) {
	var task Task
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&task).
		Post("/tasks")
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask updates a Todoist task.
func (c *Client) UpdateTask(
	ctx context.Context,
	taskID string,
	req UpdateTaskRequest,
) (*Task, error) {
	var task Task
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&task).
		Post("/tasks/" + taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// CloseTask marks a task as completed.
func (c *Client) CloseTask(ctx context.Context, taskID string) error {
	_, err := c.http.R().
		SetContext(ctx).
		Post("/tasks/" + taskID + "/close")
	return err
}

// ReopenTask reopens a completed task.
func (c *Client) ReopenTask(ctx context.Context, taskID string) error {
	_, err := c.http.R().
		SetContext(ctx).
		Post("/tasks/" + taskID + "/reopen")
	return err
}

// DeleteTask deletes a Todoist task.
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	_, err := c.http.R().
		SetContext(ctx).
		Delete("/tasks/" + taskID)
	return err
}

// MoveTaskToSection moves a task to a different section using the
// POST /tasks/{id}/move endpoint introduced in API v1.
func (c *Client) MoveTaskToSection(
	ctx context.Context,
	taskID, sectionID string,
) error {
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(MoveTaskRequest{SectionID: sectionID}).
		Post("/tasks/" + taskID + "/move")
	return err
}

// GetComments returns all comments for a task (exhausting pagination).
func (c *Client) GetComments(
	ctx context.Context,
	taskID string,
) ([]Comment, error) {
	var all []Comment
	var cursor *string
	for {
		var page paginatedResponse[Comment]
		req := c.http.R().
			SetContext(ctx).
			SetQueryParam("task_id", taskID).
			SetResult(&page)
		if cursor != nil {
			req.SetQueryParam("cursor", *cursor)
		}
		if _, err := req.Get("/comments"); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

// CreateComment adds a comment to a task.
func (c *Client) CreateComment(
	ctx context.Context,
	req CreateCommentRequest,
) (*Comment, error) {
	var comment Comment
	_, err := c.http.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&comment).
		Post("/comments")
	if err != nil {
		return nil, err
	}
	return &comment, nil
}
