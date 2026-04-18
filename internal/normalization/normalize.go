package normalization

import (
	"math"
	"sort"
	"strings"

	"github.com/user/pdf2md/internal/model"
)

// NormalizePage runs the 5-pass normalization on a single page and returns
// the resulting LogicalBlocks. If debug is true, a DebugData is also returned.
func NormalizePage(page *model.Page, opts Options, debug bool) ([]LogicalBlock, *DebugData) {
	lineSplitRatio := opts.LineSplitRatio
	if lineSplitRatio <= 0 {
		lineSplitRatio = DefaultLineSplitRatio
	}

	// Collect all VirtualLines from all flows/blocks.
	var allLines []*VirtualLine
	for fi := range page.Flows {
		for bi := range page.Flows[fi].Blocks {
			block := &page.Flows[fi].Blocks[bi]
			for li := range block.Lines {
				line := &block.Lines[li]
				vl := &VirtualLine{
					Words:         line.Words,
					XMin:          line.XMin,
					YMin:          line.YMin,
					XMax:          line.XMax,
					YMax:          line.YMax,
					FontSize:      line.FontSize,
					Role:          line.Role,
					Text:          line.Text,
					SourceLineRef: line,
					SourceBlock:   block,
					Virtual:       false,
				}
				allLines = append(allLines, vl)
			}
		}
	}

	if len(allLines) == 0 {
		return nil, nil
	}

	// Pass A: Collect gap candidates.
	var allGapCandidates []*GapCandidate
	passA(allLines, lineSplitRatio, &allGapCandidates)

	// Pass B: Vertical clustering of gap candidates.
	gapColumns := passB(allGapCandidates, allLines, lineSplitRatio)

	// Pass C: Line classification.
	passC(allLines, gapColumns)

	// Pass D: Vertical splitting of STRUCTURAL lines.
	allLines = passD(allLines)

	// Pass E: LogicalBlock reconstruction.
	logicalBlocks := passE(allLines)

	var dd *DebugData
	if debug {
		dd = &DebugData{
			VirtualLines:  allLines,
			GapCandidates: allGapCandidates,
			GapColumns:    gapColumns,
			LogicalBlocks: logicalBlocks,
		}
	}

	return logicalBlocks, dd
}

// ApplyNormalization runs normalization on each page of the document,
// converting LogicalBlocks back into model.Blocks and rebuilding flows.
func ApplyNormalization(doc *model.Document, opts Options, debug bool) {
	for i := range doc.Pages {
		page := &doc.Pages[i]
		logicalBlocks, dd := NormalizePage(page, opts, debug)

		if debug && dd != nil {
			page.NormDebugData = dd
		}

		if len(logicalBlocks) == 0 {
			continue
		}

		// Convert LogicalBlocks to model.Blocks and rebuild flows.
		var newBlocks []model.Block
		for _, lb := range logicalBlocks {
			var lines []model.Line
			for _, vl := range lb.Lines {
				lines = append(lines, virtualLineToModelLine(&vl))
			}
			newBlocks = append(newBlocks, model.Block{
				XMin:  lb.XMin,
				YMin:  lb.YMin,
				XMax:  lb.XMax,
				YMax:  lb.YMax,
				Lines: lines,
			})
		}

		// Rebuild flows: create a single flow containing all normalized blocks.
		var flowXMin, flowYMin, flowXMax, flowYMax float64
		var allLines []model.Line
		for i, block := range newBlocks {
			if i == 0 {
				flowXMin = block.XMin
				flowYMin = block.YMin
				flowXMax = block.XMax
				flowYMax = block.YMax
			} else {
				flowXMin = math.Min(flowXMin, block.XMin)
				flowYMin = math.Min(flowYMin, block.YMin)
				flowXMax = math.Max(flowXMax, block.XMax)
				flowYMax = math.Max(flowYMax, block.YMax)
			}
			allLines = append(allLines, block.Lines...)
		}

		page.Flows = []model.Flow{{
			XMin:   flowXMin,
			YMin:   flowYMin,
			XMax:   flowXMax,
			YMax:   flowYMax,
			Blocks: newBlocks,
			Lines:  allLines,
		}}
	}
}

// virtualLineToModelLine converts a VirtualLine to a model.Line.
func virtualLineToModelLine(vl *VirtualLine) model.Line {
	return model.Line{
		XMin:     vl.XMin,
		YMin:     vl.YMin,
		XMax:     vl.XMax,
		YMax:     vl.YMax,
		FontSize: vl.FontSize,
		Role:     vl.Role,
		Text:     vl.Text,
		Words:    vl.Words,
	}
}

