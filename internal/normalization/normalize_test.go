package normalization

import (
	"testing"

	"github.com/user/pdf2md/internal/model"
)

// makeWord is a helper to create a model.Word.
func makeWord(xMin, yMin, xMax, yMax float64, text string) model.Word {
	return model.Word{XMin: xMin, YMin: yMin, XMax: xMax, YMax: yMax, Text: text}
}

// makeTestLine creates a model.Line with words.
func makeTestLine(xMin, yMin, xMax, yMax float64, words []model.Word) model.Line {
	text := ""
	for i, w := range words {
		if i > 0 {
			text += " "
		}
		text += w.Text
	}
	return model.Line{
		XMin:     xMin,
		YMin:     yMin,
		XMax:     xMax,
		YMax:     yMax,
		FontSize: yMax - yMin,
		Role:     model.RoleBody,
		Text:     text,
		Words:    words,
	}
}

// makeTestPage creates a page from a set of blocks.
func makeTestPage(width, height float64, blocks []model.Block) *model.Page {
	var flowXMin, flowYMin, flowXMax, flowYMax float64
	var allLines []model.Line
	for i, b := range blocks {
		allLines = append(allLines, b.Lines...)
		if i == 0 {
			flowXMin, flowYMin, flowXMax, flowYMax = b.XMin, b.YMin, b.XMax, b.YMax
		} else {
			if b.XMin < flowXMin {
				flowXMin = b.XMin
			}
			if b.YMin < flowYMin {
				flowYMin = b.YMin
			}
			if b.XMax > flowXMax {
				flowXMax = b.XMax
			}
			if b.YMax > flowYMax {
				flowYMax = b.YMax
			}
		}
	}
	return &model.Page{
		Number: 1,
		Width:  width,
		Height: height,
		Flows: []model.Flow{{
			XMin:   flowXMin,
			YMin:   flowYMin,
			XMax:   flowXMax,
			YMax:   flowYMax,
			Blocks: blocks,
			Lines:  allLines,
		}},
	}
}

// TestPassA_ElbowDetection tests that Pass A detects large inter-word gaps.
func TestPassA_ElbowDetection(t *testing.T) {
	// Create a line with 5 words: 4 small gaps (~5pt) and 1 large gap (~50pt).
	words := []model.Word{
		makeWord(10, 100, 40, 112, "Word1"),
		makeWord(45, 100, 75, 112, "Word2"),
		makeWord(80, 100, 110, 112, "Word3"),
		// Large gap here (110 -> 160 = 50pt)
		makeWord(160, 100, 190, 112, "Word4"),
		makeWord(195, 100, 225, 112, "Word5"),
	}
	line := makeTestLine(10, 100, 225, 112, words)
	block := model.Block{XMin: 10, YMin: 100, XMax: 225, YMax: 112, Lines: []model.Line{line}}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	// Should have detected the large gap as a candidate.
	if len(dd.GapCandidates) == 0 {
		t.Fatal("expected at least one gap candidate from elbow detection")
	}

	// The large gap should be at around x=135 (center of 110-160).
	found := false
	for _, gc := range dd.GapCandidates {
		if gc.XLeft >= 109 && gc.XRight <= 161 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected gap candidate in the 110-160 range")
	}

	// Should produce logical blocks.
	if len(logicalBlocks) == 0 {
		t.Fatal("expected at least one logical block")
	}
}

// TestPassA_SkipFewWords tests that lines with < MinWordsForElbow skip elbow detection.
func TestPassA_SkipFewWords(t *testing.T) {
	// Line with only 2 words - should skip elbow detection.
	words := []model.Word{
		makeWord(10, 100, 50, 112, "Hello"),
		makeWord(200, 100, 240, 112, "World"),
	}
	line := makeTestLine(10, 100, 240, 112, words)
	block := model.Block{XMin: 10, YMin: 100, XMax: 240, YMax: 112, Lines: []model.Line{line}}
	page := makeTestPage(500, 700, []model.Block{block})

	_, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	// With only 2 words, Pass A should not detect any gap candidates.
	if len(dd.GapCandidates) != 0 {
		t.Errorf("expected 0 gap candidates for 2-word line, got %d", len(dd.GapCandidates))
	}
}

