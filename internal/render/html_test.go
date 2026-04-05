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

// makeBlock is a helper to create a Block with lines.
func makeBlock(xMin, yMin, xMax, yMax float64, lines []model.Line) model.Block {
	return model.Block{
		XMin:  xMin,
		YMin:  yMin,
		XMax:  xMax,
		YMax:  yMax,
		Lines: lines,
	}
}

// makeFlow is a helper to create a Flow with blocks.
func makeFlow(xMin, yMin, xMax, yMax float64, blocks []model.Block) model.Flow {
	// Also populate Lines for backward compatibility
	var allLines []model.Line
	for _, block := range blocks {
		allLines = append(allLines, block.Lines...)
	}
	return model.Flow{
		XMin:   xMin,
		YMin:   yMin,
		XMax:   xMax,
		YMax:   yMax,
		Blocks: blocks,
		Lines:  allLines,
	}
}

func TestHTMLBasicOutput(t *testing.T) {
	doc := makeDoc("test.pdf", []model.Page{
		{
			Number: 1, Width: 800, Height: 600,
			Flows: []model.Flow{
				makeFlow(20, 10, 220, 40, []model.Block{
					makeBlock(20, 10, 220, 40, []model.Line{
						{XMin: 20, YMin: 10, XMax: 220, YMax: 40, FontSize: 24, Role: model.RoleH1, Text: "Title"},
					}),
				}),
				makeFlow(20, 50, 420, 70, []model.Block{
					makeBlock(20, 50, 420, 70, []model.Line{
						{XMin: 20, YMin: 50, XMax: 420, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Body text"},
					}),
				}),
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
				makeFlow(5, 10, 85, 25, []model.Block{
					makeBlock(5, 10, 85, 25, []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Hello"},
					}),
				}),
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
				makeFlow(5, 10, 85, 25, []model.Block{
					makeBlock(5, 10, 85, 25, []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Page 1"},
					}),
				}),
			},
		},
		{
			Number: 2, Width: 100, Height: 200,
			Flows: []model.Flow{
				makeFlow(5, 10, 85, 25, []model.Block{
					makeBlock(5, 10, 85, 25, []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "Page 2"},
					}),
				}),
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
				makeFlow(5, 10, 85, 25, []model.Block{
					makeBlock(5, 10, 85, 25, []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 25, FontSize: 12, Role: model.RoleBody, Text: "a < b & c > d"},
					}),
				}),
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
				makeFlow(5, 10, 85, 50, []model.Block{
					makeBlock(5, 10, 85, 50, []model.Line{
						{XMin: 5, YMin: 10, XMax: 85, YMax: 50, FontSize: 24, Role: model.RoleH1, Text: "Big"},
					}),
				}),
				makeFlow(5, 60, 85, 70, []model.Block{
					makeBlock(5, 60, 85, 70, []model.Line{
						{XMin: 5, YMin: 60, XMax: 85, YMax: 70, FontSize: 8, Role: model.RoleSmall, Text: "Small"},
					}),
				}),
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

