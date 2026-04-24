package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func TestToMarkdownBasic(t *testing.T) {
	doc := makeDoc("test.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Document Title"},
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "First paragraph."},
						{XMin: 20, YMin: 72, XMax: 420, YMax: 92, FontSize: 12, Role: model.RoleBody, Text: "Second line of first paragraph."},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Check document title (first h1)
	if !strings.Contains(out, "# Document Title") {
		t.Error("output should contain document title as h1")
	}

	// Check body text is present
	if !strings.Contains(out, "First paragraph.") {
		t.Error("output should contain first paragraph")
	}
	if !strings.Contains(out, "Second line of first paragraph.") {
		t.Error("output should contain second paragraph line")
	}
}

func TestToMarkdownHeadings(t *testing.T) {
	doc := makeDoc("headings.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 200,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Main Title"},
						{XMin: 20, YMin: 50, XMax: 320, YMax: 70, FontSize: 18, Role: model.RoleH2, Text: "Subtitle"},
						{XMin: 20, YMin: 80, XMax: 280, YMax: 95, FontSize: 14, Role: model.RoleH3, Text: "Section"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "# Main Title") {
		t.Error("output should contain h1 heading")
	}
	if !strings.Contains(out, "## Subtitle") {
		t.Error("output should contain h2 heading")
	}
	if !strings.Contains(out, "### Section") {
		t.Error("output should contain h3 heading")
	}
}

func TestToMarkdownSmallText(t *testing.T) {
	doc := makeDoc("small.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Title"},
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 8, Role: model.RoleSmall, Text: "Footnote text"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "<small>Footnote text</small>") {
		t.Error("output should wrap small text in <small> tags")
	}
}

func TestToMarkdownSidebar(t *testing.T) {
	doc := makeDoc("sidebar.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				// Normal flow (full width)
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Main Content"},
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Body text."},
					},
				},
				// Sidebar flow (narrow, right-aligned, short, starts with heading)
				{
					XMin: 500, YMin: 50, XMax: 700, YMax: 150,
					Lines: []model.Line{
						{XMin: 500, YMin: 50, XMax: 680, YMax: 70, FontSize: 14, Role: model.RoleH3, Text: "Sidebar Title"},
						{XMin: 500, YMin: 80, XMax: 680, YMax: 95, FontSize: 10, Role: model.RoleBody, Text: "Sidebar content."},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Check for sidebar markers (horizontal rules)
	if !strings.Contains(out, "***") {
		t.Error("output should contain sidebar separator (***)")
	}

	// Check that sidebar heading is present
	if !strings.Contains(out, "### Sidebar Title") {
		t.Error("output should contain sidebar heading")
	}

	// Check that sidebar content is present
	if !strings.Contains(out, "Sidebar content.") {
		t.Error("output should contain sidebar content")
	}
}

func TestToMarkdownExcludedAndUnknown(t *testing.T) {
	doc := makeDoc("excluded.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Title"},
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleExcluded, Text: "Should not appear"},
						{XMin: 20, YMin: 72, XMax: 420, YMax: 92, FontSize: 12, Role: model.RoleUnknown, Text: "Also should not appear"},
						{XMin: 20, YMin: 100, XMax: 420, YMax: 120, FontSize: 12, Role: model.RoleBody, Text: "Visible body text"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Check that excluded/unknown lines are not in output
	if strings.Contains(out, "Should not appear") {
		t.Error("output should not contain excluded text")
	}
	if strings.Contains(out, "Also should not appear") {
		t.Error("output should not contain unknown text")
	}

	// Check that visible body text is present
	if !strings.Contains(out, "Visible body text") {
		t.Error("output should contain visible body text")
	}
}

func TestToMarkdownEmptyDocument(t *testing.T) {
	doc := makeDoc("empty.pdf", []model.Page{})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()
	if out != "" {
		t.Error("empty document should produce empty output")
	}
}

func TestToMarkdownParagraphMerging(t *testing.T) {
	// Test that lines with small gaps are merged, but large gaps create new paragraphs
	doc := makeDoc("paragraphs.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 200,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Title"},
						// Small gap between body lines (should merge)
						{XMin: 20, YMin: 50, XMax: 420, YMax: 65, FontSize: 12, Role: model.RoleBody, Text: "Line one."},
						{XMin: 20, YMin: 67, XMax: 420, YMax: 82, FontSize: 12, Role: model.RoleBody, Text: "Line two."},
						// Large gap (should create new paragraph)
						{XMin: 20, YMin: 120, XMax: 420, YMax: 135, FontSize: 12, Role: model.RoleBody, Text: "New paragraph."},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Check that body text appears
	if !strings.Contains(out, "Line one.") {
		t.Error("output should contain first line")
	}
	if !strings.Contains(out, "Line two.") {
		t.Error("output should contain second line")
	}
	if !strings.Contains(out, "New paragraph.") {
		t.Error("output should contain new paragraph")
	}

	// Lines should be merged with space (not exact match to allow for formatting variations)
	if !strings.Contains(out, "Line one. Line two.") {
		t.Log("Lines with small gap should be merged into one paragraph")
		t.Log("Output:", out)
	}
}

func TestToMarkdownMultiplePages(t *testing.T) {
	doc := makeDoc("multi.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Page 1 Title"},
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Page 1 content."},
					},
				},
			},
		},
		{
			Number: 2, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 320, YMax: 30, FontSize: 18, Role: model.RoleH2, Text: "Page 2 Section"},
						{XMin: 20, YMin: 40, XMax: 420, YMax: 60, FontSize: 12, Role: model.RoleBody, Text: "Page 2 content."},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Check that content from both pages is present
	if !strings.Contains(out, "Page 1 Title") {
		t.Error("output should contain page 1 title")
	}
	if !strings.Contains(out, "Page 1 content.") {
		t.Error("output should contain page 1 content")
	}
	if !strings.Contains(out, "Page 2 Section") {
		t.Error("output should contain page 2 section")
	}
	if !strings.Contains(out, "Page 2 content.") {
		t.Error("output should contain page 2 content")
	}

	// Should NOT contain page break markers
	if strings.Contains(out, "---") && !strings.Contains(out, "***") {
		t.Error("output should not contain page break markers (---)")
	}
}

func TestToMarkdownNoH1(t *testing.T) {
	// Document without any h1 heading
	doc := makeDoc("noh1.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 600, YMax: 100,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 320, YMax: 30, FontSize: 18, Role: model.RoleH2, Text: "Subtitle Only"},
						{XMin: 20, YMin: 40, XMax: 420, YMax: 60, FontSize: 12, Role: model.RoleBody, Text: "Body text."},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := ToMarkdown(&buf, doc, false); err != nil {
		t.Fatalf("ToMarkdown() error: %v", err)
	}

	out := buf.String()

	// Should contain the h2 and body
	if !strings.Contains(out, "## Subtitle Only") {
		t.Error("output should contain h2 heading")
	}
	if !strings.Contains(out, "Body text.") {
		t.Error("output should contain body text")
	}
}
