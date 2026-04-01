package extract

import (
	"regexp"

	"github.com/user/pdf2md/internal/model"
)

// Clean applies the cleaning pipeline to the document:
// 1. Filter flows by minimum text height
// 2. Filter watermark flows (edge-placed, single-line numeric flows)
// 3. Dedup lines by (yMin, xMin, text) within the same page
func Clean(doc *model.Document, minTextHeight float64) {
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	for i := range doc.Pages {
		page := &doc.Pages[i]

		// 1. Filter flows where ALL words have height < minTextHeight.
		var filteredFlows []model.Flow
		for _, flow := range page.Flows {
			keepFlow := false
			for _, line := range flow.Lines {
				// If any line has fontSize >= minTextHeight, keep the flow.
				if line.FontSize >= minTextHeight {
					keepFlow = true
					break
				}
			}
			if keepFlow {
				filteredFlows = append(filteredFlows, flow)
			}
		}
		page.Flows = filteredFlows

		// 2. Filter watermark flows.
		var nonWatermarkFlows []model.Flow
		for _, flow := range page.Flows {
			isWatermark := false

			// Check if flow has exactly 1 line.
			if len(flow.Lines) == 1 {
				line := flow.Lines[0]

				// Check if flow bbox is within 12px of any page edge.
				nearEdge := flow.XMin < 12 || flow.XMax > page.Width-12 ||
					flow.YMin < 12 || flow.YMax > page.Height-12

				if nearEdge {
					// Check if text is purely numeric OR contains both "paizo.com" and an email.
					text := line.Text
					purelyNumeric := isPurelyNumeric(text)
					containsPaizoAndEmail := containsSubstring(text, "paizo.com") && emailPattern.MatchString(text)

					if purelyNumeric || containsPaizoAndEmail {
						isWatermark = true
					}
				}
			}

			if !isWatermark {
				nonWatermarkFlows = append(nonWatermarkFlows, flow)
			}
		}
		page.Flows = nonWatermarkFlows

		// 3. Dedup lines within each flow by (yMin, xMin, text).
		for fi := range page.Flows {
			flow := &page.Flows[fi]
			seen := make(map[dedupKey]bool)
			var dedupedLines []model.Line
			for _, line := range flow.Lines {
				key := dedupKey{yMin: line.YMin, xMin: line.XMin, text: line.Text}
				if !seen[key] {
					seen[key] = true
					dedupedLines = append(dedupedLines, line)
				}
			}
			flow.Lines = dedupedLines
		}
	}
}

type dedupKey struct {
	yMin float64
	xMin float64
	text string
}

// isPurelyNumeric checks if a string contains only digits, spaces, and punctuation.
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

// containsSubstring checks if a string contains a substring (case-insensitive).
func containsSubstring(s, substr string) bool {
	return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(substr)).MatchString(s)
}
