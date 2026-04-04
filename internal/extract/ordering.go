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
		columnGroups := identifyColumnGroups(page.Flows)

		// Step 2: Drop cap detection and merging.
		page.Flows = mergeDropCaps(page.Flows, columnGroups, bodySize)

		// Step 3: Re-identify parallel column groups since indices may have changed.
		columnGroups = identifyColumnGroups(page.Flows)

		// Step 4: Final ordering.
		page.Flows = orderFlows(page.Flows, columnGroups)
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
	flows []int // indices into page.Flows
}

// identifyColumnGroups finds flows that are parallel columns.
// Two flows are parallel if their Y ranges overlap by >40% of the shorter flow's height
// AND their X ranges do not overlap.
func identifyColumnGroups(flows []model.Flow) []columnGroup {
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

		groups = append(groups, columnGroup{flows: group})
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
		for pi, fi := range g.flows {
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
			if pos+1 < len(groups[gi].flows) {
				nextIdx := groups[gi].flows[pos+1]

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

// orderFlows orders flows by yMin, with column groups ordered left-to-right.
func orderFlows(flows []model.Flow, groups []columnGroup) []model.Flow {
	if len(flows) == 0 {
		return flows
	}

	// Build a map of flow to its order key.
	type orderKey struct {
		yMin    float64
		colIdx  int
		flowIdx int
	}

	flowToKey := make(map[int]orderKey)

	// Assign order keys to flows in column groups.
	for _, g := range groups {
		for colIdx, flowIdx := range g.flows {
			flowToKey[flowIdx] = orderKey{
				yMin:    flows[flowIdx].YMin,
				colIdx:  colIdx,
				flowIdx: flowIdx,
			}
		}
	}

	// Flows not in any group get a simple yMin-based key.
	for i := range flows {
		if _, exists := flowToKey[i]; !exists {
			flowToKey[i] = orderKey{
				yMin:    flows[i].YMin,
				colIdx:  0,
				flowIdx: i,
			}
		}
	}

	// Sort flows by yMin, then by colIdx, then by original index.
	type flowWithKey struct {
		flow model.Flow
		key  orderKey
	}

	var sortable []flowWithKey
	for i, flow := range flows {
		sortable = append(sortable, flowWithKey{
			flow: flow,
			key:  flowToKey[i],
		})
	}

	sort.Slice(sortable, func(i, j int) bool {
		ki, kj := sortable[i].key, sortable[j].key

		// Primary: yMin
		if math.Abs(ki.yMin-kj.yMin) > 1.0 {
			return ki.yMin < kj.yMin
		}

		// Secondary: colIdx (left to right)
		if ki.colIdx != kj.colIdx {
			return ki.colIdx < kj.colIdx
		}

		// Tertiary: original index
		return ki.flowIdx < kj.flowIdx
	})

	var result []model.Flow
	for _, item := range sortable {
		result = append(result, item.flow)
	}

	return result
}
