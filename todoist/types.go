// Package todoist provides a client for the Todoist API v1.
package todoist

import "time"

// paginatedResponse is the wrapper returned by all list endpoints in API v1.
type paginatedResponse[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

// completedResponse wraps the completed tasks endpoints which use "items" instead of "results".
type completedResponse struct {
	Items      []Task  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// Task represents a Todoist task from API v1.
type Task struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ProjectID      string    `json:"project_id"`
	SectionID      string    `json:"section_id"`
	ParentID       string    `json:"parent_id"`
	Content        string    `json:"content"`
	Description    string    `json:"description"`
	Checked        bool      `json:"checked"`
	IsDeleted      bool      `json:"is_deleted"`
	Labels         []string  `json:"labels"`
	ChildOrder     int       `json:"child_order"`
	Priority       int       `json:"priority"`
	Due            *Due      `json:"due"`
	Deadline       *Deadline `json:"deadline"`
	Duration       *Duration `json:"duration"`
	AddedByUID     string    `json:"added_by_uid"`
	AssignedByUID  string    `json:"assigned_by_uid"`
	ResponsibleUID string    `json:"responsible_uid"`
	NoteCount      int       `json:"note_count"`
	DayOrder       int       `json:"day_order"`
	IsCollapsed    bool      `json:"is_collapsed"`
	AddedAt        string    `json:"added_at"`
	UpdatedAt      string    `json:"updated_at"`
	CompletedAt    string    `json:"completed_at"`
	CompletedByUID string    `json:"completed_by_uid"`
}

// Due represents a task's due date.
type Due struct {
	String      string `json:"string"`
	Date        string `json:"date"`
	IsRecurring bool   `json:"is_recurring"`
	Datetime    string `json:"datetime,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	Lang        string `json:"lang,omitempty"`
}

// Duration represents a task's duration.
type Duration struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

// Deadline represents a task's deadline.
type Deadline struct {
	Date     string `json:"date"`
	Timezone string `json:"timezone,omitempty"`
	Lang     string `json:"lang"`
}

// Section represents a Todoist section.
type Section struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	ProjectID    string `json:"project_id"`
	Name         string `json:"name"`
	SectionOrder int    `json:"section_order"`
	IsArchived   bool   `json:"is_archived"`
	IsDeleted    bool   `json:"is_deleted"`
	IsCollapsed  bool   `json:"is_collapsed"`
	AddedAt      string `json:"added_at"`
	UpdatedAt    string `json:"updated_at"`
	ArchivedAt   string `json:"archived_at"`
}

// Comment represents a Todoist comment (API v1 uses the Sync API format).
type Comment struct {
	ID             string              `json:"id"`
	PostedUID      string              `json:"posted_uid"`
	Content        string              `json:"content"`
	FileAttachment map[string]string   `json:"file_attachment"`
	UIDsToNotify   []string            `json:"uids_to_notify"`
	IsDeleted      bool                `json:"is_deleted"`
	PostedAt       string              `json:"posted_at"`
	Reactions      map[string][]string `json:"reactions"`
}

// Project represents a Todoist project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	IsArchived  bool   `json:"is_archived"`
	IsDeleted   bool   `json:"is_deleted"`
	IsFavorite  bool   `json:"is_favorite"`
	ViewStyle   string `json:"view_style"`
	ParentID    string `json:"parent_id"`
	ChildOrder  int    `json:"child_order"`
	IsCollapsed bool   `json:"is_collapsed"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateTaskRequest is the payload for creating a new Todoist task.
type CreateTaskRequest struct {
	Content     string   `json:"content"`
	Description string   `json:"description,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	SectionID   string   `json:"section_id,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// UpdateTaskRequest is the payload for updating a Todoist task.
type UpdateTaskRequest struct {
	Content     *string `json:"content,omitempty"`
	Description *string `json:"description,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
}

// CreateCommentRequest is the payload for creating a Todoist comment.
type CreateCommentRequest struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
}

// MoveTaskRequest is the payload for the POST /tasks/{id}/move endpoint.
type MoveTaskRequest struct {
	ProjectID string `json:"project_id,omitempty"`
	SectionID string `json:"section_id,omitempty"`
	ParentID  string `json:"parent_id,omitempty"`
}

// ParseAddedAt parses the added_at field into a time.Time.
func (t *Task) ParseAddedAt() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, t.AddedAt)
}