// TestPassB_VerticalClustering tests that vertically aligned gaps form GapColumns.
func TestPassB_VerticalClustering(t *testing.T) {
	// Two lines at different Y positions, both with a large gap at similar X.
	line1Words := []model.Word{
		makeWord(10, 50, 40, 62, "A1"),
		makeWord(45, 50, 75, 62, "A2"),
		makeWord(80, 50, 110, 62, "A3"),
		makeWord(160, 50, 190, 62, "A4"), // gap at ~110-160
		makeWord(195, 50, 225, 62, "A5"),
	}
	line2Words := []model.Word{
		makeWord(10, 70, 40, 82, "B1"),
		makeWord(45, 70, 75, 82, "B2"),
		makeWord(80, 70, 110, 82, "B3"),
		makeWord(162, 70, 192, 82, "B4"), // gap at ~110-162, similar X
		makeWord(197, 70, 227, 82, "B5"),
	}

	line1 := makeTestLine(10, 50, 225, 62, line1Words)
	line2 := makeTestLine(10, 70, 227, 82, line2Words)
	block := model.Block{
		XMin: 10, YMin: 50, XMax: 227, YMax: 82,
		Lines: []model.Line{line1, line2},
	}
	page := makeTestPage(500, 700, []model.Block{block})

	_, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	// Should have at least 2 gap candidates that form a GapColumn.
	if len(dd.GapCandidates) < 2 {
		t.Fatalf("expected at least 2 gap candidates, got %d", len(dd.GapCandidates))
	}

	// Should have at least one GapColumn with 2+ gaps (structural).
	structuralCols := 0
	for _, col := range dd.GapColumns {
		if len(col.Gaps) >= MinStructuralLines {
			structuralCols++
		}
	}
	if structuralCols == 0 {
		t.Error("expected at least one structural GapColumn with 2+ gaps")
	}
}

// TestPassC_AlignmentClassification tests line classification by alignment.
func TestPassC_AlignmentClassification(t *testing.T) {
	// Left-aligned line: text starts near block left edge, ends before right edge.
	leftWords := []model.Word{
		makeWord(10, 100, 40, 112, "Left"),
		makeWord(45, 100, 75, 112, "aligned"),
		makeWord(80, 100, 110, 112, "text"),
	}
	leftLine := makeTestLine(10, 100, 110, 112, leftWords)

	block := model.Block{
		XMin: 10, YMin: 100, XMax: 200, YMax: 112,
		Lines: []model.Line{leftLine},
	}
	page := makeTestPage(500, 700, []model.Block{block})

	_, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	if len(dd.VirtualLines) == 0 {
		t.Fatal("expected at least one VirtualLine")
	}

	vl := dd.VirtualLines[0]
	if vl.Classification != LineLeft {
		t.Errorf("expected LINE_LEFT, got %s", vl.Classification.String())
	}
}

// TestPassD_StructuralSplitting tests that structural lines are split at gap positions.
func TestPassD_StructuralSplitting(t *testing.T) {
	// Create multiple lines that all have a structural gap at similar X position.
	// This ensures the gap is confirmed as structural by Pass B (>= MinStructuralLines).
	var lines []model.Line
	for y := float64(50); y <= 120; y += 14 {
		words := []model.Word{
			makeWord(10, y, 40, y+12, "Left1"),
			makeWord(45, y, 75, y+12, "Left2"),
			makeWord(80, y, 110, y+12, "Left3"),
			makeWord(160, y, 190, y+12, "Right1"), // gap at ~110-160
			makeWord(195, y, 225, y+12, "Right2"),
		}
		lines = append(lines, makeTestLine(10, y, 225, y+12, words))
	}

	block := model.Block{
		XMin: 10, YMin: 50, XMax: 225, YMax: 134,
		Lines: lines,
	}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	// After splitting, we should have more VirtualLines than original lines.
	if len(dd.VirtualLines) <= len(lines) {
		t.Errorf("expected VirtualLines (%d) > original lines (%d) after structural split",
			len(dd.VirtualLines), len(lines))
	}

	// Check that some virtual lines are marked as virtual.
	virtualCount := 0
	for _, vl := range dd.VirtualLines {
		if vl.Virtual {
			virtualCount++
		}
	}
	if virtualCount == 0 {
		t.Error("expected some virtual lines after structural splitting")
	}

	// Should have logical blocks.
	if len(logicalBlocks) == 0 {
		t.Fatal("expected at least one logical block")
	}
}