// =========================================================================
// Pass A — Collect gap candidates (elbow detection)
// =========================================================================

func passA(lines []*VirtualLine, lineSplitRatio float64, out *[]*GapCandidate) {
	for _, vl := range lines {
		if len(vl.Words) < 2 {
			continue
		}

		// Compute all inter-word gaps.
		type gapInfo struct {
			width float64
			idx   int // index of the left word
		}
		var gaps []gapInfo
		for i := 0; i < len(vl.Words)-1; i++ {
			g := vl.Words[i+1].XMin - vl.Words[i].XMax
			if g > 0 {
				gaps = append(gaps, gapInfo{width: g, idx: i})
			}
		}

		if len(gaps) < 1 {
			continue
		}

		// Sort gaps by size ascending.
		sorted := make([]gapInfo, len(gaps))
		copy(sorted, gaps)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].width < sorted[j].width
		})

		// Elbow detection (only if enough words).
		if len(vl.Words) >= MinWordsForElbow && len(sorted) >= 2 {
			maxRatio := 0.0
			maxK := -1
			for i := 0; i < len(sorted)-1; i++ {
				if sorted[i].width <= 0 {
					continue
				}
				ratio := sorted[i+1].width / sorted[i].width
				if ratio > maxRatio {
					maxRatio = ratio
					maxK = i
				}
			}

			if maxRatio >= lineSplitRatio && maxK >= 0 {
				// All gaps in sorted[maxK+1:] are GapCandidates.
				yCenterLine := (vl.YMin + vl.YMax) / 2
				for _, g := range sorted[maxK+1:] {
					leftWord := vl.Words[g.idx]
					rightWord := vl.Words[g.idx+1]
					gc := &GapCandidate{
						XLeft:      leftWord.XMax,
						XRight:     rightWord.XMin,
						XCenter:    (leftWord.XMax + rightWord.XMin) / 2,
						YLine:      yCenterLine,
						Width:      g.width,
						Type:       GapUnclassified,
						ParentLine: vl,
					}
					vl.GapCandidates = append(vl.GapCandidates, gc)
					*out = append(*out, gc)
				}
			}
		}
	}
}

// =========================================================================
// Pass B — Vertical clustering of gap candidates
// =========================================================================

func passB(candidates []*GapCandidate, allLines []*VirtualLine, lineSplitRatio float64) []*GapColumn {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by YLine ascending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].YLine < candidates[j].YLine
	})

	// Group into Y-neighborhoods.
	var neighborhoods [][]*GapCandidate
	var current []*GapCandidate

	for _, gc := range candidates {
		if len(current) == 0 {
			current = append(current, gc)
			continue
		}
		if gc.YLine-current[len(current)-1].YLine < LineNeighborhood {
			current = append(current, gc)
		} else {
			neighborhoods = append(neighborhoods, current)
			current = []*GapCandidate{gc}
		}
	}
	if len(current) > 0 {
		neighborhoods = append(neighborhoods, current)
	}

	// Within each Y-neighborhood, group by XCenter similarity.
	var gapColumns []*GapColumn

	for _, hood := range neighborhoods {
		// Sort by XCenter.
		sort.Slice(hood, func(i, j int) bool {
			return hood[i].XCenter < hood[j].XCenter
		})

		var xGroups [][]*GapCandidate
		var xGroup []*GapCandidate

		for _, gc := range hood {
			if len(xGroup) == 0 {
				xGroup = append(xGroup, gc)
				continue
			}
			// Compare to the average XCenter of the group.
			avgX := 0.0
			for _, g := range xGroup {
				avgX += g.XCenter
			}
			avgX /= float64(len(xGroup))

			if math.Abs(gc.XCenter-avgX) <= CutGroupingTol {
				xGroup = append(xGroup, gc)
			} else {
				xGroups = append(xGroups, xGroup)
				xGroup = []*GapCandidate{gc}
			}
		}
		if len(xGroup) > 0 {
			xGroups = append(xGroups, xGroup)
		}

		for _, grp := range xGroups {
			avgX := 0.0
			for _, g := range grp {
				avgX += g.XCenter
			}
			avgX /= float64(len(grp))

			col := &GapColumn{
				XCenter: avgX,
				Gaps:    grp,
			}

			// Classify.
			if len(grp) >= MinStructuralLines {
				col.Type = gapColumnStructural // placeholder, will be set in Pass C
			} else if len(grp) == 1 {
				col.Type = GapIsolated // temp, will assign to gap
			}

			// Link gaps back to column.
			for _, gc := range grp {
				gc.VerticalGroup = col
			}

			gapColumns = append(gapColumns, col)
		}
	}

	// Mark single-gap columns as isolated.
	for _, col := range gapColumns {
		if len(col.Gaps) < MinStructuralLines {
			for _, gc := range col.Gaps {
				gc.Type = GapIsolated
			}
		}
	}

	// For structural columns: retroactively add GapCandidates to lines that were
	// skipped in Pass A (too few words) but have words spanning the column's X position.
	for _, col := range gapColumns {
		if len(col.Gaps) < MinStructuralLines {
			continue
		}
		for _, vl := range allLines {
			if len(vl.Words) < 2 {
				continue
			}
			// Check if this line already has a gap at this column.
			alreadyHas := false
			for _, gc := range vl.GapCandidates {
				if gc.VerticalGroup == col {
					alreadyHas = true
					break
				}
			}
			if alreadyHas {
				continue
			}

			// Check if words span across the column's X position.
			if vl.XMin >= col.XCenter || vl.XMax <= col.XCenter {
				continue
			}

			// Find the gap closest to the column X position.
			bestGap := findClosestGap(vl, col.XCenter)
			if bestGap != nil && math.Abs(bestGap.XCenter-col.XCenter) <= CutGroupingTol {
				bestGap.VerticalGroup = col
				vl.GapCandidates = append(vl.GapCandidates, bestGap)
				col.Gaps = append(col.Gaps, bestGap)
			}
		}
	}

	return gapColumns
}

