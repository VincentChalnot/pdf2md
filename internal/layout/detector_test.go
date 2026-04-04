package layout

import (
	"testing"

	"github.com/user/pdf2md/internal/model"
)

// TestDetectLayoutEmpty tests layout detection with no blocks
func TestDetectLayoutEmpty(t *testing.T) {
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows:  []model.Flow{},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 0 {
		t.Errorf("Expected 0 zones for empty page, got %d", len(layout.Zones))
	}
	if len(layout.HorizontalCuts) != 0 {
		t.Errorf("Expected 0 horizontal cuts for empty page, got %d", len(layout.HorizontalCuts))
	}
}

// TestDetectLayoutSingleBlock tests layout detection with a single block
func TestDetectLayoutSingleBlock(t *testing.T) {
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 100, XMax: 450, YMax: 200,
				Blocks: []model.Block{
					{XMin: 50, YMin: 100, XMax: 450, YMax: 200},
				},
			},
		},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}

	zone := layout.Zones[0]
	if zone.BandCount != 1 {
		t.Errorf("Expected 1 band, got %d", zone.BandCount)
	}
	if zone.ColumnCount != 1 {
		t.Errorf("Expected 1 column, got %d", zone.ColumnCount)
	}
	if len(zone.VerticalCuts) != 0 {
		t.Errorf("Expected 0 vertical cuts, got %d", len(zone.VerticalCuts))
	}
}

// TestDetectLayoutHorizontalCuts tests horizontal band detection
func TestDetectLayoutHorizontalCuts(t *testing.T) {
	// Create page with two single-column blocks separated by a gap > MinGapThreshold
	// According to Fix 1, horizontal cuts between mono-column bands should be suppressed
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 250,
				Blocks: []model.Block{
					{XMin: 50, YMin: 50, XMax: 450, YMax: 100},   // First band (single-column)
					{XMin: 50, YMin: 150, XMax: 450, YMax: 200},  // Second band (single-column, gap of 50pt)
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Should NOT show horizontal cut between mono-column bands (Fix 1)
	if len(layout.HorizontalCuts) != 0 {
		t.Errorf("Expected 0 horizontal cuts between mono-column bands, got %d", len(layout.HorizontalCuts))
	}

	// Both bands should be grouped into a single zone
	if len(layout.Zones) != 1 {
		t.Errorf("Expected 1 zone for consecutive mono-column bands, got %d", len(layout.Zones))
	}
}

// TestDetectLayoutVerticalCuts tests vertical column detection
func TestDetectLayoutVerticalCuts(t *testing.T) {
	// Create page with two blocks side-by-side (two columns)
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 100, XMax: 450, YMax: 200,
				Blocks: []model.Block{
					{XMin: 50, YMin: 100, XMax: 230, YMax: 200},   // Left column
					{XMin: 270, YMin: 100, XMax: 450, YMax: 200},  // Right column (gap of 40pt)
				},
			},
		},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}

	zone := layout.Zones[0]
	if zone.ColumnCount != 2 {
		t.Errorf("Expected 2 columns, got %d", zone.ColumnCount)
	}
	if len(zone.VerticalCuts) != 1 {
		t.Fatalf("Expected 1 vertical cut, got %d", len(zone.VerticalCuts))
	}

	cut := zone.VerticalCuts[0]
	if cut < 230 || cut > 270 {
		t.Errorf("Expected cut between 230 and 270, got %f", cut)
	}
}

// TestDetectLayoutMultipleZones tests zone grouping
func TestDetectLayoutMultipleZones(t *testing.T) {
	// Create page with:
	// - Single-column header
	// - Two-column body
	// - Single-column footer
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 400,
				Blocks: []model.Block{
					// Header (single column)
					{XMin: 50, YMin: 50, XMax: 450, YMax: 100},

					// Body row 1 (two columns)
					{XMin: 50, YMin: 150, XMax: 230, YMax: 200},
					{XMin: 270, YMin: 150, XMax: 450, YMax: 200},

					// Body row 2 (two columns, same structure)
					{XMin: 50, YMin: 250, XMax: 230, YMax: 300},
					{XMin: 270, YMin: 250, XMax: 450, YMax: 300},

					// Footer (single column)
					{XMin: 50, YMin: 350, XMax: 450, YMax: 400},
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Should have 3 zones: header, body (2 bands), footer
	if len(layout.Zones) != 3 {
		t.Fatalf("Expected 3 zones, got %d", len(layout.Zones))
	}

	// Check that one zone has 2 bands (the two-column body)
	foundTwoBandZone := false
	for _, zone := range layout.Zones {
		if zone.BandCount == 2 {
			foundTwoBandZone = true
			if zone.ColumnCount != 2 {
				t.Errorf("Expected 2-band zone to have 2 columns, got %d", zone.ColumnCount)
			}
		}
	}
	if !foundTwoBandZone {
		t.Errorf("Expected to find a zone with 2 bands")
	}
}

// TestDetectLayoutBandHeightVariance tests variance calculations
func TestDetectLayoutBandHeightVariance(t *testing.T) {
	// Create zone with uniform band heights (low variance)
	// Using two-column layout so bands will group into a single zone
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 350,
				Blocks: []model.Block{
					// Row 1: two columns, height 50
					{XMin: 50, YMin: 50, XMax: 240, YMax: 100},
					{XMin: 260, YMin: 50, XMax: 450, YMax: 100},
					// Row 2: two columns, height 50
					{XMin: 50, YMin: 150, XMax: 240, YMax: 200},
					{XMin: 260, YMin: 150, XMax: 450, YMax: 200},
					// Row 3: two columns, height 50
					{XMin: 50, YMin: 250, XMax: 240, YMax: 300},
					{XMin: 260, YMin: 250, XMax: 450, YMax: 300},
				},
			},
		},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}

	zone := layout.Zones[0]
	// With uniform heights, variance should be 0
	if zone.BandHeightVariance != 0 {
		t.Errorf("Expected 0 variance for uniform heights, got %f", zone.BandHeightVariance)
	}
}

