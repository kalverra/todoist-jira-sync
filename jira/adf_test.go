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
		{
			name: "inline card link",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Check this: "},{"type":"inlineCard","attrs":{"url":"https://github.com/org/repo/pull/78"}}]}]}`,
			want: "Check this: https://github.com/org/repo/pull/78",
		},
		{
			name: "text with link mark same as text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"https://example.com","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`,
			want: "https://example.com",
		},
		{
			name: "text with link mark different from text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"click here","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`,
			want: "click here https://example.com",
		},
		{
			name: "hard break",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"before"},{"type":"hardBreak"},{"type":"text","text":"after"}]}]}`,
			want: "before\nafter",
		},
		{
			name: "inline card without url",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"prefix "},{"type":"inlineCard","attrs":{}}]}]}`,
			want: "prefix ",
		},
		{
			name: "mixed text and inline card",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"PR to add docs: "},{"type":"inlineCard","attrs":{"url":"https://github.com/smartcontractkit/central-docs/pull/78"}}]}]}`,
			want: "PR to add docs: https://github.com/smartcontractkit/central-docs/pull/78",
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