// gapColumnStructural is a sentinel for structural columns before Pass C classifies them.
const gapColumnStructural GapType = -1

// findClosestGap finds or creates a GapCandidate in the line closest to targetX.
func findClosestGap(vl *VirtualLine, targetX float64) *GapCandidate {
	if len(vl.Words) < 2 {
		return nil
	}

	bestDist := math.MaxFloat64
	var bestGC *GapCandidate

	for i := 0; i < len(vl.Words)-1; i++ {
		gapLeft := vl.Words[i].XMax
		gapRight := vl.Words[i+1].XMin
		gapWidth := gapRight - gapLeft

		if gapWidth <= 0 {
			continue
		}

		center := (gapLeft + gapRight) / 2
		dist := math.Abs(center - targetX)
		if dist < bestDist {
			bestDist = dist
			bestGC = &GapCandidate{
				XLeft:      gapLeft,
				XRight:     gapRight,
				XCenter:    center,
				YLine:      (vl.YMin + vl.YMax) / 2,
				Width:      gapWidth,
				Type:       GapUnclassified,
				ParentLine: vl,
			}
		}
	}

	return bestGC
}

// =========================================================================
// Pass C — Line classification
// =========================================================================

func passC(lines []*VirtualLine, gapColumns []*GapColumn) {
	// First sub-pass: classify each line.
	for _, vl := range lines {
		// Check for structural gap candidates.
		hasStructural := false
		for _, gc := range vl.GapCandidates {
			if gc.VerticalGroup != nil && len(gc.VerticalGroup.Gaps) >= MinStructuralLines {
				hasStructural = true
				break
			}
		}

		if hasStructural {
			vl.Classification = LineStructural
		} else {
			classifyByAlignment(vl)
		}
	}

	// Second sub-pass: classify gap columns based on line classifications.
	for _, col := range gapColumns {
		if len(col.Gaps) < MinStructuralLines {
			col.Type = GapIsolated
			for _, gc := range col.Gaps {
				gc.Type = GapIsolated
			}
			continue
		}

		// Check if the lines in this column are predominantly structural or text.
		hasTextLines := false
		for _, gc := range col.Gaps {
			if gc.ParentLine != nil {
				cls := gc.ParentLine.Classification
				if cls == LineLeft || cls == LineRight || cls == LineJustified {
					hasTextLines = true
					break
				}
			}
		}

		if hasTextLines {
			col.Type = GapColumnType
		} else {
			col.Type = GapTableType
		}

		for _, gc := range col.Gaps {
			gc.Type = col.Type
		}
	}
}


