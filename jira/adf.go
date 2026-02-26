package jira

import (
	"encoding/json"
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

// ADFToText extracts all text content from an ADF document, joining
// top-level blocks (paragraphs, headings, etc.) with newlines.
func ADFToText(doc json.RawMessage) string {
	if len(doc) == 0 {
		return ""
	}
	var d adfDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return string(doc)
	}
	lines := make([]string, 0, len(d.Content))
	for _, block := range d.Content {
		lines = append(lines, extractText(block))
	}
	return strings.Join(lines, "\n")
}

func extractText(node adfNode) string {
	if node.Type == "text" {
		return node.Text
	}
	if len(node.Content) == 0 {
		return ""
	}
	parts := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		if t := extractText(child); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}
