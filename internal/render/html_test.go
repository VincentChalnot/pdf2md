package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func makeDoc(source string, pages []model.Page) *model.Document {
	return &model.Document{
		Source:  source,
		FontMap: make(map[string]model.FontSpec),
		Pages:   pages,
	}
}

func TestHTMLBasicOutput(t *testing.T) {
	doc := makeDoc("test.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				{
					XMin: 20, YMin: 10, XMax: 220, YMax: 40,
					Lines: []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Title"},
					},
				},
				{
					XMin: 20, YMin: 50, XMax: 420, YMax: 70,
					Lines: []model.Line{
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Body text"},
					},
				},
			},
		},
	})

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

	// Check font-size attribute.
	if !strings.Contains(out, "font-size=") {
		t.Error("output should contain font-size attribute")
	}

	// Check letter-spacing attribute.
	if !strings.Contains(out, `letter-spacing="0.02em"`) {
		t.Error("output should contain letter-spacing attribute")
	}

	// Check no JavaScript.
	if strings.Contains(out, "<script") {
		t.Error("output must not contain JavaScript")
	}
}

func TestHTMLTextPositioning(t *testing.T) {
	doc := makeDoc("pos.pdf", []model.Page{
		{
			Number: 1, Width: 100, Height: 200,
			Flows: []model.Flow{
				{
					XMin: 5, YMin: 10, XMax: 85, YMax: 25,
					Lines: []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Hello"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, `x="5"`) {
		t.Error("x should be line.XMin (5)")
	}
	if !strings.Contains(out, `y="25"`) {
		t.Error("y should be line.YMax (25)")
	}
	if !strings.Contains(out, `textLength="80"`) {
		t.Error("textLength should be line.XMax - line.XMin (80)")
	}
}

func TestHTMLMultiplePages(t *testing.T) {
	doc := makeDoc("multi.pdf", []model.Page{
		{
			Number: 1, Width: 100, Height: 200,
			Flows: []model.Flow{
				{
					XMin: 5, YMin: 10, XMax: 85, YMax: 25,
					Lines: []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Page 1"},
					},
				},
			},
		},
		{
			Number: 2, Width: 100, Height: 200,
			Flows: []model.Flow{
				{
					XMin: 5, YMin: 10, XMax: 85, YMax: 25,
					Lines: []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Page 2"},
					},
				},
			},
		},
	})

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
	doc := makeDoc("test<>&.pdf", []model.Page{
		{
			Number: 1, Width: 100, Height: 100,
			Flows: []model.Flow{
				{
					XMin: 5, YMin: 10, XMax: 85, YMax: 25,
					Lines: []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "a < b & c > d"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "test&lt;&gt;&amp;.pdf") {
		t.Error("source name should be HTML-escaped in title")
	}
	if !strings.Contains(out, "a &lt; b &amp; c &gt; d") {
		t.Error("line text should be HTML-escaped")
	}
}

func TestHTMLEmptyDocument(t *testing.T) {
	doc := makeDoc("empty.pdf", []model.Page{})

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

func TestHTMLFontSizes(t *testing.T) {
	doc := makeDoc("sizes.pdf", []model.Page{
		{
			Number: 1, Width: 100, Height: 200,
			Flows: []model.Flow{
				{
					XMin: 5, YMin: 10, XMax: 85, YMax: 50,
					Lines: []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 50, FontSize: 24, Role: model.RoleH1, Text: "Big"},
					},
				},
				{
					XMin: 5, YMin: 60, XMax: 85, YMax: 70,
					Lines: []model.Line{
						{XMin: 5, YMin: 60, XMax: 85, YMax: 70, FontSize: 8, Role: model.RoleSmall, Text: "Small"},
					},
				},
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// Font-size should be (YMax - YMin) * 0.9
	// For "Big": (50-10)*0.9 = 36
	if !strings.Contains(out, `font-size="36"`) {
		t.Error("Big text should have font-size 36")
	}
	// For "Small": (70-60)*0.9 = 9
	if !strings.Contains(out, `font-size="9"`) {
		t.Error("Small text should have font-size 9")
	}
}

func TestHTMLViewPortSVG(t *testing.T) {
	doc := makeDoc("viewport.pdf", []model.Page{
		{
			Number: 1, Width: 500, Height: 300,
			Flows: []model.Flow{},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// SVGs should have viewport-relative sizing via CSS
	if !strings.Contains(out, "width: 100%") {
		t.Error("SVG CSS should have width: 100%")
	}
	if !strings.Contains(out, "height: auto") {
		t.Error("SVG CSS should have height: auto")
	}
}