// TestDetectLayoutColumnWidthVariance tests column width variance
func TestDetectLayoutColumnWidthVariance(t *testing.T) {
	// Create zone with two equal-width columns (low variance)
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 100, XMax: 450, YMax: 200,
				Blocks: []model.Block{
					{XMin: 50, YMin: 100, XMax: 240, YMax: 200},   // Width: 190
					{XMin: 260, YMin: 100, XMax: 450, YMax: 200},  // Width: 190 (gap: 20)
				},
			},
		},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}

	zone := layout.Zones[0]
	if zone.ColumnCount != 2 {
		t.Fatalf("Expected 2 columns, got %d", zone.ColumnCount)
	}

	// With equal widths, variance should be very small (close to 0)
	// Allowing small tolerance for floating point arithmetic
	if zone.ColumnWidthVariance > 1.0 {
		t.Errorf("Expected low variance for equal-width columns, got %f", zone.ColumnWidthVariance)
	}
}

// TestMinGapThreshold tests that small gaps are ignored
func TestMinGapThreshold(t *testing.T) {
	// Create blocks with gap < MinGapThreshold
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 100, XMax: 450, YMax: 160,
				Blocks: []model.Block{
					{XMin: 50, YMin: 100, XMax: 450, YMax: 125},  // Height: 25
					{XMin: 50, YMin: 130, XMax: 450, YMax: 155},  // Gap: 5pt (< MinGapThreshold)
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Should NOT create a horizontal cut (gap too small)
	if len(layout.HorizontalCuts) != 0 {
		t.Errorf("Expected 0 cuts for gap < MinGapThreshold, got %d", len(layout.HorizontalCuts))
	}

	// Should merge into single band
	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}
	if layout.Zones[0].BandCount != 1 {
		t.Errorf("Expected 1 band for merged blocks, got %d", layout.Zones[0].BandCount)
	}
}

// TestColumnGroupingTolerance tests that similar cuts are matched
func TestColumnGroupingTolerance(t *testing.T) {
	// Create two bands with slightly different cut positions (within tolerance)
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 250,
				Blocks: []model.Block{
					// Band 1 - cut at ~250
					{XMin: 50, YMin: 50, XMax: 240, YMax: 100},
					{XMin: 260, YMin: 50, XMax: 450, YMax: 100},

					// Band 2 - cut at ~255 (within tolerance of 250)
					{XMin: 50, YMin: 150, XMax: 245, YMax: 200},
					{XMin: 265, YMin: 150, XMax: 450, YMax: 200},
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Should group into single zone despite slight difference
	if len(layout.Zones) != 1 {
		t.Errorf("Expected bands with similar cuts to group into 1 zone, got %d", len(layout.Zones))
	}

	if len(layout.Zones) > 0 && layout.Zones[0].BandCount != 2 {
		t.Errorf("Expected zone to contain 2 bands, got %d", layout.Zones[0].BandCount)
	}
}

// TestDetectLayoutZoneBoundingBox tests zone bounding box calculation
func TestDetectLayoutZoneBoundingBox(t *testing.T) {
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 100, XMax: 450, YMax: 300,
				Blocks: []model.Block{
					{XMin: 50, YMin: 100, XMax: 240, YMax: 150},
					{XMin: 260, YMin: 100, XMax: 450, YMax: 150},
					{XMin: 50, YMin: 200, XMax: 240, YMax: 250},
					{XMin: 260, YMin: 200, XMax: 450, YMax: 250},
				},
			},
		},
	}

	layout := DetectLayout(page)

	if len(layout.Zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(layout.Zones))
	}

	zone := layout.Zones[0]

	// Zone should span all blocks
	if zone.XMin != 50 || zone.XMax != 450 {
		t.Errorf("Expected zone X range [50, 450], got [%f, %f]", zone.XMin, zone.XMax)
	}
	if zone.YMin != 100 || zone.YMax != 250 {
		t.Errorf("Expected zone Y range [100, 250], got [%f, %f]", zone.YMin, zone.YMax)
	}
}