func classifyByAlignment(vl *VirtualLine) {
	if vl.SourceBlock == nil || len(vl.Words) == 0 {
		vl.Classification = LineLeft
		return
	}

	block := vl.SourceBlock
	blockWidth := block.XMax - block.XMin

	if blockWidth <= 0 {
		vl.Classification = LineLeft
		return
	}

	firstWord := vl.Words[0]
	lastWord := vl.Words[len(vl.Words)-1]

	emptyLeft := firstWord.XMin - block.XMin
	emptyRight := block.XMax - lastWord.XMax
	textSpan := lastWord.XMax - firstWord.XMin
	coverage := textSpan / blockWidth

	// Compute gap variance (sigma).
	var gapSigma float64
	if len(vl.Words) >= 2 {
		var gaps []float64
		for i := 0; i < len(vl.Words)-1; i++ {
			g := vl.Words[i+1].XMin - vl.Words[i].XMax
			if g > 0 {
				gaps = append(gaps, g)
			}
		}
		if len(gaps) > 0 {
			mean := 0.0
			for _, g := range gaps {
				mean += g
			}
			mean /= float64(len(gaps))
			variance := 0.0
			for _, g := range gaps {
				diff := g - mean
				variance += diff * diff
			}
			variance /= float64(len(gaps))
			gapSigma = math.Sqrt(variance)
		}
	}

	// Store debug info.
	vl.EmptyLeft = emptyLeft
	vl.EmptyRight = emptyRight
	vl.Coverage = coverage
	vl.GapSigma = gapSigma

	// Classification rules (in order).
	if coverage >= JustifiedCoverage && gapSigma > 0 {
		vl.Classification = LineJustified
	} else if emptyRight >= AlignMargin && emptyLeft < AlignMargin {
		vl.Classification = LineLeft
	} else if emptyLeft >= AlignMargin && emptyRight < AlignMargin {
		vl.Classification = LineRight
	} else {
		vl.Classification = LineLeft // fallback
	}
}

// =========================================================================
// Pass D — Vertical splitting of STRUCTURAL lines
// =========================================================================

func passD(lines []*VirtualLine) []*VirtualLine {
	var result []*VirtualLine

	for _, vl := range lines {
		if vl.Classification != LineStructural {
			result = append(result, vl)
			continue
		}

		// Collect structural gap positions (sorted by XCenter).
		var splitPositions []float64
		for _, gc := range vl.GapCandidates {
			if gc.VerticalGroup != nil && len(gc.VerticalGroup.Gaps) >= MinStructuralLines {
				splitPositions = append(splitPositions, gc.XCenter)
			}
		}

		if len(splitPositions) == 0 {
			result = append(result, vl)
			continue
		}

		sort.Float64s(splitPositions)

		// Split words at each gap position.
		clusters := splitWordsAtPositions(vl.Words, splitPositions)

		for _, cluster := range clusters {
			if len(cluster) == 0 {
				continue
			}

			subLine := &VirtualLine{
				Words:          cluster,
				XMin:           cluster[0].XMin,
				YMin:           vl.YMin,
				XMax:           cluster[len(cluster)-1].XMax,
				YMax:           vl.YMax,
				FontSize:       vl.FontSize,
				Role:           vl.Role,
				Classification: LineLeft, // default within cell
				SourceLineRef:  vl.SourceLineRef,
				SourceBlock:    vl.SourceBlock,
				Virtual:        true,
			}

			// Rebuild text from words.
			var parts []string
			for _, w := range cluster {
				if w.Text != "" {
					parts = append(parts, w.Text)
				}
			}
			subLine.Text = strings.Join(parts, " ")

			// Recalculate bounding box from words.
			for _, w := range cluster {
				subLine.XMin = math.Min(subLine.XMin, w.XMin)
				subLine.YMin = math.Min(subLine.YMin, w.YMin)
				subLine.XMax = math.Max(subLine.XMax, w.XMax)
				subLine.YMax = math.Max(subLine.YMax, w.YMax)
			}

			result = append(result, subLine)
		}
	}

	return result
}

