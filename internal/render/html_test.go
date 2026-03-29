package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func TestHTMLBasicOutput(t *testing.T) {
	doc := &model.Document{
		Source: "test.pdf",
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12, Family: "Times", Role: model.RoleBody},
			"1": {ID: "1", Size: 24, Family: "Arial", Role: model.RoleH1},
		},
		Pages: []model.Page{
			{
				Number: 1, Width: 800, Height: 600,
				Elements: []model.Element{
					{Top: 10, Left: 20, Width: 200, Height: 30, FontID: "1", Role: model.RoleH1, Text: "Title"},
					{Top: 50, Left: 20, Width: 400, Height: 20, FontID: "0", Role: model.RoleBody, Text: "Body text"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// Check it's a valid HTML document.
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("output should contain DOCTYPE")
	}
	if !strings.Contains(out, "<html") {
		t.Error("output should contain <html>")
	}
	if !strings.Contains(out, "</html>") {
		t.Error("output should contain </html>")
	}

	// Check SVG structure.
	if !strings.Contains(out, "<svg") {
		t.Error("output should contain <svg>")
	}
	if !strings.Contains(out, `width="800"`) {
		t.Error("output should contain SVG width")
	}
	if !strings.Contains(out, `height="600"`) {
		t.Error("output should contain SVG height")
	}

	// Check text elements with textLength.
	if !strings.Contains(out, "textLength=") {
		t.Error("output should contain textLength attribute")
	}
	if !strings.Contains(out, `lengthAdjust="spacingAndGlyphs"`) {
		t.Error("output should contain lengthAdjust attribute")
	}

	// Check text content.
	if !strings.Contains(out, ">Title</text>") {
		t.Error("output should contain 'Title' text element")
	}
	if !strings.Contains(out, ">Body text</text>") {
		t.Error("output should contain 'Body text' text element")
	}

	// Check CSS classes.
	if !strings.Contains(out, `class="h1"`) {
		t.Error("output should contain class h1")
	}
	if !strings.Contains(out, `class="body"`) {
		t.Error("output should contain class body")
	}

	// Check no JavaScript.
	if strings.Contains(out, "<script") {
		t.Error("output must not contain JavaScript")
	}
}

func TestHTMLTextPositioning(t *testing.T) {
	doc := &model.Document{
		Source: "pos.pdf",
		Pages: []model.Page{
			{
				Number: 1, Width: 100, Height: 200,
				Elements: []model.Element{
					{Top: 10, Left: 5, Width: 80, Height: 15, Role: model.RoleBody, Text: "Hello"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// y should be Top + Height = 10 + 15 = 25
	if !strings.Contains(out, `x="5"`) {
		t.Error("x should be element.Left (5)")
	}
	if !strings.Contains(out, `y="25"`) {
		t.Error("y should be element.Top + element.Height (25)")
	}
	if !strings.Contains(out, `textLength="80"`) {
		t.Error("textLength should be element.Width (80)")
	}
}

func TestHTMLMultiplePages(t *testing.T) {
	doc := &model.Document{
		Source: "multi.pdf",
		Pages: []model.Page{
			{Number: 1, Width: 100, Height: 200, Elements: []model.Element{
				{Top: 10, Left: 5, Width: 80, Height: 15, Role: model.RoleBody, Text: "Page 1"},
			}},
			{Number: 2, Width: 100, Height: 200, Elements: []model.Element{
				{Top: 10, Left: 5, Width: 80, Height: 15, Role: model.RoleBody, Text: "Page 2"},
			}},
		},
	}

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()
	if strings.Count(out, "<div class=\"page\">") != 2 {
		t.Error("should have 2 page divs")
	}
	if strings.Count(out, "<svg") != 2 {
		t.Error("should have 2 SVG elements")
	}
}

func TestHTMLEscaping(t *testing.T) {
	doc := &model.Document{
		Source: "test<>&.pdf",
		Pages: []model.Page{
			{Number: 1, Width: 100, Height: 100, Elements: []model.Element{
				{Top: 10, Left: 5, Width: 80, Height: 15, Role: model.RoleBody, Text: "a < b & c > d"},
			}},
		},
	}

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "test&lt;&gt;&amp;.pdf") {
		t.Error("source name should be HTML-escaped in title")
	}
	if !strings.Contains(out, "a &lt; b &amp; c &gt; d") {
		t.Error("element text should be HTML-escaped")
	}
}

func TestHTMLEmptyDocument(t *testing.T) {
	doc := &model.Document{
		Source: "empty.pdf",
		Pages: []model.Page{},
	}

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("empty document should still produce valid HTML")
	}
	if !strings.Contains(out, "</html>") {
		t.Error("empty document should still close HTML")
	}
}