func TestHTMLDebugOverlays(t *testing.T) {
	doc := makeDoc("debug.pdf", []model.Page{
		{
			Number: 1, Width: 200, Height: 300,
			Flows: []model.Flow{
				makeFlow(10, 20, 190, 100, []model.Block{
					// Block with heading (line height 20pt > 14pt threshold)
					makeBlock(10, 20, 90, 50, []model.Line{
						{XMin: 10, YMin: 30, XMax: 90, YMax: 50, FontSize: 24, Role: model.RoleH1, Text: "Header"},
					}),
					// Block with normal text (line heights 12pt < 14pt threshold)
					makeBlock(10, 60, 90, 100, []model.Line{
						{XMin: 10, YMin: 68, XMax: 90, YMax: 80, FontSize: 12, Role: model.RoleBody, Text: "Line 1"},
						{XMin: 10, YMin: 88, XMax: 90, YMax: 100, FontSize: 12, Role: model.RoleBody, Text: "Line 2"},
					}),
				}),
				makeFlow(100, 20, 190, 50, []model.Block{
					// Block with normal text (line height 12pt < 14pt threshold)
					makeBlock(100, 20, 190, 50, []model.Line{
						{XMin: 100, YMin: 38, XMax: 190, YMax: 50, FontSize: 12, Role: model.RoleBody, Text: "Sidebar"},
					}),
				}),
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// Check for debug overlay layers
	if !strings.Contains(out, `<g class="debug-flow-layer">`) {
		t.Error("output should contain debug-flow-layer")
	}
	if !strings.Contains(out, `<g class="debug-block-layer">`) {
		t.Error("output should contain debug-block-layer")
	}
	if !strings.Contains(out, `<g class="debug-line-layer">`) {
		t.Error("output should contain debug-line-layer")
	}
	if !strings.Contains(out, `<g class="text-layer">`) {
		t.Error("output should contain text-layer")
	}

	// Check for flow rectangles with labels
	if !strings.Contains(out, `class="debug-flow"`) {
		t.Error("output should contain debug-flow class for flow rectangles")
	}
	if !strings.Contains(out, ">F0</text>") {
		t.Error("output should contain flow label F0")
	}
	if !strings.Contains(out, ">F1</text>") {
		t.Error("output should contain flow label F1")
	}

	// Check for block rectangles with labels
	// Note: First block (H1 with 30pt line height) will be marked as heading
	if !strings.Contains(out, `class="debug-block"`) {
		t.Error("output should contain debug-block class for non-heading block rectangles")
	}
	if !strings.Contains(out, `class="debug-heading"`) {
		t.Error("output should contain debug-heading class for heading block")
	}
	if !strings.Contains(out, ">H0</text>") {
		t.Error("output should contain heading label H0 for first block")
	}
	if !strings.Contains(out, ">B1</text>") {
		t.Error("output should contain block label B1")
	}
	if !strings.Contains(out, ">B2</text>") {
		t.Error("output should contain block label B2")
	}

	// Check for line rectangles (no labels)
	if !strings.Contains(out, `class="debug-line"`) {
		t.Error("output should contain debug-line class for line rectangles")
	}

	// Verify there are 4 line rectangles (1 + 2 + 1)
	lineCount := strings.Count(out, `class="debug-line"`)
	if lineCount != 4 {
		t.Errorf("expected 4 line rectangles, got %d", lineCount)
	}

	// Check CSS styles for debug overlays
	if !strings.Contains(out, ".debug-flow") {
		t.Error("CSS should contain .debug-flow style")
	}
	if !strings.Contains(out, ".debug-block") {
		t.Error("CSS should contain .debug-block style")
	}
	if !strings.Contains(out, ".debug-line") {
		t.Error("CSS should contain .debug-line style")
	}
	if !strings.Contains(out, ".debug-label") {
		t.Error("CSS should contain .debug-label style")
	}
}

func TestHTMLLayoutDetectionOverlays(t *testing.T) {
	// Create a page with two-column layout to test layout detection
	doc := makeDoc("layout-test.pdf", []model.Page{
		{
			Number: 1, Width: 500, Height: 700,
			Flows: []model.Flow{
				makeFlow(50, 50, 450, 400, []model.Block{
					// Header (single column) - line height 20pt (> 14pt, will be marked as heading)
					makeBlock(50, 50, 450, 100, []model.Line{
						{XMin: 50, YMin: 70, XMax: 450, YMax: 90, FontSize: 24, Role: model.RoleH1, Text: "Title"},
					}),
					// Body row 1 (two columns) - line height 12pt (< 14pt, normal text)
					makeBlock(50, 150, 230, 200, []model.Line{
						{XMin: 50, YMin: 188, XMax: 230, YMax: 200, FontSize: 12, Role: model.RoleBody, Text: "Left column"},
					}),
					makeBlock(270, 150, 450, 200, []model.Line{
						{XMin: 270, YMin: 188, XMax: 450, YMax: 200, FontSize: 12, Role: model.RoleBody, Text: "Right column"},
					}),
					// Body row 2 (two columns) - line height 12pt (< 14pt, normal text)
					makeBlock(50, 250, 230, 300, []model.Line{
						{XMin: 50, YMin: 288, XMax: 230, YMax: 300, FontSize: 12, Role: model.RoleBody, Text: "Left 2"},
					}),
					makeBlock(270, 250, 450, 300, []model.Line{
						{XMin: 270, YMin: 288, XMax: 450, YMax: 300, FontSize: 12, Role: model.RoleBody, Text: "Right 2"},
					}),
					// Footer (single column) - line height 10pt (< 14pt, normal text)
					makeBlock(50, 350, 450, 400, []model.Line{
						{XMin: 50, YMin: 390, XMax: 450, YMax: 400, FontSize: 10, Role: model.RoleSmall, Text: "Footer"},
					}),
				}),
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// Check for layout detection layers
	if !strings.Contains(out, `<g class="debug-layout">`) {
		t.Error("output should contain debug-layout layer for zones")
	}
	if !strings.Contains(out, `<g class="debug-band-layer">`) {
		t.Error("output should contain debug-band-layer")
	}
	if !strings.Contains(out, `<g class="debug-horizontal-cuts">`) {
		t.Error("output should contain debug-horizontal-cuts layer")
	}
	if !strings.Contains(out, `<g class="debug-vertical-cuts">`) {
		t.Error("output should contain debug-vertical-cuts layer")
	}

	// Check for page info in plain HTML (above SVG)
	// With heading exclusion, we have:
	// - Heading block excluded (H1 with 20pt line height)
	// - Multi-column zone with 2 bands (body rows 1 and 2)
	// - Footer is single-column mono band
	// So: 3 bands total
	if !strings.Contains(out, "Page: 1") {
		t.Error("output should contain page number in page-info")
	}
	if !strings.Contains(out, "Bands:") {
		t.Error("output should contain 'Bands:' in page-info")
	}
	if !strings.Contains(out, "Columns:") {
		t.Error("output should contain 'Columns:' in page-info")
	}
	if !strings.Contains(out, `class="page-info"`) {
		t.Error("output should contain page-info class")
	}

	// Check for zone rectangles with the layout zone class
	if !strings.Contains(out, `class="debug-layout-zone"`) {
		t.Error("output should contain debug-layout-zone class for zone rectangles")
	}

	// Check for horizontal cut lines
	if !strings.Contains(out, `class="debug-horizontal-cut"`) {
		t.Error("output should contain debug-horizontal-cut class")
	}

	// Check for vertical cut lines (should be present in two-column body)
	if !strings.Contains(out, `class="debug-vertical-cut"`) {
		t.Error("output should contain debug-vertical-cut class")
	}

	// Check for band outline rectangles
	if !strings.Contains(out, `class="debug-band-outline"`) {
		t.Error("output should contain debug-band-outline class")
	}

	// Check CSS styles for layout overlays
	if !strings.Contains(out, ".debug-layout-zone") {
		t.Error("CSS should contain .debug-layout-zone style")
	}
	if !strings.Contains(out, ".debug-layout-label") {
		t.Error("CSS should contain .debug-layout-label style")
	}
	if !strings.Contains(out, ".debug-band-outline") {
		t.Error("CSS should contain .debug-band-outline style")
	}
	if !strings.Contains(out, ".debug-horizontal-cut") {
		t.Error("CSS should contain .debug-horizontal-cut style")
	}
	if !strings.Contains(out, ".debug-vertical-cut") {
		t.Error("CSS should contain .debug-vertical-cut style")
	}
}

func TestHTMLPageInfoDisplay(t *testing.T) {
	doc := makeDoc("pageinfo.pdf", []model.Page{
		{
			Number: 3, Width: 500, Height: 700,
			Flows: []model.Flow{
				makeFlow(50, 50, 450, 400, []model.Block{
					// Two-column band
					makeBlock(50, 50, 230, 100, []model.Line{
						{XMin: 50, YMin: 58, XMax: 230, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Left col"},
						{XMin: 50, YMin: 78, XMax: 230, YMax: 90, FontSize: 12, Role: model.RoleBody, Text: "Left col 2"},
					}),
					makeBlock(270, 50, 450, 100, []model.Line{
						{XMin: 270, YMin: 58, XMax: 450, YMax: 70, FontSize: 12, Role: model.RoleBody, Text: "Right col"},
						{XMin: 270, YMin: 78, XMax: 450, YMax: 90, FontSize: 12, Role: model.RoleBody, Text: "Right col 2"},
					}),
					// Single-column band
					makeBlock(50, 150, 450, 200, []model.Line{
						{XMin: 50, YMin: 158, XMax: 450, YMax: 170, FontSize: 12, Role: model.RoleBody, Text: "Full width 1"},
						{XMin: 50, YMin: 178, XMax: 450, YMax: 190, FontSize: 12, Role: model.RoleBody, Text: "Full width 2"},
					}),
				}),
			},
		},
	})

	var buf bytes.Buffer
	if err := HTML(&buf, doc); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	// Should have page info in plain HTML
	if !strings.Contains(out, `class="page-info"`) {
		t.Error("output should contain page-info element")
	}
	if !strings.Contains(out, "Page: 3") {
		t.Error("page-info should contain page number")
	}
	if !strings.Contains(out, "Bands:") {
		t.Error("page-info should contain band count")
	}
	if !strings.Contains(out, "Columns:") {
		t.Error("page-info should contain column info")
	}

	// Page info should be outside and before the SVG
	pageInfoIdx := strings.Index(out, `class="page-info"`)
	svgIdx := strings.Index(out, "<svg")
	if pageInfoIdx > svgIdx {
		t.Error("page-info should appear before SVG element")
	}

	// Border should be darker grey
	if !strings.Contains(out, "#666") {
		t.Error("SVG border should use darker grey (#666)")
	}
	if strings.Contains(out, "#ccc") {
		t.Error("SVG border should NOT use light grey (#ccc)")
	}
}