// TestPassE_LogicalBlockReconstruction tests merging of contiguous lines.
func TestPassE_LogicalBlockReconstruction(t *testing.T) {
	// Create two contiguous left-aligned lines that should merge into one LogicalBlock.
	line1 := makeTestLine(10, 50, 200, 62, []model.Word{
		makeWord(10, 50, 200, 62, "First"),
	})
	line2 := makeTestLine(10, 63, 200, 75, []model.Word{
		makeWord(10, 63, 200, 75, "Second"),
	})

	block := model.Block{
		XMin: 10, YMin: 50, XMax: 200, YMax: 75,
		Lines: []model.Line{line1, line2},
	}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, _ := NormalizePage(page, Options{}, false)

	// Two contiguous lines with same classification and small Y gap -> 1 LogicalBlock.
	if len(logicalBlocks) != 1 {
		t.Fatalf("expected 1 logical block, got %d", len(logicalBlocks))
	}

	if len(logicalBlocks[0].Lines) != 2 {
		t.Errorf("expected 2 lines in logical block, got %d", len(logicalBlocks[0].Lines))
	}
}

// TestPassE_LargeGapSplitsBlocks tests that a large Y gap creates separate LogicalBlocks.
func TestPassE_LargeGapSplitsBlocks(t *testing.T) {
	line1 := makeTestLine(10, 50, 200, 62, []model.Word{
		makeWord(10, 50, 200, 62, "First"),
	})
	// Large gap (> MergeGapThreshold = 6.0pt)
	line2 := makeTestLine(10, 100, 200, 112, []model.Word{
		makeWord(10, 100, 200, 112, "Second"),
	})

	block := model.Block{
		XMin: 10, YMin: 50, XMax: 200, YMax: 112,
		Lines: []model.Line{line1, line2},
	}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, _ := NormalizePage(page, Options{}, false)

	// Large gap should separate into 2 LogicalBlocks.
	if len(logicalBlocks) != 2 {
		t.Fatalf("expected 2 logical blocks for large Y gap, got %d", len(logicalBlocks))
	}
}

// TestNormalizePage_EmptyPage tests that an empty page produces no logical blocks.
func TestNormalizePage_EmptyPage(t *testing.T) {
	page := &model.Page{
		Number: 1, Width: 500, Height: 700,
		Flows: []model.Flow{},
	}

	logicalBlocks, dd := NormalizePage(page, Options{}, true)

	if logicalBlocks != nil {
		t.Errorf("expected nil logical blocks for empty page, got %d", len(logicalBlocks))
	}
	if dd != nil {
		t.Error("expected nil debug data for empty page")
	}
}

// TestNormalizePage_SingleWordLine tests a line with a single word.
func TestNormalizePage_SingleWordLine(t *testing.T) {
	line := makeTestLine(10, 50, 50, 62, []model.Word{
		makeWord(10, 50, 50, 62, "Hello"),
	})
	block := model.Block{
		XMin: 10, YMin: 50, XMax: 50, YMax: 62,
		Lines: []model.Line{line},
	}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	if len(logicalBlocks) != 1 {
		t.Fatalf("expected 1 logical block, got %d", len(logicalBlocks))
	}

	// No gap candidates for single-word line.
	if len(dd.GapCandidates) != 0 {
		t.Errorf("expected 0 gap candidates, got %d", len(dd.GapCandidates))
	}
}

