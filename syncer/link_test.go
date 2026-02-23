package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJiraKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "valid markdown link prefix",
			content: "[DEVEX-123](https://example.atlassian.net/browse/DEVEX-123) My task",
			want:    "DEVEX-123",
		},
		{
			name:    "no prefix",
			content: "Just a plain task name",
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "link in middle of content is not matched",
			content: "Do [PROJ-1](https://x.atlassian.net/browse/PROJ-1) stuff",
			want:    "",
		},
		{
			name:    "underscore in project key",
			content: "[MY_PROJ-42](https://x.atlassian.net/browse/MY_PROJ-42) Task",
			want:    "MY_PROJ-42",
		},
		{
			name:    "http scheme",
			content: "[TEST-1](http://localhost/browse/TEST-1) Local task",
			want:    "TEST-1",
		},
		{
			name:    "link only, no title",
			content: "[PROJ-99](https://example.atlassian.net/browse/PROJ-99)",
			want:    "PROJ-99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractJiraKey(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripJiraPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "with prefix",
			content: "[DEVEX-123](https://example.atlassian.net/browse/DEVEX-123) My task",
			want:    "My task",
		},
		{
			name:    "no prefix",
			content: "Just a plain task name",
			want:    "Just a plain task name",
		},
		{
			name:    "prefix only",
			content: "[PROJ-1](https://x.atlassian.net/browse/PROJ-1)",
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StripJiraPrefix(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrependJiraLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		jiraKey     string
		jiraBaseURL string
		want        string
	}{
		{
			name:        "plain content",
			content:     "My task",
			jiraKey:     "DEVEX-42",
			jiraBaseURL: "https://example.atlassian.net",
			want:        "[DEVEX-42](https://example.atlassian.net/browse/DEVEX-42) My task",
		},
		{
			name:        "empty content",
			content:     "",
			jiraKey:     "PROJ-1",
			jiraBaseURL: "https://example.atlassian.net",
			want:        "[PROJ-1](https://example.atlassian.net/browse/PROJ-1)",
		},
		{
			name:        "replaces existing prefix",
			content:     "[OLD-1](https://example.atlassian.net/browse/OLD-1) My task",
			jiraKey:     "NEW-2",
			jiraBaseURL: "https://example.atlassian.net",
			want:        "[NEW-2](https://example.atlassian.net/browse/NEW-2) My task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PrependJiraLink(tt.content, tt.jiraKey, tt.jiraBaseURL)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	baseURL := "https://example.atlassian.net"
	content := "My original task"
	key := "DEVEX-99"

	linked := PrependJiraLink(content, key, baseURL)
	extractedKey := ExtractJiraKey(linked)
	assert.Equal(t, key, extractedKey)

	stripped := StripJiraPrefix(linked)
	assert.Equal(t, content, stripped)
}

func TestNormalizeJiraURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "bare domain",
			raw:  "example.atlassian.net",
			want: "https://example.atlassian.net",
		},
		{
			name: "with trailing slash",
			raw:  "https://example.atlassian.net/",
			want: "https://example.atlassian.net",
		},
		{
			name: "already normalized",
			raw:  "https://example.atlassian.net",
			want: "https://example.atlassian.net",
		},
		{
			name: "http scheme preserved",
			raw:  "http://localhost:8080/",
			want: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeJiraURL(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}
