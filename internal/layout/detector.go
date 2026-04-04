package layout

import (
	"math"
	"sort"

	"github.com/user/pdf2md/internal/model"
)

// Detection thresholds (configurable constants)
const (
	// MinGapThreshold is the minimum vertical or horizontal gap (in PDF points)
	// to consider as a valid cut between bands or columns
	MinGapThreshold = 6.0

	// ColumnGroupingTolerance is the tolerance (in PDF points) for matching
	// vertical cut positions when grouping bands into layout zones
	ColumnGroupingTolerance = 8.0
)

// interval represents an occupied range on an axis
type interval struct {
	min, max float64
}

// LayoutZone represents a contiguous region of the page with consistent column structure
type LayoutZone struct {
	// Index of this zone (0-based)
	Index int
	// Bands that belong to this zone
	Bands []*HorizontalBand
	// Vertical cut positions (X coordinates) that define columns
	VerticalCuts []float64
	// Computed metrics
	BandCount           int
	ColumnCount         int
	BandHeightVariance  float64
	ColumnWidthVariance float64
	// Bounding box of the entire zone
	XMin, YMin, XMax, YMax float64
}

// HorizontalBand represents a horizontal slice of the page between two horizontal cuts
type HorizontalBand struct {
	// Bounding box of the band
	XMin, YMin, XMax, YMax float64
	// Blocks contained in this band
	Blocks []model.Block
	// Vertical cut positions within this band
	VerticalCuts []float64
}

// HorizontalCut represents a horizontal empty gap between bands
type HorizontalCut struct {
	Y          float64 // Y coordinate of the cut
	XMin, XMax float64 // Horizontal extent of the cut
}

// PageLayout contains all layout information for a single page
type PageLayout struct {
	Zones          []*LayoutZone
	HorizontalCuts []HorizontalCut
}

// DetectLayout performs X-Y cut analysis on a page and returns layout zones
func DetectLayout(page *model.Page) *PageLayout {
	if page == nil {
		return &PageLayout{}
	}

	// Collect all blocks from all flows
	var allBlocks []model.Block
	for _, flow := range page.Flows {
		allBlocks = append(allBlocks, flow.Blocks...)
	}

	if len(allBlocks) == 0 {
		return &PageLayout{}
	}

	// Step 1: Find horizontal cuts (page-level Y-axis projection)
	bands, hCuts := findHorizontalBands(allBlocks, page.Width)

	// Step 2: Find vertical cuts for each band (X-axis projection)
	for _, band := range bands {
		band.VerticalCuts = findVerticalCuts(band.Blocks)
	}

	// Step 3: Group bands by column structure into layout zones
	zones := groupBandsIntoZones(bands)

	// Step 4: Characterize each zone (compute metrics)
	for i, zone := range zones {
		zone.Index = i
		characterizeZone(zone)
	}

	return &PageLayout{
		Zones:          zones,
		HorizontalCuts: hCuts,
	}
}

// findHorizontalBands performs Y-axis projection and identifies horizontal bands
func findHorizontalBands(blocks []model.Block, pageWidth float64) ([]*HorizontalBand, []HorizontalCut) {
	if len(blocks) == 0 {
		return nil, nil
	}

	// Project blocks onto Y axis to find occupied intervals
	intervals := make([]interval, len(blocks))
	for i, block := range blocks {
		intervals[i] = interval{min: block.YMin, max: block.YMax}
	}

	// Sort intervals by min Y
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].min < intervals[j].min
	})

	// Merge overlapping intervals to find occupied regions
	occupied := mergeIntervals(intervals)

	// Find gaps between occupied regions
	var cuts []HorizontalCut
	for i := 0; i < len(occupied)-1; i++ {
		gapStart := occupied[i].max
		gapEnd := occupied[i+1].min
		gapSize := gapEnd - gapStart

		if gapSize >= MinGapThreshold {
			cuts = append(cuts, HorizontalCut{
				Y:    (gapStart + gapEnd) / 2,
				XMin: 0,
				XMax: pageWidth,
			})
		}
	}

	// Create bands from occupied regions
	var bands []*HorizontalBand
	for _, occ := range occupied {
		band := &HorizontalBand{
			YMin: occ.min,
			YMax: occ.max,
		}

		// Collect blocks that fall within this band
		for _, block := range blocks {
			// Block belongs to band if it overlaps with the band's Y range
			if block.YMax > band.YMin && block.YMin < band.YMax {
				band.Blocks = append(band.Blocks, block)
				// Update band bounding box
				if len(band.Blocks) == 1 {
					band.XMin = block.XMin
					band.XMax = block.XMax
				} else {
					band.XMin = math.Min(band.XMin, block.XMin)
					band.XMax = math.Max(band.XMax, block.XMax)
				}
			}
		}

		if len(band.Blocks) > 0 {
			bands = append(bands, band)
		}
	}

	return bands, cuts
}

