package jira

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/todoist-jira-sync/config"
)

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

	cfg, err := config.Load()
	require.NoError(t, err)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	client, err := NewClient(cfg, logger)
	require.NoError(t, err)
	return client, os.Getenv("JIRA_PROJECT")
}

func testID() string {
	return uuid.New().String()[:8]
}

func createTestIssue(t *testing.T, client *Client, project string) *Issue {
	t.Helper()
	ctx := context.Background()

	summary := fmt.Sprintf("e2e-test-%s", testID())
	created, err := client.CreateIssue(ctx, &Issue{
		Fields: &IssueFields{
			Project:     &Project{Key: project},
			Summary:     summary,
			Description: TextToADF("e2e test issue"),
			IssueType:   &IssueType{Name: "Story"},
			Duedate:     time.Now().Format("2006-01-02"),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.DeleteIssue(context.Background(), created.Key)
		require.NoError(t, err)
	})

	issue, err := client.GetIssue(ctx, created.Key, nil)
	require.NoError(t, err)
	return issue
}

func TestJiraIssueCRUD(t *testing.T) { //nolint:paralleltest
	client, project := e2eSetup(t)
	ctx := context.Background()

	issue := createTestIssue(t, client, project)
	assert.NotEmpty(t, issue.Key)
	assert.Equal(t, "e2e test issue", ADFToText(issue.Fields.Description))
	assert.Equal(t, time.Now().Format("2006-01-02"), issue.Fields.Duedate)

	newSummary := fmt.Sprintf("e2e-test-updated-%s", testID())
	newDesc := "updated description"
	newDue := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	err := client.UpdateIssue(ctx, issue.Key, &Issue{
		Fields: &IssueFields{
			Summary:     newSummary,
			Description: TextToADF(newDesc),
			Duedate:     newDue,
		},
	})
	require.NoError(t, err)

	fetched, err := client.GetIssue(ctx, issue.Key, nil)
	require.NoError(t, err)
	assert.Equal(t, newSummary, fetched.Fields.Summary)
	assert.Equal(t, newDesc, ADFToText(fetched.Fields.Description))
	assert.Equal(t, newDue, fetched.Fields.Duedate)
}
