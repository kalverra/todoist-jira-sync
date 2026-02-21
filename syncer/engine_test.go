package syncer

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/todoist-jira-sync/config"
	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../.env")
	os.Exit(m.Run())
}

type e2eEnv struct {
	todoistClient *todoist.Client
	jiraClient    *jira.Client
	cfg           *config.Config
	engine        *Engine
	logger        zerolog.Logger
	projectID     string
}

func e2eSetup(t *testing.T) *e2eEnv {
	t.Helper()

	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("set RUN_E2E_TESTS=true to run E2E tests")
	}
	for _, env := range []string{
		"TODOIST_API_TOKEN", "TODOIST_PROJECT",
		"JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_PROJECT",
	} {
		require.NotEmpty(t, os.Getenv(env),
			"%s must be set for E2E tests", env,
		)
	}

	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr},
	).With().Timestamp().Logger()

	cfg := &config.Config{
		TodoistToken:   os.Getenv("TODOIST_API_TOKEN"),
		TodoistProject: os.Getenv("TODOIST_PROJECT"),
		JiraURL:        os.Getenv("JIRA_URL"),
		JiraEmail:      os.Getenv("JIRA_EMAIL"),
		JiraToken:      os.Getenv("JIRA_API_TOKEN"),
		JiraProject:    os.Getenv("JIRA_PROJECT"),
	}
	require.NoError(t, cfg.Validate())

	tc := todoist.NewClient(cfg.TodoistToken, logger)
	jc := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken, logger)
	engine := NewEngine(tc, jc, cfg, logger)

	project, err := tc.FindProjectByName(
		context.Background(), cfg.TodoistProject,
	)
	require.NoError(t, err)

	return &e2eEnv{
		todoistClient: tc,
		jiraClient:    jc,
		cfg:           cfg,
		engine:        engine,
		logger:        logger,
		projectID:     project.ID,
	}
}

func testID() string {
	return uuid.New().String()[:8]
}

