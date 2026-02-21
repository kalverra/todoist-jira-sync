package syncer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractJiraKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "with valid footer",
			description: "Some task description\n\n---\nsynced-jira-key: PROJ-123\nsynced-at: 2025-01-01T00:00:00Z",
			want:        "PROJ-123",
		},
		{
			name:        "no footer",
			description: "Just a plain description",
			want:        "",
		},
		{
			name:        "empty description",
			description: "",
			want:        "",
		},
		{
			name:        "multi-word project key",
			description: "\n\n---\nsynced-jira-key: MY_PROJ-42\nsynced-at: 2025-01-01T00:00:00Z",
			want:        "MY_PROJ-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractJiraKey(tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractTodoistID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "with valid footer",
			description: "Issue description\n\n---\nsynced-todoist-id: 8675309\nsynced-at: 2025-01-01T00:00:00Z",
			want:        "8675309",
		},
		{
			name:        "no footer",
			description: "Just a plain description",
			want:        "",
		},
		{
			name:        "empty description",
			description: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractTodoistID(tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractSyncedAt(t *testing.T) {
	t.Parallel()

	t.Run("valid timestamp", func(t *testing.T) {
		t.Parallel()
		desc := "content\n\n---\nsynced-jira-key: PROJ-1\nsynced-at: 2025-06-15T10:30:00Z"
		got, ok := ExtractSyncedAt(desc)
		require.True(t, ok)
		expected := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
		assert.Equal(t, expected, got)
	})

	t.Run("no synced-at", func(t *testing.T) {
		t.Parallel()
		_, ok := ExtractSyncedAt("plain description")
		assert.False(t, ok)
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		t.Parallel()
		_, ok := ExtractSyncedAt("synced-at: not-a-date")
		assert.False(t, ok)
	})
}

func TestUserDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "with footer",
			description: "User content here\n\n---\nsynced-jira-key: PROJ-1\nsynced-at: 2025-01-01T00:00:00Z",
			want:        "User content here",
		},
		{
			name:        "no footer",
			description: "Just user content",
			want:        "Just user content",
		},
		{
			name:        "empty",
			description: "",
			want:        "",
		},
		{
			name:        "multiline user content with footer",
			description: "Line 1\nLine 2\nLine 3\n\n---\nsynced-todoist-id: 123\nsynced-at: 2025-01-01T00:00:00Z",
			want:        "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := UserDescription(tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetJiraKeyRoundTrip(t *testing.T) {
	t.Parallel()

	syncTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("add to empty description", func(t *testing.T) {
		t.Parallel()
		result := SetJiraKey("", "PROJ-42", syncTime)
		assert.Equal(t, "PROJ-42", ExtractJiraKey(result))
		assert.Empty(t, UserDescription(result))
		got, ok := ExtractSyncedAt(result)
		require.True(t, ok)
		assert.Equal(t, syncTime, got)
	})

	t.Run("add to existing description", func(t *testing.T) {
		t.Parallel()
		result := SetJiraKey("My task notes", "TEST-99", syncTime)
		assert.Equal(t, "TEST-99", ExtractJiraKey(result))
		assert.Equal(t, "My task notes", UserDescription(result))
	})

	t.Run("overwrite existing footer", func(t *testing.T) {
		t.Parallel()
		first := SetJiraKey("Notes", "OLD-1", syncTime)
		newTime := syncTime.Add(time.Hour)
		second := SetJiraKey(first, "NEW-2", newTime)
		assert.Equal(t, "NEW-2", ExtractJiraKey(second))
		assert.Equal(t, "Notes", UserDescription(second))
		got, ok := ExtractSyncedAt(second)
		require.True(t, ok)
		assert.Equal(t, newTime, got)
	})
}

func TestSetTodoistIDRoundTrip(t *testing.T) {
	t.Parallel()

	syncTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("add to empty description", func(t *testing.T) {
		t.Parallel()
		result := SetTodoistID("", "12345678", syncTime)
		assert.Equal(t, "12345678", ExtractTodoistID(result))
		assert.Empty(t, UserDescription(result))
	})

	t.Run("add to existing description", func(t *testing.T) {
		t.Parallel()
		result := SetTodoistID("Issue body", "99887766", syncTime)
		assert.Equal(t, "99887766", ExtractTodoistID(result))
		assert.Equal(t, "Issue body", UserDescription(result))
	})

	t.Run("overwrite existing footer", func(t *testing.T) {
		t.Parallel()
		first := SetTodoistID("Body", "111", syncTime)
		newTime := syncTime.Add(2 * time.Hour)
		second := SetTodoistID(first, "222", newTime)
		assert.Equal(t, "222", ExtractTodoistID(second))
		assert.Equal(t, "Body", UserDescription(second))
		got, ok := ExtractSyncedAt(second)
		require.True(t, ok)
		assert.Equal(t, newTime, got)
	})
}