// TestFix1_SuppressHorizontalCutsBetweenMonoColumns tests Fix 1
// Horizontal cuts should only appear when at least one adjacent band is multi-column
func TestFix1_SuppressHorizontalCutsBetweenMonoColumns(t *testing.T) {
	// Create page with mono-column band -> multi-column band -> mono-column band
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 400,
				Blocks: []model.Block{
					// Mono-column band 1
					{XMin: 50, YMin: 50, XMax: 450, YMax: 100},

					// Multi-column band (gap of 50pt from previous)
					{XMin: 50, YMin: 200, XMax: 230, YMax: 250},
					{XMin: 270, YMin: 200, XMax: 450, YMax: 250},

					// Mono-column band 2 (gap of 50pt from previous)
					{XMin: 50, YMin: 350, XMax: 450, YMax: 400},
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Should have 2 horizontal cuts:
	// - One between mono-column and multi-column (at ~150)
	// - One between multi-column and mono-column (at ~300)
	if len(layout.HorizontalCuts) != 2 {
		t.Errorf("Expected 2 horizontal cuts (only between mono/multi bands), got %d", len(layout.HorizontalCuts))
	}
}

// TestFix2_BandGroupingWithSharedCuts tests Fix 2
// Bands should group if they share at least one vertical cut position
func TestFix2_BandGroupingWithSharedCuts(t *testing.T) {
	// Create bands with different numbers of cuts, but sharing one position
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 300,
				Blocks: []model.Block{
					// Band 1: 2 columns (1 cut at ~250)
					{XMin: 50, YMin: 50, XMax: 240, YMax: 100},
					{XMin: 260, YMin: 50, XMax: 450, YMax: 100},

					// Band 2: 3 columns (2 cuts, one at ~250, one at ~350)
					{XMin: 50, YMin: 150, XMax: 240, YMax: 200},
					{XMin: 260, YMin: 150, XMax: 340, YMax: 200},
					{XMin: 360, YMin: 150, XMax: 450, YMax: 200},
				},
			},
		},
	}

	layout := DetectLayout(page)

	// These two bands should be grouped into one zone because they share
	// the cut position at ~250 (within tolerance)
	if len(layout.Zones) != 1 {
		t.Errorf("Expected bands with shared cut positions to group into 1 zone, got %d", len(layout.Zones))
	}

	if len(layout.Zones) > 0 && layout.Zones[0].BandCount != 2 {
		t.Errorf("Expected zone to contain 2 bands, got %d", layout.Zones[0].BandCount)
	}
}

// TestFix3_HeadingBlockExclusion tests Fix 3
// Blocks with large line height should be marked as headings and excluded from layout
func TestFix3_HeadingBlockExclusion(t *testing.T) {
	// Create page with a heading block (large line height) and normal blocks
	page := &model.Page{
		Number: 1,
		Width:  500,
		Height: 700,
		Flows: []model.Flow{
			{
				XMin: 50, YMin: 50, XMax: 450, YMax: 200,
				Blocks: []model.Block{
					// Heading block with large line height (20pt > 14pt threshold)
					{
						XMin: 50, YMin: 50, XMax: 450, YMax: 100,
						Lines: []model.Line{
							{XMin: 50, YMin: 50, YMax: 70, Text: "Big Heading"},
						},
					},

					// Normal blocks side-by-side in the same horizontal band (multi-column)
					// Use exact same Y coordinates to ensure they're in the same band
					{
						XMin: 50, YMin: 150, XMax: 230, YMax: 200,
						Lines: []model.Line{
							{XMin: 50, YMin: 190, YMax: 200, Text: "Normal text"},
						},
					},
					{
						XMin: 270, YMin: 150, XMax: 450, YMax: 200,
						Lines: []model.Line{
							{XMin: 270, YMin: 190, YMax: 200, Text: "Normal text"},
						},
					},
				},
			},
		},
	}

	layout := DetectLayout(page)

	// Check that the heading block was marked
	if !page.Flows[0].Blocks[0].IsHeading {
		t.Errorf("Expected first block to be marked as heading")
	}

	if page.Flows[0].Blocks[1].IsHeading {
		t.Errorf("Expected second block NOT to be marked as heading")
	}

	if page.Flows[0].Blocks[2].IsHeading {
		t.Errorf("Expected third block NOT to be marked as heading")
	}

	// Layout should only include the multi-column band (heading excluded)
	// The two normal blocks should be in the same band since they have the same Y range
	if len(layout.Zones) != 1 {
		t.Errorf("Expected 1 zone (heading excluded), got %d", len(layout.Zones))
		for i, zone := range layout.Zones {
			t.Logf("Zone %d: bands=%d, columns=%d, verticalCuts=%v", i, zone.BandCount, zone.ColumnCount, zone.VerticalCuts)
		}
	}

	if len(layout.Zones) > 0 {
		zone := layout.Zones[0]
		if zone.BandCount != 1 {
			t.Errorf("Expected 1 band in zone (heading excluded), got %d", zone.BandCount)
		}
		if zone.ColumnCount != 2 {
			t.Errorf("Expected 2 columns, got %d", zone.ColumnCount)
		}
	}
}

