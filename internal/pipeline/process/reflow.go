// Package process contains handlers for the Process pipeline event.
//
// The Process phase is the reflow step: it puts all elements in the proper
// reading order and annotates lines with paragraph-boundary information so that
// the renderer can produce clean, well-structured output without needing to
// re-interpret layout geometry.
package process

import (
	"sort"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// ReflowHandler analyses each flow and marks the first line of every new
// paragraph with StartsNewParagraph = true.
//
// A paragraph break is inserted when:
//   - The current line is a heading (h1–h5), or
//   - The previous line was a heading, or
//   - The vertical gap between the previous line's bottom and the current
//     line's top exceeds 1.5× the median body line height in that flow.
//
// Priority: 10 (only handler in the Process phase).
type ReflowHandler struct{}

// NewReflowHandler returns a ReflowHandler.
func NewReflowHandler() *ReflowHandler { return &ReflowHandler{} }

func (h *ReflowHandler) Event() event.Event { return event.Process }
func (h *ReflowHandler) Priority() int      { return 10 }

func (h *ReflowHandler) Run(doc *model.Document) error {
	for pi := range doc.Pages {
		for fi := range doc.Pages[pi].Flows {
			reflowFlow(&doc.Pages[pi].Flows[fi])
		}
	}
	return nil
}

// reflowFlow annotates lines in a single flow with StartsNewParagraph.
func reflowFlow(flow *model.Flow) {
	if len(flow.Lines) == 0 {
		return
	}

	bodyLineHeight := medianBodyLineHeight(flow)

	var lastRole model.FontRole
	var lastYMax float64
	firstVisible := true

	for i := range flow.Lines {
		line := &flow.Lines[i]

		// Skip excluded/unknown lines — they don't participate in paragraph logic.
		if line.Role == model.RoleExcluded || line.Role == model.RoleUnknown {
			continue
		}

		if firstVisible {
			line.StartsNewParagraph = true
			firstVisible = false
			lastRole = line.Role
			lastYMax = line.YMax
			continue
		}

		starts := false
		if isHeading(line.Role) || isHeading(lastRole) {
			starts = true
		} else if bodyLineHeight > 0 {
			gap := line.YMin - lastYMax
			if gap > bodyLineHeight*1.5 {
				starts = true
			}
		}

		line.StartsNewParagraph = starts
		lastRole = line.Role
		lastYMax = line.YMax
	}
}

func isHeading(r model.FontRole) bool {
	return r == model.RoleH1 || r == model.RoleH2 || r == model.RoleH3 ||
		r == model.RoleH4 || r == model.RoleH5
}

// medianBodyLineHeight returns the median height of body lines in a flow.
func medianBodyLineHeight(flow *model.Flow) float64 {
	var heights []float64
	for _, line := range flow.Lines {
		if line.Role == model.RoleBody {
			heights = append(heights, line.YMax-line.YMin)
		}
	}
	if len(heights) == 0 {
		return 0
	}
	sort.Float64s(heights)
	mid := len(heights) / 2
	if len(heights)%2 == 0 && mid > 0 {
		return (heights[mid-1] + heights[mid]) / 2
	}
	return heights[mid]
}
