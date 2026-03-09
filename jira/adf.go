package jira

import (
	"encoding/json"
	"fmt"
	"strings"
)

// adfDoc is the top-level ADF document structure.
type adfDoc struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []adfNode `json:"content"`
}

// adfNode is a recursive node in an ADF document.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Content []adfNode `json:"content,omitempty"`
	Marks   []adfMark `json:"marks,omitempty"`
	Attrs   *adfAttrs `json:"attrs,omitempty"`
}

type adfMark struct {
	Type  string    `json:"type"`
	Attrs *adfAttrs `json:"attrs,omitempty"`
}

type adfAttrs struct {
	Href     string `json:"href,omitempty"`
	URL      string `json:"url,omitempty"`
	Text     string `json:"text,omitempty"`
	Level    int    `json:"level,omitempty"`
	Language string `json:"language,omitempty"`
	Order    int    `json:"order,omitempty"`
}

// TextToADF wraps plain text into an ADF document.
// Each line becomes a separate paragraph node.
func TextToADF(text string) json.RawMessage {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	paragraphs := make([]adfNode, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			paragraphs = append(paragraphs, adfNode{Type: "paragraph"})
			continue
		}
		paragraphs = append(paragraphs, adfNode{
			Type:    "paragraph",
			Content: []adfNode{{Type: "text", Text: line}},
		})
	}
	doc := adfDoc{Type: "doc", Version: 1, Content: paragraphs}
	b, _ := json.Marshal(doc)
	return b
}

// ADFToMarkdown converts an ADF document to markdown, preserving headings,
// lists, code blocks, blockquotes, and inline formatting marks.
func ADFToMarkdown(doc json.RawMessage) string {
	if len(doc) == 0 {
		return ""
	}
	var d adfDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return string(doc)
	}
	blocks := make([]string, 0, len(d.Content))
	for _, block := range d.Content {
		blocks = append(blocks, blockToMarkdown(block))
	}
	return strings.Join(blocks, "\n")
}

func blockToMarkdown(node adfNode) string {
	switch node.Type {
	case "heading":
		level := 1
		if node.Attrs != nil && node.Attrs.Level > 0 {
			level = node.Attrs.Level
		}
		return strings.Repeat("#", level) + " " + inlineContent(node.Content)

	case "bulletList":
		items := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			items = append(items, "- "+listItemContent(item))
		}
		return strings.Join(items, "\n")

	case "orderedList":
		start := 1
		if node.Attrs != nil && node.Attrs.Order > 0 {
			start = node.Attrs.Order
		}
		items := make([]string, 0, len(node.Content))
		for i, item := range node.Content {
			items = append(items, fmt.Sprintf("%d. %s", start+i, listItemContent(item)))
		}
		return strings.Join(items, "\n")

	case "blockquote":
		lines := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			lines = append(lines, "> "+blockToMarkdown(child))
		}
		return strings.Join(lines, "\n")

	case "codeBlock":
		lang := ""
		if node.Attrs != nil {
			lang = node.Attrs.Language
		}
		return "```" + lang + "\n" + inlineContent(node.Content) + "\n```"

	case "rule":
		return "---"

	default:
		return inlineContent(node.Content)
	}
}

func listItemContent(item adfNode) string {
	parts := make([]string, 0, len(item.Content))
	for _, child := range item.Content {
		parts = append(parts, blockToMarkdown(child))
	}
	return strings.Join(parts, "\n")
}

func inlineContent(nodes []adfNode) string {
	parts := make([]string, 0, len(nodes))
	for _, child := range nodes {
		if t := inlineToMarkdown(child); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}

func inlineToMarkdown(node adfNode) string {
	if node.Type == "text" {
		return applyMarks(node.Text, node.Marks)
	}
	if node.Type == "inlineCard" {
		if node.Attrs != nil && node.Attrs.URL != "" {
			return node.Attrs.URL
		}
		return ""
	}
	if node.Type == "mention" {
		if node.Attrs != nil && node.Attrs.Text != "" {
			return node.Attrs.Text
		}
		return ""
	}
	if node.Type == "hardBreak" {
		return "\n"
	}
	return inlineContent(node.Content)
}

func applyMarks(text string, marks []adfMark) string {
	for _, m := range marks {
		switch m.Type {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			if m.Attrs != nil && m.Attrs.Href != "" && m.Attrs.Href != text {
				text = "[" + text + "](" + m.Attrs.Href + ")"
			}
		}
	}
	return text
}
