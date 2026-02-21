package jira

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
		"JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_PROJECT",
	} {
		require.NotEmpty(t, os.Getenv(env),
			"%s must be set for E2E tests", env,
		)
	}

	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr},
	).With().Timestamp().Logger()
	client := NewClient(
		os.Getenv("JIRA_URL"),
		os.Getenv("JIRA_EMAIL"),
		os.Getenv("JIRA_API_TOKEN"),
		logger,
	)
	return client, os.Getenv("JIRA_PROJECT")
}

func testID() string {
	return uuid.New().String()[:8]
}

func createTestIssue(
	t *testing.T,
	client *Client,
	project string,
) *Issue {
	t.Helper()
	ctx := context.Background()

	summary := fmt.Sprintf("e2e-test-%s", testID())
	created, err := client.CreateIssue(ctx, CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:     IssueProject{Key: project},
			Summary:     summary,
			Description: "e2e test issue",
			IssueType:   IssueType{Name: "Task"},
			DueDate:     "2026-12-31",
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := client.DeleteIssue(
			context.Background(), created.Key,
		); err != nil {
			t.Logf("cleanup: delete issue %s: %v", created.Key, err)
		}
	})

	issue, err := client.GetIssue(ctx, created.Key)
	require.NoError(t, err)
	return issue
}

func TestJiraIssueCRUD(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)
	assert.NotEmpty(t, issue.Key)
	assert.Equal(t, "e2e test issue", issue.Fields.Description)
	assert.Equal(t, "2026-12-31", issue.Fields.DueDate)

	newSummary := fmt.Sprintf("e2e-test-updated-%s", testID())
	newDesc := "updated description"
	newDue := "2027-06-15"
	err := client.UpdateIssue(ctx, issue.Key, UpdateIssueRequest{
		Fields: map[string]any{
			"summary":     newSummary,
			"description": newDesc,
			"duedate":     newDue,
		},
	})
	require.NoError(t, err)

	fetched, err := client.GetIssue(ctx, issue.Key)
	require.NoError(t, err)
	assert.Equal(t, newSummary, fetched.Fields.Summary)
	assert.Equal(t, newDesc, fetched.Fields.Description)
	assert.Equal(t, newDue, fetched.Fields.DueDate)
}

func TestJiraTransitions(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)

	transitions, err := client.GetTransitions(ctx, issue.Key)
	require.NoError(t, err)
	assert.NotEmpty(t, transitions, "issue should have transitions")

	err = client.TransitionIssueTo(ctx, issue.Key, "In Progress")
	require.NoError(t, err)

	fetched, err := client.GetIssue(ctx, issue.Key)
	require.NoError(t, err)
	require.NotNil(t, fetched.Fields.Status)
	assert.Equal(t, "In Progress", fetched.Fields.Status.Name)
}

func TestJiraComments(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)

	body := fmt.Sprintf("test comment %s", testID())
	comment, err := client.AddComment(ctx, issue.Key, body)
	require.NoError(t, err)
	assert.Equal(t, body, comment.Body)
	assert.NotEmpty(t, comment.ID)

	comments, err := client.GetComments(ctx, issue.Key)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, body, comments[0].Body)
}

func TestJiraSearch(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)

	jql := fmt.Sprintf(
		"project = %s AND summary ~ %q",
		project, issue.Fields.Summary,
	)
	results, err := client.SearchIssues(ctx, jql, 10)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	found := false
	for _, r := range results {
		if r.Key == issue.Key {
			found = true
			break
		}
	}
	assert.True(t, found,
		"search should find issue %s", issue.Key,
	)
}
