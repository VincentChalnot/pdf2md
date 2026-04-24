package pre_process

import (
	"fmt"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

// pageHeight is the page height used in all test fixtures (standard A4 in PDF units).
const pageHeight = 842.0

// makeFlow builds a Flow with one line whose text is text, positioned at the given Y
// coordinates on a page that is pageWidth × pageHeight.
func makeFlow(xMin, yMin, xMax, yMax float64, text string) model.Flow {
	line := model.Line{
		XMin: xMin, YMin: yMin,
		XMax: xMax, YMax: yMax,
		FontSize: 10,
		Text:     text,
	}
	return model.Flow{
		XMin: xMin, YMin: yMin,
		XMax: xMax, YMax: yMax,
		Lines: []model.Line{line},
	}
}

// headerFlow returns a flow that sits in the header zone of the page.
// Y centre ≈ 50/842 ≈ 0.059 which is < hfZoneFraction (0.15).
func headerFlow(text string) model.Flow { return makeFlow(50, 40, 500, 60, text) }

// footerFlow returns a flow that sits in the footer zone of the page.
// Y centre ≈ 815/842 ≈ 0.968 which is > 1 - hfZoneFraction (0.85).
func footerFlow(text string) model.Flow { return makeFlow(50, 805, 500, 825, text) }

// pageNumFlow returns a short numeric flow at the very bottom of the page.
// Y centre ≈ 835/842 ≈ 0.991.
func pageNumFlow(n int) model.Flow {
	return makeFlow(200, 825, 250, 845, fmt.Sprintf("%d", n))
}

// bodyFlow returns a flow that is squarely in the body area of the page.
func bodyFlow(text string) model.Flow { return makeFlow(50, 300, 500, 320, text) }

// makeDoc builds a Document with n pages each containing the given flows.
// The page numbers are 1-based.
func makeDoc(pages [][]model.Flow) *model.Document {
	doc := &model.Document{}
	for i, flows := range pages {
		doc.Pages = append(doc.Pages, model.Page{
			Number: i + 1,
			Width:  595,
			Height: pageHeight,
			Flows:  flows,
		})
	}
	return doc
}

// flowTexts returns the texts of every first-line in each flow of a page.
func flowTexts(page model.Page) []string {
	var out []string
	for _, f := range page.Flows {
		if len(f.Lines) > 0 {
			out = append(out, f.Lines[0].Text)
		}
	}
	return out
}

// ── Tests ────────────────────────────────────────────────────────────────────

// TestHeaderKeptOnFirstPage verifies that a header repeating on all pages is
// retained on the first page and removed from all subsequent pages.
func TestHeaderKeptOnFirstPage(t *testing.T) {
	const headerText = "My Book Title"
	pages := make([][]model.Flow, 5)
	for i := range pages {
		pages[i] = []model.Flow{
			headerFlow(headerText),
			bodyFlow(fmt.Sprintf("Body content page %d", i+1)),
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Page 0 must still have the header.
	texts0 := flowTexts(doc.Pages[0])
	if !contains(texts0, headerText) {
		t.Errorf("page 1: expected header %q to be kept, got %v", headerText, texts0)
	}

	// Pages 1-4 must NOT have the header.
	for i := 1; i < 5; i++ {
		texts := flowTexts(doc.Pages[i])
		if contains(texts, headerText) {
			t.Errorf("page %d: header %q should have been removed, got %v", i+1, headerText, texts)
		}
	}
}

// TestFooterKeptOnLastPage verifies that a footer repeating on all pages is
// retained only on the last page and removed from all earlier pages.
func TestFooterKeptOnLastPage(t *testing.T) {
	const footerText = "Copyright 2024 Acme Corp"
	pages := make([][]model.Flow, 5)
	for i := range pages {
		pages[i] = []model.Flow{
			bodyFlow(fmt.Sprintf("Body content page %d", i+1)),
			footerFlow(footerText),
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Pages 0-3 must NOT have the footer.
	for i := 0; i < 4; i++ {
		texts := flowTexts(doc.Pages[i])
		if contains(texts, footerText) {
			t.Errorf("page %d: footer %q should have been removed, got %v", i+1, footerText, texts)
		}
	}

	// Last page must still have the footer.
	texts4 := flowTexts(doc.Pages[4])
	if !contains(texts4, footerText) {
		t.Errorf("page 5: expected footer %q to be kept, got %v", footerText, texts4)
	}
}

// TestPageNumbersRemovedEntirely verifies that page numbers are removed from all pages.
func TestPageNumbersRemovedEntirely(t *testing.T) {
	pages := make([][]model.Flow, 5)
	for i := range pages {
		pages[i] = []model.Flow{
			bodyFlow(fmt.Sprintf("Body content page %d", i+1)),
			pageNumFlow(i + 1),
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for i, page := range doc.Pages {
		for _, f := range page.Flows {
			if hfIsPageNum(hfNormalizeText(hfFlowText(f))) {
				t.Errorf("page %d: page-number flow %q should have been removed", i+1, hfFlowText(f))
			}
		}
	}
}

// TestBodyContentNotRemoved verifies that flows outside the header/footer zones are
// never touched by the handler.
func TestBodyContentNotRemoved(t *testing.T) {
	const bodyText = "This is important body content."
	pages := make([][]model.Flow, 4)
	for i := range pages {
		pages[i] = []model.Flow{
			headerFlow("Same Header"),
			bodyFlow(bodyText),
			footerFlow("Same Footer"),
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for i, page := range doc.Pages {
		texts := flowTexts(page)
		if !contains(texts, bodyText) {
			t.Errorf("page %d: body flow %q was unexpectedly removed, got %v", i+1, bodyText, texts)
		}
	}
}

// TestFuzzyHeaderMatching verifies that headers with slight OCR-style variations are
// still detected and filtered correctly.
func TestFuzzyHeaderMatching(t *testing.T) {
	// Slight variations that should all normalise to the same header cluster.
	headerVariants := []string{
		"My Book Title",
		"My B0ok Title",  // OCR: 'o' → '0'
		"My Book Titlc",  // OCR: 'e' → 'c'
		"My Book Title",
		"My Book Titie",  // OCR: 'l' → 'i'
	}
	pages := make([][]model.Flow, len(headerVariants))
	for i, hdr := range headerVariants {
		pages[i] = []model.Flow{
			headerFlow(hdr),
			bodyFlow(fmt.Sprintf("Body page %d", i+1)),
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Exactly one page should contain a header-zone flow.
	headerPages := 0
	for _, page := range doc.Pages {
		for _, f := range page.Flows {
			relY := (f.YMin + f.YMax) / 2 / pageHeight
			if relY < hfZoneFraction {
				headerPages++
			}
		}
	}
	if headerPages != 1 {
		t.Errorf("expected exactly 1 page to retain the header, got %d", headerPages)
	}
}

// TestSinglePageNoFilter verifies that a single-page document is left untouched.
func TestSinglePageNoFilter(t *testing.T) {
	doc := makeDoc([][]model.Flow{
		{
			headerFlow("Only Header"),
			bodyFlow("Only Body"),
			footerFlow("Only Footer"),
		},
	})
	original := len(doc.Pages[0].Flows)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if got := len(doc.Pages[0].Flows); got != original {
		t.Errorf("single-page document: expected %d flows, got %d", original, got)
	}
}

// TestAlternatingPageNumbers verifies that page numbers at alternating horizontal
// positions (odd pages on the right, even pages on the left) but the same vertical
// position are all removed.
func TestAlternatingPageNumbers(t *testing.T) {
	pages := make([][]model.Flow, 6)
	for i := range pages {
		num := i + 1
		// Alternate left/right.
		var xMin, xMax float64
		if num%2 == 0 {
			xMin, xMax = 50, 80 // left
		} else {
			xMin, xMax = 510, 540 // right
		}
		pgFlow := model.Flow{
			XMin: xMin, YMin: 825, XMax: xMax, YMax: 845,
			Lines: []model.Line{{XMin: xMin, YMin: 825, XMax: xMax, YMax: 845, Text: fmt.Sprintf("%d", num)}},
		}
		pages[i] = []model.Flow{
			bodyFlow(fmt.Sprintf("Body page %d", num)),
			pgFlow,
		}
	}
	doc := makeDoc(pages)

	h := NewHeaderFooterHandler()
	if err := h.Run(doc); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for i, page := range doc.Pages {
		for _, f := range page.Flows {
			relY := (f.YMin + f.YMax) / 2 / pageHeight
			if relY > 1.0-hfZoneFraction && hfIsPageNum(hfNormalizeText(hfFlowText(f))) {
				t.Errorf("page %d: page number flow should have been removed, got %q", i+1, hfFlowText(f))
			}
		}
	}
}

// TestPageNumTemplates verifies the page-number candidate heuristic for common formats.
func TestPageNumTemplates(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"42", true},
		{"- 42 -", true},
		{"Page 42", true},
		{"42 of 150", true},
		{"p. 42", true},
		{"pg 42", true},
		{"no. 42", true},
		{"[42]", true},
		{"Section 42", false},          // "Section" is not a recognised page-word
		{"Introduction", false},        // no digit
		{"Chapter 1 Introduction", false}, // long remainder after stripping
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			got := hfIsPageNum(hfNormalizeText(tc.text))
			if got != tc.want {
				t.Errorf("hfIsPageNum(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// TestLevenshteinSimilarity spot-checks the similarity function.
func TestLevenshteinSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
	}{
		{"hello", "hello", 1.0},
		{"hello", "helo", 0.8},
		{"hello", "world", 0.0},
		{"My Book Title", "My B0ok Title", 0.9}, // one char diff
	}
	for _, tc := range cases {
		got := hfSimilarity(tc.a, tc.b)
		if got < tc.min {
			t.Errorf("hfSimilarity(%q, %q) = %.3f, want >= %.3f", tc.a, tc.b, got, tc.min)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
