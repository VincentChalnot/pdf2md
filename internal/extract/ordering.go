package extract

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/user/pdf2md/internal/model"
)

// EstablishReadingOrder orders flows within each page for reading.
// This includes column detection, drop cap merging, and final ordering.
func EstablishReadingOrder(doc *model.Document) {
	// Get body size for drop cap detection.
	bodySize := getBodySize(doc)

	for i := range doc.Pages {
		page := &doc.Pages[i]

		if len(page.Flows) == 0 {
			continue
		}

		// Step 1: Identify parallel column groups.
		columnGroups := IdentifyColumnGroups(page.Flows)

		// Step 2: Drop cap detection and merging.
		page.Flows = mergeDropCaps(page.Flows, columnGroups, bodySize)

		// Step 3: Final ordering.
		page.Flows = orderFlows(page.Flows)
	}
}

// getBodySize returns the body font size from the document's FontMap.
func getBodySize(doc *model.Document) float64 {
	for _, fs := range doc.FontMap {
		if fs.Role == model.RoleBody {
			return fs.Size
		}
	}
	return 10.0 // default fallback
}

// columnGroup represents a set of flows that form parallel columns.
type columnGroup struct {
	Flows []int // indices into page.Flows
}

// identifyColumnGroups finds flows that are parallel columns.
// Two flows are parallel if their Y ranges overlap by >40% of the shorter flow's height
// AND their X ranges do not overlap.
func IdentifyColumnGroups(flows []model.Flow) []columnGroup {
	n := len(flows)
	if n == 0 {
		return nil
	}

	// Build adjacency list for flows that are parallel.
	adj := make([][]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if areParallel(flows[i], flows[j]) {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	// Group connected components.
	visited := make([]bool, n)
	var groups []columnGroup

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}

		// BFS to find connected component.
		var group []int
		queue := []int{i}
		visited[i] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			group = append(group, curr)

			for _, neighbor := range adj[curr] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}

		// Sort group by xMin for column ordering.
		sort.Slice(group, func(a, b int) bool {
			return flows[group[a]].XMin < flows[group[b]].XMin
		})

		groups = append(groups, columnGroup{Flows: group})
	}

	return groups
}

// areParallel checks if two flows are parallel columns.
func areParallel(f1, f2 model.Flow) bool {
	// Check if X ranges do not overlap.
	xOverlap := !(f1.XMax <= f2.XMin || f2.XMax <= f1.XMin)
	if xOverlap {
		return false
	}

	// Check if Y ranges overlap by >40% of shorter flow's height.
	h1 := f1.YMax - f1.YMin
	h2 := f2.YMax - f2.YMin
	shorterHeight := math.Min(h1, h2)

	yOverlapStart := math.Max(f1.YMin, f2.YMin)
	yOverlapEnd := math.Min(f1.YMax, f2.YMax)
	yOverlap := yOverlapEnd - yOverlapStart

	if yOverlap < 0 {
		yOverlap = 0
	}

	return yOverlap > shorterHeight*0.4
}

// mergeDropCaps detects drop caps and merges them into the next flow.
func mergeDropCaps(flows []model.Flow, groups []columnGroup, bodySize float64) []model.Flow {
	if len(flows) == 0 {
		return flows
	}

	// Build a map of flow index to column group and position within group.
	flowToGroup := make(map[int]int)
	flowToPos := make(map[int]int)
	for gi, g := range groups {
		for pi, fi := range g.Flows {
			flowToGroup[fi] = gi
			flowToPos[fi] = pi
		}
	}

	toRemove := make(map[int]bool)
	merged := make(map[int]bool)

	for i, flow := range flows {
		if merged[i] {
			continue
		}

		// Check if this is a drop cap.
		if isDropCap(flow, bodySize) {
			// Find the next flow in the same column group.
			gi, inGroup := flowToGroup[i]
			if !inGroup {
				continue
			}

			pos := flowToPos[i]
			if pos+1 < len(groups[gi].Flows) {
				nextIdx := groups[gi].Flows[pos+1]

				// Prepend drop cap letter to first line of next flow.
				if len(flows[nextIdx].Lines) > 0 {
					dropCapLetter := getDropCapLetter(flow)
					flows[nextIdx].Lines[0].Text = dropCapLetter + flows[nextIdx].Lines[0].Text

					// Mark drop cap flow for removal.
					toRemove[i] = true
					merged[nextIdx] = true
				}
			}
		}
	}

	// Build result without removed flows.
	var result []model.Flow
	for i, flow := range flows {
		if !toRemove[i] {
			result = append(result, flow)
		}
	}

	return result
}