// TestApplyNormalization tests the full integration with model.Document.
func TestApplyNormalization(t *testing.T) {
	doc := &model.Document{
		Source:  "test.pdf",
		FontMap: map[string]model.FontSpec{"10.0": {Size: 10, Role: model.RoleBody}},
		Pages: []model.Page{
			{
				Number: 1, Width: 500, Height: 700,
				Flows: []model.Flow{{
					XMin: 10, YMin: 50, XMax: 200, YMax: 75,
					Blocks: []model.Block{{
						XMin: 10, YMin: 50, XMax: 200, YMax: 75,
						Lines: []model.Line{
							makeTestLine(10, 50, 200, 62, []model.Word{
								makeWord(10, 50, 100, 62, "Hello"),
								makeWord(105, 50, 200, 62, "World"),
							}),
							makeTestLine(10, 63, 200, 75, []model.Word{
								makeWord(10, 63, 100, 75, "Foo"),
								makeWord(105, 63, 200, 75, "Bar"),
							}),
						},
					}},
					Lines: []model.Line{
						makeTestLine(10, 50, 200, 62, []model.Word{
							makeWord(10, 50, 100, 62, "Hello"),
							makeWord(105, 50, 200, 62, "World"),
						}),
						makeTestLine(10, 63, 200, 75, []model.Word{
							makeWord(10, 63, 100, 75, "Foo"),
							makeWord(105, 63, 200, 75, "Bar"),
						}),
					},
				}},
			},
		},
	}

	ApplyNormalization(doc, Options{}, true)

	// Page should still have flows and blocks.
	if len(doc.Pages[0].Flows) == 0 {
		t.Fatal("expected flows after normalization")
	}
	if len(doc.Pages[0].Flows[0].Blocks) == 0 {
		t.Fatal("expected blocks after normalization")
	}

	// Debug data should be stored.
	if doc.Pages[0].NormDebugData == nil {
		t.Fatal("expected NormDebugData to be set with debug=true")
	}

	dd, ok := doc.Pages[0].NormDebugData.(*DebugData)
	if !ok {
		t.Fatal("NormDebugData should be *DebugData")
	}
	if len(dd.LogicalBlocks) == 0 {
		t.Fatal("expected logical blocks in debug data")
	}
}

// TestLineSplitRatioOverride tests that the LineSplitRatio option works.
func TestLineSplitRatioOverride(t *testing.T) {
	// With a very high split ratio, no gaps should be detected.
	words := []model.Word{
		makeWord(10, 100, 40, 112, "W1"),
		makeWord(45, 100, 75, 112, "W2"),
		makeWord(80, 100, 110, 112, "W3"),
		makeWord(160, 100, 190, 112, "W4"),
		makeWord(195, 100, 225, 112, "W5"),
	}
	line := makeTestLine(10, 100, 225, 112, words)
	block := model.Block{XMin: 10, YMin: 100, XMax: 225, YMax: 112, Lines: []model.Line{line}}
	page := makeTestPage(500, 700, []model.Block{block})

	// Very high ratio - should detect no structural gaps.
	_, ddHigh := NormalizePage(page, Options{LineSplitRatio: 1000.0}, true)
	if ddHigh == nil {
		t.Fatal("expected debug data")
	}
	if len(ddHigh.GapCandidates) != 0 {
		t.Errorf("expected 0 gap candidates with high split ratio, got %d", len(ddHigh.GapCandidates))
	}

	// Default ratio - should detect some gaps.
	_, ddDefault := NormalizePage(page, Options{}, true)
	if ddDefault == nil {
		t.Fatal("expected debug data")
	}
	// With default ratio (3.0), the 50pt gap vs 5pt gaps should be detected.
	if len(ddDefault.GapCandidates) == 0 {
		t.Error("expected gap candidates with default split ratio")
	}
}