func TestSyncTodoistToJira(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-sync-t2j-%s", testID())
	task, err := env.todoistClient.CreateTask(ctx, todoist.CreateTaskRequest{
		Content:     taskName,
		Description: "todoist to jira test",
		ProjectID:   env.projectID,
		DueDate:     "2026-12-25",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if err := env.todoistClient.DeleteTask(
			cleanupCtx, task.ID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", task.ID, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	updatedTask, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	jiraKey := ExtractJiraKey(updatedTask.Description)
	require.NotEmpty(t, jiraKey, "todoist task should have jira key in description")
	t.Cleanup(func() {
		if err := env.jiraClient.DeleteIssue(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	issue, err := env.jiraClient.GetIssue(ctx, jiraKey)
	require.NoError(t, err)
	assert.Equal(t, taskName, issue.Fields.Summary)
	assert.Contains(t, issue.Fields.Description, "todoist to jira test")
	assert.Equal(t, "2026-12-25", issue.Fields.DueDate)

	todoistID := ExtractTodoistID(issue.Fields.Description)
	assert.Equal(t, task.ID, todoistID)
}

func TestSyncJiraToTodoist(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	summary := fmt.Sprintf("e2e-sync-j2t-%s", testID())
	created, err := env.jiraClient.CreateIssue(ctx, jira.CreateIssueRequest{
		Fields: jira.CreateIssueFields{
			Project:     jira.IssueProject{Key: env.cfg.JiraProject},
			Summary:     summary,
			Description: "jira to todoist test",
			IssueType:   jira.IssueType{Name: "Task"},
			DueDate:     "2026-11-15",
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := env.jiraClient.DeleteIssue(
			context.Background(), created.Key,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", created.Key, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	issue, err := env.jiraClient.GetIssue(ctx, created.Key)
	require.NoError(t, err)
	todoistID := ExtractTodoistID(issue.Fields.Description)
	require.NotEmpty(t, todoistID,
		"jira issue should have todoist id in description",
	)
	t.Cleanup(func() {
		if err := env.todoistClient.DeleteTask(
			context.Background(), todoistID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", todoistID, err)
		}
	})

	task, err := env.todoistClient.GetTask(ctx, todoistID)
	require.NoError(t, err)
	assert.Equal(t, summary, task.Content)
	assert.Contains(t, task.Description, "jira to todoist test")
	require.NotNil(t, task.Due)
	assert.Equal(t, "2026-11-15", task.Due.Date)

	jiraKey := ExtractJiraKey(task.Description)
	assert.Equal(t, created.Key, jiraKey)
}

func TestSyncComments(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-sync-comments-%s", testID())
	task, err := env.todoistClient.CreateTask(ctx, todoist.CreateTaskRequest{
		Content:   taskName,
		ProjectID: env.projectID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := env.todoistClient.DeleteTask(
			context.Background(), task.ID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", task.ID, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	updatedTask, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	jiraKey := ExtractJiraKey(updatedTask.Description)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if err := env.jiraClient.DeleteIssue(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	todoistComment := fmt.Sprintf("todoist comment %s", testID())
	_, err = env.todoistClient.CreateComment(ctx, todoist.CreateCommentRequest{
		TaskID:  task.ID,
		Content: todoistComment,
	})
	require.NoError(t, err)

	jiraComment := fmt.Sprintf("jira comment %s", testID())
	_, err = env.jiraClient.AddComment(ctx, jiraKey, jiraComment)
	require.NoError(t, err)

	// Wait briefly for API consistency before syncing comments
	time.Sleep(2 * time.Second)

	require.NoError(t, env.engine.Run(ctx))

	jiraComments, err := env.jiraClient.GetComments(ctx, jiraKey)
	require.NoError(t, err)
	foundTodoistComment := false
	for _, c := range jiraComments {
		if c.Body == "[From Todoist] "+todoistComment {
			foundTodoistComment = true
			break
		}
	}
	assert.True(t, foundTodoistComment,
		"jira should have synced todoist comment",
	)

	todoistComments, err := env.todoistClient.GetComments(ctx, task.ID)
	require.NoError(t, err)
	foundJiraComment := false
	for _, c := range todoistComments {
		if c.Content == "[From Jira] "+jiraComment {
			foundJiraComment = true
			break
		}
	}
	assert.True(t, foundJiraComment,
		"todoist should have synced jira comment",
	)
}

func TestSyncStatusChange(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-sync-status-%s", testID())
	task, err := env.todoistClient.CreateTask(ctx, todoist.CreateTaskRequest{
		Content:   taskName,
		ProjectID: env.projectID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := env.todoistClient.DeleteTask(
			context.Background(), task.ID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", task.ID, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	updatedTask, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	jiraKey := ExtractJiraKey(updatedTask.Description)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if err := env.jiraClient.DeleteIssue(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	err = env.jiraClient.TransitionIssueTo(ctx, jiraKey, "In Progress")
	require.NoError(t, err)

	// Wait for Jira to update the timestamp
	time.Sleep(2 * time.Second)

	require.NoError(t, env.engine.Run(ctx))

	sections, err := env.todoistClient.GetSections(ctx, env.projectID)
	require.NoError(t, err)

	targetSection := env.cfg.SectionForJiraStatus("In Progress")
	sectionIDs := make(map[string]string)
	for _, s := range sections {
		sectionIDs[s.Name] = s.ID
	}

	refetched, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)

	expectedSectionID := sectionIDs[targetSection]
	if expectedSectionID != "" {
		assert.Equal(t, expectedSectionID, refetched.SectionID,
			"task should be in %q section", targetSection,
		)
	}
}

func TestSyncDueDateUpdate(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-sync-due-%s", testID())
	task, err := env.todoistClient.CreateTask(ctx, todoist.CreateTaskRequest{
		Content:   taskName,
		ProjectID: env.projectID,
		DueDate:   "2026-06-01",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := env.todoistClient.DeleteTask(
			context.Background(), task.ID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", task.ID, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	updatedTask, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	jiraKey := ExtractJiraKey(updatedTask.Description)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if err := env.jiraClient.DeleteIssue(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	newDue := "2027-03-15"
	err = env.jiraClient.UpdateIssue(ctx, jiraKey, jira.UpdateIssueRequest{
		Fields: map[string]any{"duedate": newDue},
	})
	require.NoError(t, err)

	// Wait for Jira to update the timestamp
	time.Sleep(2 * time.Second)

	require.NoError(t, env.engine.Run(ctx))

	refetched, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, refetched.Due)
	assert.Equal(t, newDue, refetched.Due.Date)
}