// isDropCap checks if a flow is a drop cap.
func isDropCap(flow model.Flow, bodySize float64) bool {
	// Must have exactly 1 line with 1 word (letter).
	if len(flow.Lines) != 1 {
		return false
	}

	line := flow.Lines[0]
	words := strings.Fields(line.Text)
	if len(words) != 1 {
		return false
	}

	word := words[0]
	// Must be a single uppercase letter.
	runes := []rune(word)
	if len(runes) != 1 {
		return false
	}

	if !unicode.IsUpper(runes[0]) {
		return false
	}

	// Font size must be > body × 3.0.
	return line.FontSize > bodySize*3.0
}

// getDropCapLetter extracts the letter from a drop cap flow.
func getDropCapLetter(flow model.Flow) string {
	if len(flow.Lines) == 0 {
		return ""
	}
	return strings.TrimSpace(flow.Lines[0].Text)
}

// mergeBandFlows merges splitted lines inside the same column within a band.
func mergeBandFlows(flows []model.Flow) []model.Flow {
	n := len(flows)
	if n == 0 {
		return nil
	}

	adj := make([][]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			f1, f2 := flows[i], flows[j]
			xOverlap := !(f1.XMax <= f2.XMin || f2.XMax <= f1.XMin)
			if xOverlap {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	visited := make([]bool, n)
	var colGroups [][]model.Flow

	for i := 0; i < n; i++ {
		if !visited[i] {
			var group []model.Flow
			q := []int{i}
			visited[i] = true
			for len(q) > 0 {
				curr := q[0]
				q = q[1:]
				group = append(group, flows[curr])
				for _, neighbor := range adj[curr] {
					if !visited[neighbor] {
						visited[neighbor] = true
						q = append(q, neighbor)
					}
				}
			}
			colGroups = append(colGroups, group)
		}
	}

	var merged []model.Flow
	for _, group := range colGroups {
		if len(group) == 1 {
			merged = append(merged, group[0])
			continue
		}

		var allLines []model.Line
		fXMin, fYMin := math.MaxFloat64, math.MaxFloat64
		fXMax, fYMax := -math.MaxFloat64, -math.MaxFloat64

		for _, f := range group {
			fXMin = math.Min(fXMin, f.XMin)
			fYMin = math.Min(fYMin, f.YMin)
			fXMax = math.Max(fXMax, f.XMax)
			fYMax = math.Max(fYMax, f.YMax)
			allLines = append(allLines, f.Lines...)
		}

		sort.Slice(allLines, func(i, j int) bool {
			if math.Abs(allLines[i].YMin-allLines[j].YMin) > 2.0 {
				return allLines[i].YMin < allLines[j].YMin
			}
			return allLines[i].XMin < allLines[j].XMin
		})

		var mergedLines []model.Line
		var currLines []model.Line

		flush := func(g []model.Line) {
			if len(g) == 0 {
				return
			}
			sort.Slice(g, func(i, j int) bool { return g[i].XMin < g[j].XMin })

			lXMin := g[0].XMin
			lYMin := g[0].YMin
			lXMax := g[len(g)-1].XMax
			lYMax := g[0].YMax

			var textBuilder strings.Builder
			for i, l := range g {
				if i > 0 {
					prev := g[i-1]
					gap := l.XMin - prev.XMax
					charWidth := l.FontSize * 0.5
					if charWidth <= 0 {
						charWidth = 3.0
					}
					numSpaces := int(math.Round(gap / charWidth))
					if numSpaces < 1 {
						numSpaces = 1
					}
					if numSpaces > 30 {
						numSpaces = 30
					}
					textBuilder.WriteString(strings.Repeat(" ", numSpaces))
				}
				textBuilder.WriteString(l.Text)
				lYMax = math.Max(lYMax, l.YMax)
			}
			mergedLines = append(mergedLines, model.Line{
				XMin:     lXMin,
				YMin:     lYMin,
				XMax:     lXMax,
				YMax:     lYMax,
				FontSize: g[0].FontSize,
				Role:     func() model.FontRole { if len(g) > 1 { return model.RoleTable }; return g[0].Role }(),
				Text:     textBuilder.String(),
			})
		}

		for _, l := range allLines {
			if len(currLines) == 0 {
				currLines = append(currLines, l)
				continue
			}
			if math.Abs(l.YMin-currLines[0].YMin) <= 2.0 {
				currLines = append(currLines, l)
			} else {
				flush(currLines)
				currLines = []model.Line{l}
			}
		}
		flush(currLines)

		// Create a single flow with the merged lines
		var blocks []model.Block
		if len(mergedLines) > 0 {
			blocks = []model.Block{{XMin: fXMin, YMin: fYMin, XMax: fXMax, YMax: fYMax, Lines: mergedLines}}
		}
		merged = append(merged, model.Flow{
			XMin:   fXMin,
			YMin:   fYMin,
			XMax:   fXMax,
			YMax:   fYMax,
			Lines:  mergedLines,
			Blocks: blocks,
		})
	}
	return merged
}

// orderFlows orders flows by forming horizontal bands and reading column-by-column inside each band.
func orderFlows(flows []model.Flow) []model.Flow {
	if len(flows) == 0 {
		return flows
	}

	// 1. Sort all flows initially by YMin to process top-to-bottom
	sorted := make([]model.Flow, len(flows))
	copy(sorted, flows)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].YMin < sorted[j].YMin
	})

	// 2. Group into Horizontal Bands based on Y-overlap
	type Band struct {
		yMax  float64
		flows []model.Flow
	}
	var bands []Band

	for _, flow := range sorted {
		if len(bands) == 0 {
			bands = append(bands, Band{yMax: flow.YMax, flows: []model.Flow{flow}})
			continue
		}

		lastIdx := len(bands) - 1
		// Check if flow overlaps vertically with the current band.
		// A flow overlaps if its top is above the band's bottom (with a 1.0 point tolerance)
		if flow.YMin < bands[lastIdx].yMax-1.0 {
			bands[lastIdx].flows = append(bands[lastIdx].flows, flow)
			if flow.YMax > bands[lastIdx].yMax {
				bands[lastIdx].yMax = flow.YMax
			}
		} else {
			bands = append(bands, Band{yMax: flow.YMax, flows: []model.Flow{flow}})
		}
	}

	// 3. Sort within each band
	var result []model.Flow
	for _, band := range bands {
		bandMerged := mergeBandFlows(band.flows)

		sort.Slice(bandMerged, func(i, j int) bool {
			f1, f2 := bandMerged[i], bandMerged[j]

			// Primary: XMin (Column by Column left to right)
			if math.Abs(f1.XMin-f2.XMin) > 10.0 {
				return f1.XMin < f2.XMin
			}

			// Secondary: YMin (Top to bottom within the same column)
			if math.Abs(f1.YMin-f2.YMin) > 1.0 {
				return f1.YMin < f2.YMin
			}

			// Fallback: XMax
			return f1.XMax < f2.XMax
		})

		result = append(result, bandMerged...)
	}

	return result
}