// TestIsSameClassFamily tests the classification family matching.
func TestIsSameClassFamily(t *testing.T) {
	tests := []struct {
		a, b   LineType
		expect bool
	}{
		{LineLeft, LineRight, true},
		{LineLeft, LineJustified, true},
		{LineRight, LineJustified, true},
		{LineLeft, LineStructural, false},
		{LineStructural, LineStructural, true},
		{LineUnclassified, LineLeft, true},
		{LineUnclassified, LineStructural, false},
	}

	for _, tt := range tests {
		got := isSameClassFamily(tt.a, tt.b)
		if got != tt.expect {
			t.Errorf("isSameClassFamily(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.expect)
		}
	}
}

// TestSplitWordsAtPositions tests the word splitting logic.
func TestSplitWordsAtPositions(t *testing.T) {
	words := []model.Word{
		makeWord(10, 0, 40, 10, "A"),
		makeWord(50, 0, 80, 10, "B"),
		makeWord(150, 0, 180, 10, "C"),
		makeWord(190, 0, 220, 10, "D"),
	}

	// Split at x=100 (between B and C).
	clusters := splitWordsAtPositions(words, []float64{100})
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	if len(clusters[0]) != 2 {
		t.Errorf("expected 2 words in first cluster, got %d", len(clusters[0]))
	}
	if len(clusters[1]) != 2 {
		t.Errorf("expected 2 words in second cluster, got %d", len(clusters[1]))
	}
}

// TestBlockType_String tests BlockType string representation.
func TestBlockType_String(t *testing.T) {
	if BlockText.String() != "TEXT" {
		t.Errorf("expected 'TEXT', got %q", BlockText.String())
	}
	if BlockStructural.String() != "STRUCTURAL" {
		t.Errorf("expected 'STRUCTURAL', got %q", BlockStructural.String())
	}
}

// TestLineType_String tests LineType string representation.
func TestLineType_String(t *testing.T) {
	tests := []struct {
		lt   LineType
		want string
	}{
		{LineUnclassified, "UNCLASSIFIED"},
		{LineLeft, "LEFT"},
		{LineRight, "RIGHT"},
		{LineJustified, "JUSTIFIED"},
		{LineStructural, "STRUCTURAL"},
	}
	for _, tt := range tests {
		if got := tt.lt.String(); got != tt.want {
			t.Errorf("LineType(%d).String() = %q, want %q", tt.lt, got, tt.want)
		}
	}
}

// TestGapType_String tests GapType string representation.
func TestGapType_String(t *testing.T) {
	tests := []struct {
		gt   GapType
		want string
	}{
		{GapUnclassified, "UNCLASSIFIED"},
		{GapColumnType, "COLUMN"},
		{GapTableType, "TABLE"},
		{GapIsolated, "ISOLATED"},
	}
	for _, tt := range tests {
		if got := tt.gt.String(); got != tt.want {
			t.Errorf("GapType(%d).String() = %q, want %q", tt.gt, got, tt.want)
		}
	}
}

// TestTwoColumnDetection is an integration test simulating a two-column PDF page
// where Poppler groups all words into a single wide line per row.
func TestTwoColumnDetection(t *testing.T) {
	// Simulate: two columns of text where Poppler creates single wide lines.
	// Left column: x=[10, 200], Right column: x=[300, 490]
	// Gap between columns: 200-300 (100pt gap)
	var lines []model.Line

	for row := 0; row < 6; row++ {
		y := 50 + float64(row)*14
		// Each line spans the full width with a big gap in the middle.
		words := []model.Word{
			makeWord(10, y, 60, y+12, "Left"),
			makeWord(65, y, 120, y+12, "column"),
			makeWord(125, y, 190, y+12, "text"),
			// Large gap (200pt -> 300pt)
			makeWord(300, y, 360, y+12, "Right"),
			makeWord(365, y, 430, y+12, "column"),
			makeWord(435, y, 490, y+12, "text"),
		}
		lines = append(lines, makeTestLine(10, y, 490, y+12, words))
	}

	block := model.Block{
		XMin: 10, YMin: 50, XMax: 490, YMax: 50 + 6*14,
		Lines: lines,
	}
	page := makeTestPage(500, 700, []model.Block{block})

	logicalBlocks, dd := NormalizePage(page, Options{}, true)

	if dd == nil {
		t.Fatal("expected debug data")
	}

	// The large gap between columns should be detected.
	if len(dd.GapCandidates) == 0 {
		t.Fatal("expected gap candidates for two-column layout")
	}

	// Should have a structural GapColumn.
	structCols := 0
	for _, col := range dd.GapColumns {
		if len(col.Gaps) >= MinStructuralLines {
			structCols++
		}
	}
	if structCols == 0 {
		t.Error("expected at least one structural GapColumn")
	}

	// After splitting, should have more logical blocks than before
	// (lines split into left and right sub-lines).
	if len(logicalBlocks) < 2 {
		t.Errorf("expected at least 2 logical blocks for two-column layout, got %d", len(logicalBlocks))
	}

	// Check that some lines are virtual (split from structural lines).
	virtualCount := 0
	for _, vl := range dd.VirtualLines {
		if vl.Virtual {
			virtualCount++
		}
	}
	if virtualCount == 0 {
		t.Error("expected virtual lines from structural splitting")
	}
}
