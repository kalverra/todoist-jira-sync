package jira

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
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

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	client, err := NewClient(
		os.Getenv("JIRA_URL"),
		os.Getenv("JIRA_EMAIL"),
		os.Getenv("JIRA_API_TOKEN"),
		logger,
	)
	require.NoError(t, err)
	return client, os.Getenv("JIRA_PROJECT")
}

func testID() string {
	return uuid.New().String()[:8]
}

func createTestIssue(
	t *testing.T,
	client *Client,
	project string,
) *cloud.Issue {
	t.Helper()
	ctx := context.Background()

	summary := fmt.Sprintf("e2e-test-%s", testID())
	created, _, err := client.Issue.Create(ctx, &cloud.Issue{
		Fields: &cloud.IssueFields{
			Project:     cloud.Project{Key: project},
			Summary:     summary,
			Description: "e2e test issue",
			Type:        cloud.IssueType{Name: "Story"},
			Duedate:     cloud.Date(time.Now()),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := client.Issue.Delete(context.Background(), created.Key)
		require.NoError(t, err)
	})

	issue, _, err := client.Issue.Get(ctx, created.Key, nil)
	require.NoError(t, err)
	return issue
}

func TestJiraIssueCRUD(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)
	assert.NotEmpty(t, issue.Key)
	assert.Equal(t, "e2e test issue", issue.Fields.Description)
	assert.Equal(t, time.Now(), issue.Fields.Duedate)

	newSummary := fmt.Sprintf("e2e-test-updated-%s", testID())
	newDesc := "updated description"
	newDue := cloud.Date(time.Now().AddDate(0, 0, 1))
	issue.Fields.Summary = newSummary
	issue.Fields.Description = newDesc
	issue.Fields.Duedate = cloud.Date(time.Now())

	_, _, err := client.Issue.Update(ctx, issue, nil)
	require.NoError(t, err)

	fetched, _, err := client.Issue.Get(ctx, issue.Key, nil)
	require.NoError(t, err)
	assert.Equal(t, newSummary, fetched.Fields.Summary)
	assert.Equal(t, newDesc, fetched.Fields.Description)
	assert.Equal(t, newDue, fetched.Fields.Duedate)
}
