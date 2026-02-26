package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextToADF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "empty",
			text: "",
			want: "",
		},
		{
			name: "single line",
			text: "hello world",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello world"}]}]}`,
		},
		{
			name: "multiple lines",
			text: "line one\nline two",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"line one"}]},{"type":"paragraph","content":[{"type":"text","text":"line two"}]}]}`,
		},
		{
			name: "blank line creates empty paragraph",
			text: "before\n\nafter",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"before"}]},{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"after"}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TextToADF(tt.text)
			if tt.want == "" {
				assert.Nil(t, got)
			} else {
				assert.JSONEq(t, tt.want, string(got))
			}
		})
	}
}

func TestADFToText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		adf  string
		want string
	}{
		{
			name: "empty",
			adf:  "",
			want: "",
		},
		{
			name: "single paragraph",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
			want: "hello",
		},
		{
			name: "multiple paragraphs",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"line 1"}]},{"type":"paragraph","content":[{"type":"text","text":"line 2"}]}]}`,
			want: "line 1\nline 2",
		},
		{
			name: "empty paragraph",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"before"}]},{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"after"}]}]}`,
			want: "before\n\nafter",
		},
		{
			name: "nested content",
			adf:  `{"type":"doc","version":1,"content":[{"type":"heading","content":[{"type":"text","text":"Title"}]},{"type":"paragraph","content":[{"type":"text","text":"Body"}]}]}`,
			want: "Title\nBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ADFToText(json.RawMessage(tt.adf))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	text := "hello world\nsecond line"
	adf := TextToADF(text)
	require.NotNil(t, adf)
	got := ADFToText(adf)
	assert.Equal(t, text, got)
}
