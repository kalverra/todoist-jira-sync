package todoist

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../.env")
	os.Exit(m.Run())
}

func e2eSetup(t *testing.T) (*Client, string) {
	t.Helper()

	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("set RUN_E2E_TESTS=true to run E2E tests")
	}
	for _, env := range []string{
		"TODOIST_API_TOKEN", "TODOIST_PROJECT",
	} {
		require.NotEmpty(t, os.Getenv(env),
			"%s must be set for E2E tests", env,
		)
	}

	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr},
	).With().Timestamp().Logger()
	client := NewClient(os.Getenv("TODOIST_API_TOKEN"), logger)

	project, err := client.FindProjectByName(
		context.Background(), os.Getenv("TODOIST_PROJECT"),
	)
	require.NoError(t, err)
	return client, project.ID
}

func testID() string {
	return uuid.New().String()[:8]
}

func TestTodoistTaskCRUD(t *testing.T) { //nolint:paralleltest
	client, projectID := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-test-crud-%s", testID())
	task, err := client.CreateTask(ctx, CreateTaskRequest{
		Content:     taskName,
		Description: "initial description",
		ProjectID:   projectID,
		DueDate:     "2026-12-31",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := client.DeleteTask(
			context.Background(), task.ID,
		); err != nil {
			t.Logf("cleanup: delete task %s: %v", task.ID, err)
		}
	})

	assert.Equal(t, taskName, task.Content)
	assert.Equal(t, "initial description", task.Description)
	assert.NotEmpty(t, task.ID)

	fetched, err := client.GetTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, taskName, fetched.Content)
	assert.Equal(t, "initial description", fetched.Description)
	require.NotNil(t, fetched.Due)
	assert.Equal(t, "2026-12-31", fetched.Due.Date)

	newContent := fmt.Sprintf("e2e-test-updated-%s", testID())
	newDesc := "updated description"
	newDue := "2027-06-15"
	updated, err := client.UpdateTask(ctx, task.ID, UpdateTaskRequest{
		Content:     &newContent,
		Description: &newDesc,
		DueDate:     &newDue,
	})
	require.NoError(t, err)
	assert.Equal(t, newContent, updated.Content)
	assert.Equal(t, newDesc, updated.Description)

	fetched2, err := client.GetTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, newContent, fetched2.Content)
	assert.Equal(t, newDesc, fetched2.Description)
	require.NotNil(t, fetched2.Due)
	assert.Equal(t, newDue, fetched2.Due.Date)
}

func TestTodoistComments(t *testing.T) { //nolint:paralleltest
	client, projectID := e2eSetup(t)
	ctx := context.Background()

	task, err := client.CreateTask(ctx, CreateTaskRequest{
		Content:   fmt.Sprintf("e2e-test-comments-%s", testID()),
		ProjectID: projectID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := client.DeleteTask(
			context.Background(), task.ID,
		); err != nil {
			t.Logf("cleanup: delete task %s: %v", task.ID, err)
		}
	})

	commentBody := fmt.Sprintf("test comment %s", testID())
	comment, err := client.CreateComment(ctx, CreateCommentRequest{
		TaskID:  task.ID,
		Content: commentBody,
	})
	require.NoError(t, err)
	assert.Equal(t, commentBody, comment.Content)
	assert.NotEmpty(t, comment.ID)

	comments, err := client.GetComments(ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, commentBody, comments[0].Content)
}

func TestTodoistSections(t *testing.T) { //nolint:paralleltest
	client, projectID := e2eSetup(t)
	ctx := context.Background()

	sections, err := client.GetSections(ctx, projectID)
	require.NoError(t, err)
	assert.NotEmpty(t, sections,
		"project should have at least one section",
	)
	for _, s := range sections {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.Name)
		assert.Equal(t, projectID, s.ProjectID)
	}
}