// findVerticalCuts performs X-axis projection on blocks within a band
func findVerticalCuts(blocks []model.Block) []float64 {
	if len(blocks) <= 1 {
		return nil
	}

	// Project blocks onto X axis
	intervals := make([]interval, len(blocks))
	for i, block := range blocks {
		intervals[i] = interval{min: block.XMin, max: block.XMax}
	}

	// Sort intervals by min X
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].min < intervals[j].min
	})

	// Merge overlapping intervals
	occupied := mergeIntervals(intervals)

	// Find gaps between occupied regions
	var cuts []float64
	for i := 0; i < len(occupied)-1; i++ {
		gapStart := occupied[i].max
		gapEnd := occupied[i+1].min
		gapSize := gapEnd - gapStart

		if gapSize >= MinGapThreshold {
			cuts = append(cuts, (gapStart + gapEnd) / 2)
		}
	}

	return cuts
}

// mergeIntervals merges overlapping intervals (assumes sorted by min)
// Only merges if they truly overlap or are within MinGapThreshold
func mergeIntervals(intervals []interval) []interval {
	if len(intervals) == 0 {
		return nil
	}

	merged := []interval{intervals[0]}
	for i := 1; i < len(intervals); i++ {
		last := &merged[len(merged)-1]
		current := intervals[i]

		// Calculate gap between last and current
		gap := current.min - last.max

		if gap < MinGapThreshold {
			// Overlapping or gap is too small - merge
			last.max = math.Max(last.max, current.max)
		} else {
			// Gap is large enough - keep separate
			merged = append(merged, current)
		}
	}

	return merged
}

// groupBandsIntoZones groups bands with similar column structure into zones
func groupBandsIntoZones(bands []*HorizontalBand) []*LayoutZone {
	if len(bands) == 0 {
		return nil
	}

	var zones []*LayoutZone

	for _, band := range bands {
		// Single-column bands (no vertical cuts) each form their own zone
		// Multi-column bands can be grouped if their cuts match
		hasCuts := len(band.VerticalCuts) > 0

		if !hasCuts {
			// Single-column band - create its own zone
			zone := &LayoutZone{
				Bands:        []*HorizontalBand{band},
				VerticalCuts: band.VerticalCuts,
			}
			zones = append(zones, zone)
			continue
		}

		// Multi-column band - try to find an existing zone with matching cuts
		matched := false
		for _, zone := range zones {
			if cutsMatch(zone.VerticalCuts, band.VerticalCuts) {
				// Add band to existing zone
				zone.Bands = append(zone.Bands, band)
				matched = true
				break
			}
		}

		if !matched {
			// Create new zone
			zone := &LayoutZone{
				Bands:        []*HorizontalBand{band},
				VerticalCuts: band.VerticalCuts,
			}
			zones = append(zones, zone)
		}
	}

	return zones
}

// cutsMatch checks if two sets of vertical cuts match within tolerance
func cutsMatch(cuts1, cuts2 []float64) bool {
	if len(cuts1) != len(cuts2) {
		return false
	}

	// Both have no cuts - they match
	if len(cuts1) == 0 {
		return true
	}

	// Check each cut position within tolerance
	for i := range cuts1 {
		if math.Abs(cuts1[i]-cuts2[i]) > ColumnGroupingTolerance {
			return false
		}
	}

	return true
}

// characterizeZone computes metrics for a layout zone
func characterizeZone(zone *LayoutZone) {
	if len(zone.Bands) == 0 {
		return
	}

	// Compute bounding box
	zone.XMin = zone.Bands[0].XMin
	zone.XMax = zone.Bands[0].XMax
	zone.YMin = zone.Bands[0].YMin
	zone.YMax = zone.Bands[0].YMax

	for _, band := range zone.Bands {
		zone.XMin = math.Min(zone.XMin, band.XMin)
		zone.XMax = math.Max(zone.XMax, band.XMax)
		zone.YMin = math.Min(zone.YMin, band.YMin)
		zone.YMax = math.Max(zone.YMax, band.YMax)
	}

	// Band count
	zone.BandCount = len(zone.Bands)

	// Column count (vertical cuts + 1)
	zone.ColumnCount = len(zone.VerticalCuts) + 1

	// Band height variance
	if len(zone.Bands) > 0 {
		heights := make([]float64, len(zone.Bands))
		var sumHeight float64
		for i, band := range zone.Bands {
			heights[i] = band.YMax - band.YMin
			sumHeight += heights[i]
		}
		meanHeight := sumHeight / float64(len(heights))

		var variance float64
		for _, h := range heights {
			diff := h - meanHeight
			variance += diff * diff
		}
		zone.BandHeightVariance = variance / float64(len(heights))
	}

	// Column width variance
	if len(zone.VerticalCuts) > 0 {
		// Compute column widths from cuts
		var widths []float64
		widths = append(widths, zone.VerticalCuts[0]-zone.XMin) // First column
		for i := 0; i < len(zone.VerticalCuts)-1; i++ {
			widths = append(widths, zone.VerticalCuts[i+1]-zone.VerticalCuts[i])
		}
		widths = append(widths, zone.XMax-zone.VerticalCuts[len(zone.VerticalCuts)-1]) // Last column

		var sumWidth float64
		for _, w := range widths {
			sumWidth += w
		}
		meanWidth := sumWidth / float64(len(widths))

		var variance float64
		for _, w := range widths {
			diff := w - meanWidth
			variance += diff * diff
		}
		zone.ColumnWidthVariance = variance / float64(len(widths))
	}
}
