package pre_process

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// ReadingOrderHandler establishes the correct reading order for each page.
// It performs three sub-steps in sequence:
//  1. Identify parallel column groups (flows side-by-side on the page).
//  2. Merge drop caps into the following flow.
//  3. Order flows into horizontal bands, merging co-located fragments into
//     table-like lines and sorting left-to-right, top-to-bottom.
//
// Priority: 30 (runs after FontRoles in PreProcess).
type ReadingOrderHandler struct{}

// NewReadingOrderHandler returns a ReadingOrderHandler.
func NewReadingOrderHandler() *ReadingOrderHandler { return &ReadingOrderHandler{} }

func (h *ReadingOrderHandler) Event() event.Event { return event.PreProcess }
func (h *ReadingOrderHandler) Priority() int      { return 30 }

func (h *ReadingOrderHandler) Run(doc *model.Document) error {
	bodySize := getBodySize(doc)

	for i := range doc.Pages {
		page := &doc.Pages[i]
		if len(page.Flows) == 0 {
			continue
		}

		columnGroups := identifyColumnGroups(page.Flows)
		page.Flows = mergeDropCaps(page.Flows, columnGroups, bodySize)
		page.Flows = orderFlows(page.Flows)
	}
	return nil
}

// getBodySize returns the body font size from the document's FontMap.
func getBodySize(doc *model.Document) float64 {
	for _, fs := range doc.FontMap {
		if fs.Role == model.RoleBody {
			return fs.Size
		}
	}
	return 10.0
}

// columnGroup represents a set of flows that form parallel columns.
type columnGroup struct {
	flows []int // indices into page.Flows
}

// identifyColumnGroups finds flows that sit side-by-side as parallel columns.
// Two flows are parallel if their X ranges do not overlap and their Y ranges
// overlap by more than 40% of the shorter flow's height.
func identifyColumnGroups(flows []model.Flow) []columnGroup {
	n := len(flows)
	if n == 0 {
		return nil
	}

	adj := make([][]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if areParallel(flows[i], flows[j]) {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	visited := make([]bool, n)
	var groups []columnGroup

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		var group []int
		queue := []int{i}
		visited[i] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			group = append(group, curr)
			for _, nb := range adj[curr] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		sort.Slice(group, func(a, b int) bool {
			return flows[group[a]].XMin < flows[group[b]].XMin
		})
		groups = append(groups, columnGroup{flows: group})
	}

	return groups
}

func areParallel(f1, f2 model.Flow) bool {
	xOverlap := !(f1.XMax <= f2.XMin || f2.XMax <= f1.XMin)
	if xOverlap {
		return false
	}
	h1 := f1.YMax - f1.YMin
	h2 := f2.YMax - f2.YMin
	shorter := math.Min(h1, h2)
	yStart := math.Max(f1.YMin, f2.YMin)
	yEnd := math.Min(f1.YMax, f2.YMax)
	yOverlap := yEnd - yStart
	if yOverlap < 0 {
		yOverlap = 0
	}
	return yOverlap > shorter*0.4
}

// mergeDropCaps detects drop-cap flows (single oversized uppercase letter) and
// prepends the letter to the first line of the following flow in the same column group.
func mergeDropCaps(flows []model.Flow, groups []columnGroup, bodySize float64) []model.Flow {
	if len(flows) == 0 {
		return flows
	}

	flowToGroup := make(map[int]int)
	flowToPos := make(map[int]int)
	for gi, g := range groups {
		for pi, fi := range g.flows {
			flowToGroup[fi] = gi
			flowToPos[fi] = pi
		}
	}

	toRemove := make(map[int]bool)
	merged := make(map[int]bool)

	for i, flow := range flows {
		if merged[i] || !isDropCap(flow, bodySize) {
			continue
		}
		gi, inGroup := flowToGroup[i]
		if !inGroup {
			continue
		}
		pos := flowToPos[i]
		if pos+1 < len(groups[gi].flows) {
			nextIdx := groups[gi].flows[pos+1]
			if len(flows[nextIdx].Lines) > 0 {
				letter := strings.TrimSpace(flows[i].Lines[0].Text)
				flows[nextIdx].Lines[0].Text = letter + flows[nextIdx].Lines[0].Text
				toRemove[i] = true
				merged[nextIdx] = true
			}
		}
	}

	var result []model.Flow
	for i, flow := range flows {
		if !toRemove[i] {
			result = append(result, flow)
		}
	}
	return result
}

func isDropCap(flow model.Flow, bodySize float64) bool {
	if len(flow.Lines) != 1 {
		return false
	}
	line := flow.Lines[0]
	words := strings.Fields(line.Text)
	if len(words) != 1 {
		return false
	}
	runes := []rune(words[0])
	if len(runes) != 1 || !unicode.IsUpper(runes[0]) {
		return false
	}
	return line.FontSize > bodySize*3.0
}

// mergeBandFlows merges fragments that occupy the same horizontal position within
// a band (co-located columns) into single lines, spacing them proportionally.
// Lines that result from merging multiple fragments are tagged as RoleTable.
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
		if visited[i] {
			continue
		}
		var group []model.Flow
		q := []int{i}
		visited[i] = true
		for len(q) > 0 {
			curr := q[0]
			q = q[1:]
			group = append(group, flows[curr])
			for _, nb := range adj[curr] {
				if !visited[nb] {
					visited[nb] = true
					q = append(q, nb)
				}
			}
		}
		colGroups = append(colGroups, group)
	}

	var result []model.Flow
	for _, group := range colGroups {
		if len(group) == 1 {
			result = append(result, group[0])
			continue
		}
		result = append(result, mergeFlowGroup(group))
	}
	return result
}

