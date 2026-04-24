// Package pre_process contains handlers for the PreProcess pipeline event.
//
// The PreProcess phase covers everything related to layout detection,
// line/block cleanup, splitting and grouping.
package pre_process

import (
	"regexp"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// CleanHandler removes noise from the parsed document:
//  1. Flows where every line is below the minimum text height are dropped.
//  2. Single-line flows near page edges that look like watermarks or page numbers
//     are dropped.
//  3. Duplicate lines (same yMin, xMin, text) within a flow are removed.
//
// Priority: 10 (runs first in PreProcess).
type CleanHandler struct {
	minTextHeight float64
}

// NewCleanHandler returns a CleanHandler configured with the given minimum word height.
func NewCleanHandler(minTextHeight float64) *CleanHandler {
	return &CleanHandler{minTextHeight: minTextHeight}
}

func (h *CleanHandler) Event() event.Event { return event.PreProcess }
func (h *CleanHandler) Priority() int      { return 10 }

func (h *CleanHandler) Run(doc *model.Document) error {
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	for i := range doc.Pages {
		page := &doc.Pages[i]

		// 1. Filter flows where ALL lines have fontSize < minTextHeight.
		var sizeFiltered []model.Flow
		for _, flow := range page.Flows {
			keep := false
			for _, line := range flow.Lines {
				if line.FontSize >= h.minTextHeight {
					keep = true
					break
				}
			}
			if keep {
				sizeFiltered = append(sizeFiltered, flow)
			}
		}
		page.Flows = sizeFiltered

		// 2. Filter watermark flows.
		var nonWatermark []model.Flow
		for _, flow := range page.Flows {
			if isWatermark(flow, page, emailPattern) {
				continue
			}
			nonWatermark = append(nonWatermark, flow)
		}
		page.Flows = nonWatermark

		// 3. Deduplicate lines within each flow by (yMin, xMin, text).
		for fi := range page.Flows {
			flow := &page.Flows[fi]
			seen := make(map[dedupKey]bool)
			var deduped []model.Line
			for _, line := range flow.Lines {
				k := dedupKey{yMin: line.YMin, xMin: line.XMin, text: line.Text}
				if !seen[k] {
					seen[k] = true
					deduped = append(deduped, line)
				}
			}
			flow.Lines = deduped
		}
	}
	return nil
}

type dedupKey struct {
	yMin float64
	xMin float64
	text string
}

// isWatermark returns true when a flow looks like a page number or publisher watermark.
func isWatermark(flow model.Flow, page *model.Page, emailPattern *regexp.Regexp) bool {
	if len(flow.Lines) != 1 {
		return false
	}
	line := flow.Lines[0]

	nearEdge := flow.XMin < 12 || flow.XMax > page.Width-12 ||
		flow.YMin < 12 || flow.YMax > page.Height-12
	if !nearEdge {
		return false
	}

	text := line.Text
	if isPurelyNumeric(text) {
		return true
	}
	if containsSubstring(text, "paizo.com") && emailPattern.MatchString(text) {
		return true
	}
	return false
}

func isPurelyNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r == ' ' || r == '.' || r == ',' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func containsSubstring(s, substr string) bool {
	return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(substr)).MatchString(s)
}
