package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestADFToMarkdown(t *testing.T) {
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
			name: "heading level 1",
			adf:  `{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Title"}]},{"type":"paragraph","content":[{"type":"text","text":"Body"}]}]}`,
			want: "# Title\nBody",
		},
		{
			name: "heading level 3",
			adf:  `{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":3},"content":[{"type":"text","text":"Section"}]}]}`,
			want: "### Section",
		},
		{
			name: "heading without level defaults to h1",
			adf:  `{"type":"doc","version":1,"content":[{"type":"heading","content":[{"type":"text","text":"No Level"}]}]}`,
			want: "# No Level",
		},
		{
			name: "bullet list",
			adf:  `{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}]}]}`,
			want: "- first\n- second",
		},
		{
			name: "ordered list",
			adf:  `{"type":"doc","version":1,"content":[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"alpha"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"beta"}]}]}]}]}`,
			want: "1. alpha\n2. beta",
		},
		{
			name: "ordered list with start offset",
			adf:  `{"type":"doc","version":1,"content":[{"type":"orderedList","attrs":{"order":3},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"third"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"fourth"}]}]}]}]}`,
			want: "3. third\n4. fourth",
		},
		{
			name: "blockquote",
			adf:  `{"type":"doc","version":1,"content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted text"}]}]}]}`,
			want: "> quoted text",
		},
		{
			name: "code block without language",
			adf:  `{"type":"doc","version":1,"content":[{"type":"codeBlock","content":[{"type":"text","text":"fmt.Println()"}]}]}`,
			want: "```\nfmt.Println()\n```",
		},
		{
			name: "code block with language",
			adf:  `{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println()"}]}]}`,
			want: "```go\nfmt.Println()\n```",
		},
		{
			name: "horizontal rule",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"above"}]},{"type":"rule"},{"type":"paragraph","content":[{"type":"text","text":"below"}]}]}`,
			want: "above\n---\nbelow",
		},
		{
			name: "bold text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"this is "},{"type":"text","text":"bold","marks":[{"type":"strong"}]}]}]}`,
			want: "this is **bold**",
		},
		{
			name: "italic text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"this is "},{"type":"text","text":"italic","marks":[{"type":"em"}]}]}]}`,
			want: "this is *italic*",
		},
		{
			name: "inline code",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"run "},{"type":"text","text":"go build","marks":[{"type":"code"}]}]}]}`,
			want: "run `go build`",
		},
		{
			name: "strikethrough",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"this is "},{"type":"text","text":"removed","marks":[{"type":"strike"}]}]}]}`,
			want: "this is ~~removed~~",
		},
		{
			name: "bold and italic combined",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"important","marks":[{"type":"strong"},{"type":"em"}]}]}]}`,
			want: "***important***",
		},
		{
			name: "link with different text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"click here","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`,
			want: "[click here](https://example.com)",
		},
		{
			name: "link with same text as href",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"https://example.com","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`,
			want: "https://example.com",
		},
		{
			name: "inline card link",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Check this: "},{"type":"inlineCard","attrs":{"url":"https://github.com/org/repo/pull/78"}}]}]}`,
			want: "Check this: https://github.com/org/repo/pull/78",
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
		{
			name: "mention with text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"abc123","text":"@Name Of User"}},{"type":"text","text":" finished most of this"}]}]}`,
			want: "@Name Of User finished most of this",
		},
		{
			name: "mention without text",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"abc123"}},{"type":"text","text":" did something"}]}]}`,
			want: " did something",
		},
		{
			name: "mediaSingle image",
			adf:  `{"type":"doc","version":1,"content":[{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"abc-123","type":"file","collection":"proj"}}]}]}`,
			want: "[IMAGE, See Jira]",
		},
		{
			name: "mediaGroup with multiple images",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"See below:"}]},{"type":"mediaGroup","content":[{"type":"media","attrs":{"id":"img1","type":"file"}},{"type":"media","attrs":{"id":"img2","type":"file"}}]}]}`,
			want: "See below:\n[IMAGE, See Jira]",
		},
		{
			name: "mediaInline in paragraph",
			adf:  `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Check this "},{"type":"mediaInline","attrs":{"id":"abc","type":"file"}},{"type":"text","text":" screenshot"}]}]}`,
			want: "Check this [IMAGE, See Jira] screenshot",
		},
		{
			name: "full document with mixed blocks",
			adf: `{"type":"doc","version":1,"content":[
				{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Overview"}]},
				{"type":"paragraph","content":[{"type":"text","text":"Some "},{"type":"text","text":"bold","marks":[{"type":"strong"}]},{"type":"text","text":" text."}]},
				{"type":"bulletList","content":[
					{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item one"}]}]},
					{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item two"}]}]}
				]},
				{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(\"hello\")"}]}
			]}`,
			want: "## Overview\nSome **bold** text.\n- item one\n- item two\n```go\nfmt.Println(\"hello\")\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ADFToMarkdown(json.RawMessage(tt.adf))
			assert.Equal(t, tt.want, got)
		})
	}
}