// mergeFlowGroup combines multiple overlapping flows into one, joining co-located
// lines with gap-proportional spaces.
func mergeFlowGroup(group []model.Flow) model.Flow {
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
	var curr []model.Line

	flush := func(g []model.Line) {
		if len(g) == 0 {
			return
		}
		sort.Slice(g, func(i, j int) bool { return g[i].XMin < g[j].XMin })

		lXMin := g[0].XMin
		lYMin := g[0].YMin
		lXMax := g[len(g)-1].XMax
		lYMax := g[0].YMax

		var sb strings.Builder
		for i, l := range g {
			if i > 0 {
				gap := l.XMin - g[i-1].XMax
				charWidth := l.FontSize * 0.5
				if charWidth <= 0 {
					charWidth = 3.0
				}
				n := int(math.Round(gap / charWidth))
				if n < 1 {
					n = 1
				}
				if n > 30 {
					n = 30
				}
				sb.WriteString(strings.Repeat(" ", n))
			}
			sb.WriteString(l.Text)
			lYMax = math.Max(lYMax, l.YMax)
		}

		role := g[0].Role
		if len(g) > 1 {
			role = model.RoleTable
		}
		mergedLines = append(mergedLines, model.Line{
			XMin:     lXMin,
			YMin:     lYMin,
			XMax:     lXMax,
			YMax:     lYMax,
			FontSize: g[0].FontSize,
			Role:     role,
			Text:     sb.String(),
		})
	}

	for _, l := range allLines {
		if len(curr) == 0 {
			curr = append(curr, l)
			continue
		}
		if math.Abs(l.YMin-curr[0].YMin) <= 2.0 {
			curr = append(curr, l)
		} else {
			flush(curr)
			curr = []model.Line{l}
		}
	}
	flush(curr)

	var blocks []model.Block
	if len(mergedLines) > 0 {
		blocks = []model.Block{{XMin: fXMin, YMin: fYMin, XMax: fXMax, YMax: fYMax, Lines: mergedLines}}
	}
	return model.Flow{
		XMin:   fXMin,
		YMin:   fYMin,
		XMax:   fXMax,
		YMax:   fYMax,
		Lines:  mergedLines,
		Blocks: blocks,
	}
}

// orderFlows groups flows into horizontal bands sorted top-to-bottom, then within
// each band merges co-located fragments and sorts left-to-right.
func orderFlows(flows []model.Flow) []model.Flow {
	if len(flows) == 0 {
		return flows
	}

	sorted := make([]model.Flow, len(flows))
	copy(sorted, flows)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].YMin < sorted[j].YMin
	})

	type band struct {
		yMax  float64
		flows []model.Flow
	}
	var bands []band

	for _, flow := range sorted {
		if len(bands) == 0 {
			bands = append(bands, band{yMax: flow.YMax, flows: []model.Flow{flow}})
			continue
		}
		last := &bands[len(bands)-1]
		if flow.YMin < last.yMax-1.0 {
			last.flows = append(last.flows, flow)
			if flow.YMax > last.yMax {
				last.yMax = flow.YMax
			}
		} else {
			bands = append(bands, band{yMax: flow.YMax, flows: []model.Flow{flow}})
		}
	}

	var result []model.Flow
	for _, b := range bands {
		merged := mergeBandFlows(b.flows)
		sort.Slice(merged, func(i, j int) bool {
			f1, f2 := merged[i], merged[j]
			if math.Abs(f1.XMin-f2.XMin) > 10.0 {
				return f1.XMin < f2.XMin
			}
			if math.Abs(f1.YMin-f2.YMin) > 1.0 {
				return f1.YMin < f2.YMin
			}
			return f1.XMax < f2.XMax
		})
		result = append(result, merged...)
	}
	return result
}
