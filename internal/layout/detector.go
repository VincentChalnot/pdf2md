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

	// HeadingLineHeightThreshold is the maximum line height (in PDF points)
	// above which a block is considered a heading and excluded from layout detection
	HeadingLineHeightThreshold = 14.0
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

// DetectLayout performs X-Y cut analysis on a page and returns layout zones.
// bodyLineHeight is the standard body text line height used for band merging thresholds.
func DetectLayout(page *model.Page, bodyLineHeight float64) *PageLayout {
	if page == nil {
		return &PageLayout{}
	}

	// Collect all blocks from all flows and mark heading blocks
	var allBlocks []model.Block
	for flowIdx, flow := range page.Flows {
		for blockIdx, block := range flow.Blocks {
			// Mark heading blocks in the original flow data
			if isHeadingBlock(&block) {
				page.Flows[flowIdx].Blocks[blockIdx].IsHeading = true
				block.IsHeading = true // Also mark in the copy
			}
			allBlocks = append(allBlocks, block)
		}
	}

	if len(allBlocks) == 0 {
		return &PageLayout{}
	}

	// Filter out heading blocks for layout detection
	var layoutBlocks []model.Block
	for _, block := range allBlocks {
		if !block.IsHeading {
			layoutBlocks = append(layoutBlocks, block)
		}
	}

	// If all blocks are headings, return empty layout
	if len(layoutBlocks) == 0 {
		return &PageLayout{}
	}

	// Step 1: Find horizontal bands with gap threshold based on body line height
	bandGapThreshold := math.Max(MinGapThreshold, bodyLineHeight*1.2)
	bands, _ := findHorizontalBands(layoutBlocks, page.Width, bandGapThreshold)

	// Step 2: Find vertical cuts for each band (X-axis projection)
	for _, band := range bands {
		band.VerticalCuts = findVerticalCuts(band.Blocks)
	}

	// Step 4: Merge consecutive single-column bands
	bands = mergeSingleColumnBands(bands)

	// Step 5: Recalculate vertical cuts for merged bands
	for _, band := range bands {
		band.VerticalCuts = findVerticalCuts(band.Blocks)
	}

	// Step 6: Group bands by column structure into layout zones
	zones := groupBandsIntoZones(bands)

	// Step 7: Recalculate horizontal cuts from final bands and filter
	hCuts := recalculateHorizontalCuts(bands, page.Width)
	filteredCuts := filterHorizontalCuts(bands, hCuts)

	// Step 8: Characterize each zone (compute metrics)
	for i, zone := range zones {
		zone.Index = i
		characterizeZone(zone)
	}

	return &PageLayout{
		Zones:          zones,
		HorizontalCuts: filteredCuts,
	}
}

// isHeadingBlock checks if a block contains large text (heading)
func isHeadingBlock(block *model.Block) bool {
	if len(block.Lines) == 0 {
		return false
	}

	// Compute maximum line height across all lines
	var maxLineHeight float64
	for _, line := range block.Lines {
		lineHeight := line.YMax - line.YMin
		if lineHeight > maxLineHeight {
			maxLineHeight = lineHeight
		}
	}

	return maxLineHeight > HeadingLineHeightThreshold
}

// filterHorizontalCuts removes cuts between consecutive mono-column bands
func filterHorizontalCuts(bands []*HorizontalBand, cuts []HorizontalCut) []HorizontalCut {
	if len(cuts) == 0 || len(bands) < 2 {
		return cuts
	}

	var filtered []HorizontalCut

	for _, cut := range cuts {
		// Find the two bands on either side of this cut
		var bandAbove, bandBelow *HorizontalBand

		for i := 0; i < len(bands)-1; i++ {
			// Check if this cut is between bands[i] and bands[i+1]
			if bands[i].YMax <= cut.Y && cut.Y <= bands[i+1].YMin {
				bandAbove = bands[i]
				bandBelow = bands[i+1]
				break
			}
		}

		// If we found both bands, check if at least one is multi-column
		if bandAbove != nil && bandBelow != nil {
			hasAboveCuts := len(bandAbove.VerticalCuts) > 0
			hasBelowCuts := len(bandBelow.VerticalCuts) > 0

			// Only keep cut if at least one band is multi-column
			if hasAboveCuts || hasBelowCuts {
				filtered = append(filtered, cut)
			}
		} else {
			// Keep the cut if we can't determine (shouldn't happen in practice)
			filtered = append(filtered, cut)
		}
	}

	return filtered
}

