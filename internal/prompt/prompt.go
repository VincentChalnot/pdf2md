package prompt

import (
	"bytes"
	"text/template"
)

// DefaultSystemPrompt is the default system prompt for the LLM.
const DefaultSystemPrompt = `You are converting a page from a PDF document into clean Markdown.
The text was extracted with pdftotext -layout, which uses ASCII spacing
to approximate the visual layout of the page.

Rules:
- The page may have one or two columns: reorder the text in reading order
  (left column top-to-bottom first, then right column).
- Detect headings by visual prominence: ALL CAPS lines, short isolated lines,
  or lines with large surrounding whitespace. Convert to Markdown heading
  levels (##, ###, ####) based on relative importance.
- Convert ASCII-art tables into proper Markdown tables.
- Render key-value blocks (e.g. stat blocks) as bold key + value pairs
  or Markdown definition lists.
- Remove page artifacts: isolated page numbers, decorative separators
  (lines of dashes, underscores, or dots).
- Preserve all actual text content exactly — do not summarize, paraphrase,
  add or remove content.
- Output only the Markdown, no commentary, no code fences.`

var userTemplate = template.Must(template.New("user").Parse(
	`{{- if .Context}}[CONTEXT: last lines of previous page]
{{.Context}}
---
{{end}}[PAGE {{.PageNum}}]
{{.PageText}}
`))

// UserPromptData holds the data for the user prompt template.
type UserPromptData struct {
	Context  string
	PageNum  int
	PageText string
}

// BuildUserPrompt renders the user prompt template with the given data.
func BuildUserPrompt(data UserPromptData) (string, error) {
	var buf bytes.Buffer
	if err := userTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