// splitWordsAtPositions splits words into clusters at the given X positions.
func splitWordsAtPositions(words []model.Word, positions []float64) [][]model.Word {
	if len(positions) == 0 {
		return [][]model.Word{words}
	}

	// Sort words by XMin.
	sorted := make([]model.Word, len(words))
	copy(sorted, words)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].XMin < sorted[j].XMin
	})

	var clusters [][]model.Word
	var current []model.Word
	posIdx := 0

	for _, w := range sorted {
		wordCenter := (w.XMin + w.XMax) / 2

		// Check if we've passed a split position.
		if posIdx < len(positions) && wordCenter > positions[posIdx] {
			if len(current) > 0 {
				clusters = append(clusters, current)
				current = nil
			}
			posIdx++
			// Skip any additional positions we've passed.
			for posIdx < len(positions) && wordCenter > positions[posIdx] {
				posIdx++
			}
		}

		current = append(current, w)
	}
	if len(current) > 0 {
		clusters = append(clusters, current)
	}

	return clusters
}

// =========================================================================
// Pass E — LogicalBlock reconstruction
// =========================================================================

func passE(lines []*VirtualLine) []LogicalBlock {
	if len(lines) == 0 {
		return nil
	}

	// Sort all VirtualLine objects by YMin.
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].YMin != lines[j].YMin {
			return lines[i].YMin < lines[j].YMin
		}
		return lines[i].XMin < lines[j].XMin
	})

	var blocks []LogicalBlock

	// Greedy accumulator.
	current := newLogicalBlock(lines[0])

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		prevLine := lines[i-1]

		yGap := line.YMin - prevLine.YMax
		sameFamily := isSameClassFamily(current.Lines[len(current.Lines)-1].Classification, line.Classification)

		// Poppler block separation constraint: check if there's a block boundary
		// with a Y gap >= MergeGapThreshold between the parent blocks.
		blockSepOK := checkBlockSeparation(prevLine, line)

		if sameFamily && yGap < MergeGapThreshold && blockSepOK {
			appendToLogicalBlock(&current, line)
		} else {
			finalizeLogicalBlock(&current)
			blocks = append(blocks, current)
			current = newLogicalBlock(line)
		}
	}

	finalizeLogicalBlock(&current)
	blocks = append(blocks, current)

	return blocks
}

func newLogicalBlock(line *VirtualLine) LogicalBlock {
	lb := LogicalBlock{
		Lines: []VirtualLine{*line},
		XMin:  line.XMin,
		YMin:  line.YMin,
		XMax:  line.XMax,
		YMax:  line.YMax,
	}
	if line.SourceBlock != nil {
		lb.SourceBlocks = []*model.Block{line.SourceBlock}
	}
	lb.Virtual = line.Virtual
	return lb
}

func appendToLogicalBlock(lb *LogicalBlock, line *VirtualLine) {
	lb.Lines = append(lb.Lines, *line)
	lb.XMin = math.Min(lb.XMin, line.XMin)
	lb.YMin = math.Min(lb.YMin, line.YMin)
	lb.XMax = math.Max(lb.XMax, line.XMax)
	lb.YMax = math.Max(lb.YMax, line.YMax)

	if line.SourceBlock != nil {
		found := false
		for _, b := range lb.SourceBlocks {
			if b == line.SourceBlock {
				found = true
				break
			}
		}
		if !found {
			lb.SourceBlocks = append(lb.SourceBlocks, line.SourceBlock)
		}
	}
	if line.Virtual {
		lb.Virtual = true
	}
}

func finalizeLogicalBlock(lb *LogicalBlock) {
	hasStructural := false
	for _, line := range lb.Lines {
		if line.Classification == LineStructural {
			hasStructural = true
			break
		}
	}
	if hasStructural {
		lb.Type = BlockStructural
	} else {
		lb.Type = BlockText
	}
}

// isSameClassFamily checks if two line classifications are in the same family.
// TEXT family: LineLeft, LineRight, LineJustified
// STRUCTURAL family: LineStructural
func isSameClassFamily(a, b LineType) bool {
	aText := a == LineLeft || a == LineRight || a == LineJustified || a == LineUnclassified
	bText := b == LineLeft || b == LineRight || b == LineJustified || b == LineUnclassified
	return aText == bText
}

// checkBlockSeparation verifies the Poppler block separation constraint.
// Two lines from different Poppler blocks can be in the same LogicalBlock
// only if the Y gap between their parent blocks is < MergeGapThreshold.
func checkBlockSeparation(prev, curr *VirtualLine) bool {
	if prev.SourceBlock == nil || curr.SourceBlock == nil {
		return true
	}
	if prev.SourceBlock == curr.SourceBlock {
		return true
	}

	// Check if the blocks are separated by a large Y gap.
	blockGap := curr.SourceBlock.YMin - prev.SourceBlock.YMax
	return blockGap < MergeGapThreshold
}