// findHorizontalBands performs Y-axis projection and identifies horizontal bands
func findHorizontalBands(blocks []model.Block, pageWidth, gapThreshold float64) ([]*HorizontalBand, []HorizontalCut) {
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
	occupied := mergeIntervals(intervals, gapThreshold)

	// Find gaps between occupied regions
	var cuts []HorizontalCut
	for i := 0; i < len(occupied)-1; i++ {
		gapStart := occupied[i].max
		gapEnd := occupied[i+1].min
		gapSize := gapEnd - gapStart

		if gapSize >= gapThreshold {
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
	occupied := mergeIntervals(intervals, MinGapThreshold)

	// Find gaps between occupied regions
	var cuts []float64
	for i := 0; i < len(occupied)-1; i++ {
		gapStart := occupied[i].max
		gapEnd := occupied[i+1].min
		gapSize := gapEnd - gapStart

		if gapSize >= MinGapThreshold {
			cuts = append(cuts, (gapStart+gapEnd)/2)
		}
	}

	return cuts
}

// mergeIntervals merges overlapping intervals (assumes sorted by min)
// Only merges if they truly overlap or are within gapThreshold
func mergeIntervals(intervals []interval, gapThreshold float64) []interval {
	if len(intervals) == 0 {
		return nil
	}

	merged := []interval{intervals[0]}
	for i := 1; i < len(intervals); i++ {
		last := &merged[len(merged)-1]
		current := intervals[i]

		// Calculate gap between last and current
		gap := current.min - last.max

		if gap < gapThreshold {
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
	var currentZone *LayoutZone

	for _, band := range bands {
		hasCuts := len(band.VerticalCuts) > 0

		isSingleLine := BandLineCount(band) == 1

		if currentZone == nil {
			// Start first zone
			currentZone = &LayoutZone{
				Bands:        []*HorizontalBand{band},
				VerticalCuts: band.VerticalCuts,
			}
			continue
		}

		// Check if this band is compatible with the current zone
		// New rule: bands are compatible if they share at least one vertical cut position
		// Single-line bands don't interfere with layout detection, so they are always compatible

		// Check if the current zone ONLY contains single-line bands
		onlySingleLine := true
		for _, b := range currentZone.Bands {
			if BandLineCount(b) != 1 {
				onlySingleLine = false
				break
			}
		}

		// If the current zone only has single-line bands (so no vertical cuts established yet),
		// the next band dictates the zone's vertical cuts.
		adoptCuts := onlySingleLine && hasCuts

		if isSingleLine || adoptCuts || cutsSharePosition(currentZone.VerticalCuts, band.VerticalCuts) {
			// Add band to current zone
			currentZone.Bands = append(currentZone.Bands, band)
			// Keep track of all unique vertical cuts in the zone
			if hasCuts && !isSingleLine {
				currentZone.VerticalCuts = mergeVerticalCuts(currentZone.VerticalCuts, band.VerticalCuts)
			}
		} else {
			// Close current zone and start a new one
			zones = append(zones, currentZone)
			currentZone = &LayoutZone{
				Bands:        []*HorizontalBand{band},
				VerticalCuts: band.VerticalCuts,
			}
		}
	}

	// Add the last zone
	if currentZone != nil {
		zones = append(zones, currentZone)
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

// cutsSharePosition checks if two sets of vertical cuts share at least one position within tolerance
func cutsSharePosition(cuts1, cuts2 []float64) bool {
	// Both have no cuts - they are compatible (both single-column)
	if len(cuts1) == 0 && len(cuts2) == 0 {
		return true
	}

	// One has cuts, the other doesn't - not compatible
	if len(cuts1) == 0 || len(cuts2) == 0 {
		return false
	}

	// Check if any cut in cuts1 matches any cut in cuts2 within tolerance
	for _, c1 := range cuts1 {
		for _, c2 := range cuts2 {
			if math.Abs(c1-c2) <= ColumnGroupingTolerance {
				return true
			}
		}
	}

	return false
}

// mergeVerticalCuts combines two sets of vertical cuts, merging positions within tolerance
func mergeVerticalCuts(cuts1, cuts2 []float64) []float64 {
	if len(cuts1) == 0 {
		return cuts2
	}
	if len(cuts2) == 0 {
		return cuts1
	}

	// Start with cuts1
	result := make([]float64, len(cuts1))
	copy(result, cuts1)

	// Add cuts from cuts2 that don't match any existing cut
	for _, c2 := range cuts2 {
		found := false
		for _, c1 := range result {
			if math.Abs(c1-c2) <= ColumnGroupingTolerance {
				found = true
				break
			}
		}
		if !found {
			result = append(result, c2)
		}
	}

	// Sort result
	sort.Float64s(result)
	return result
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

	// Band count (excluding single-line bands as they don't count towards the layout detection maximums)
	zone.BandCount = 0
	for _, band := range zone.Bands {
		if BandLineCount(band) != 1 {
			zone.BandCount++
		}
	}
	// Fallback if all bands were single-line
	if zone.BandCount == 0 && len(zone.Bands) > 0 {
		zone.BandCount = 1
	}

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

// BandLineCount returns the total number of lines across all blocks in a band.
func BandLineCount(band *HorizontalBand) int {
	count := 0
	for _, block := range band.Blocks {
		count += len(block.Lines)
	}
	return count
}

// mergeBandInto merges the source band into the target band.
func mergeBandInto(target, source *HorizontalBand) {
	target.Blocks = append(target.Blocks, source.Blocks...)
	target.XMin = math.Min(target.XMin, source.XMin)
	target.XMax = math.Max(target.XMax, source.XMax)
	target.YMin = math.Min(target.YMin, source.YMin)
	target.YMax = math.Max(target.YMax, source.YMax)
}

// mergeSingleColumnBands merges consecutive bands that both have a single column
// (no vertical cuts), even if lines are not aligned.
func mergeSingleColumnBands(bands []*HorizontalBand) []*HorizontalBand {
	if len(bands) <= 1 {
		return bands
	}

	result := []*HorizontalBand{bands[0]}

	for i := 1; i < len(bands); i++ {
		prev := result[len(result)-1]
		curr := bands[i]

		// Both are single-column (no vertical cuts)
		if len(prev.VerticalCuts) == 0 && len(curr.VerticalCuts) == 0 {
			mergeBandInto(prev, curr)
		} else {
			result = append(result, curr)
		}
	}

	return result
}

// recalculateHorizontalCuts creates horizontal cuts between the final bands.
func recalculateHorizontalCuts(bands []*HorizontalBand, pageWidth float64) []HorizontalCut {
	if len(bands) < 2 {
		return nil
	}

	var cuts []HorizontalCut
	for i := 0; i < len(bands)-1; i++ {
		gapStart := bands[i].YMax
		gapEnd := bands[i+1].YMin
		if gapEnd > gapStart {
			cuts = append(cuts, HorizontalCut{
				Y:    (gapStart + gapEnd) / 2,
				XMin: 0,
				XMax: pageWidth,
			})
		}
	}
	return cuts
}
