package syncer

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	jiraCloud "github.com/andygrunwald/go-jira/v2/cloud"
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
	jc, err := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken, logger)
	require.NoError(t, err)
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
		Labels:      []string{linkLabel},
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
	jiraKey := ExtractJiraKey(updatedTask.Content)
	require.NotEmpty(t, jiraKey, "todoist task content should have jira key prefix")
	t.Cleanup(func() {
		if _, err := env.jiraClient.Issue.Delete(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	issue, _, err := env.jiraClient.Issue.Get(ctx, jiraKey, nil)
	require.NoError(t, err)
	assert.Equal(t, taskName, issue.Fields.Summary)
	assert.Contains(t, issue.Fields.Description, "todoist to jira test")
	assert.Equal(t, taskName, StripJiraPrefix(updatedTask.Content))
}

func TestSyncJiraToTodoist(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	summary := fmt.Sprintf("e2e-sync-j2t-%s", testID())
	created, _, err := env.jiraClient.Issue.Create(ctx, &jiraCloud.Issue{
		Fields: &jiraCloud.IssueFields{
			Project:     jiraCloud.Project{Key: env.cfg.JiraProject},
			Summary:     summary,
			Description: "jira to todoist test",
			Type:        jiraCloud.IssueType{Name: "Task"},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if _, err := env.jiraClient.Issue.Delete(
			context.Background(), created.Key,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", created.Key, err)
		}
	})

	require.NoError(t, env.engine.Run(ctx))

	tasks, err := env.todoistClient.GetTasks(ctx, env.projectID)
	require.NoError(t, err)

	var linkedTask *todoist.Task
	for i := range tasks {
		if ExtractJiraKey(tasks[i].Content) == created.Key {
			linkedTask = &tasks[i]
			break
		}
	}
	require.NotNil(t, linkedTask, "should find todoist task linked to %s", created.Key)
	t.Cleanup(func() {
		if err := env.todoistClient.DeleteTask(
			context.Background(), linkedTask.ID,
		); err != nil {
			t.Logf("cleanup: delete todoist task %s: %v", linkedTask.ID, err)
		}
	})

	assert.Equal(t, summary, StripJiraPrefix(linkedTask.Content))
	assert.Contains(t, linkedTask.Description, "jira to todoist test")
	assert.Contains(t, linkedTask.Labels, linkLabel)
}

func TestSyncComments(t *testing.T) { //nolint:paralleltest
	env := e2eSetup(t)
	ctx := context.Background()

	taskName := fmt.Sprintf("e2e-sync-comments-%s", testID())
	task, err := env.todoistClient.CreateTask(ctx, todoist.CreateTaskRequest{
		Content:   taskName,
		ProjectID: env.projectID,
		Labels:    []string{linkLabel},
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
	jiraKey := ExtractJiraKey(updatedTask.Content)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if _, err := env.jiraClient.Issue.Delete(
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
	_, _, err = env.jiraClient.Issue.AddComment(ctx, jiraKey, &jiraCloud.Comment{Body: jiraComment})
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	require.NoError(t, env.engine.Run(ctx))

	issue, _, err := env.jiraClient.Issue.Get(ctx, jiraKey, nil)
	require.NoError(t, err)
	foundTodoistComment := false
	if issue.Fields.Comments != nil {
		for _, c := range issue.Fields.Comments.Comments {
			if c.Body == "[From Todoist] "+todoistComment {
				foundTodoistComment = true
				break
			}
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
		Labels:    []string{linkLabel},
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
	jiraKey := ExtractJiraKey(updatedTask.Content)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if _, err := env.jiraClient.Issue.Delete(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	_, err = env.jiraClient.Issue.DoTransition(ctx, jiraKey, "In Progress")
	require.NoError(t, err)

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
		Labels:    []string{linkLabel},
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
	jiraKey := ExtractJiraKey(updatedTask.Content)
	require.NotEmpty(t, jiraKey)
	t.Cleanup(func() {
		if _, err := env.jiraClient.Issue.Delete(
			context.Background(), jiraKey,
		); err != nil {
			t.Logf("cleanup: delete jira issue %s: %v", jiraKey, err)
		}
	})

	newDue := "2027-03-15"
	newDueTime, _ := time.Parse("2006-01-02", newDue)
	_, _, err = env.jiraClient.Issue.Update(ctx, &jiraCloud.Issue{
		Key: jiraKey,
		Fields: &jiraCloud.IssueFields{
			Duedate: jiraCloud.Date(newDueTime),
		},
	}, nil)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	require.NoError(t, env.engine.Run(ctx))

	refetched, err := env.todoistClient.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, refetched.Due)
	assert.Equal(t, newDue, refetched.Due.Date)
}
