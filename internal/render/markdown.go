package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/user/pdf2md/internal/model"
)

// ToMarkdown converts a parsed Document to Markdown format.
// This is a deterministic conversion based on the JSON structure with no LLM involved.
func ToMarkdown(w io.Writer, doc *model.Document) error {
	if len(doc.Pages) == 0 {
		return nil
	}

	var firstH1 string
	var firstH1Used bool
	var output strings.Builder

	// Process all pages
	for pageIdx, page := range doc.Pages {
		// Process flows in order (already sorted by extract pipeline)
		for flowIdx, flow := range page.Flows {
			if len(flow.Lines) == 0 {
				continue
			}

			// Check if this flow is a sidebar
			isSidebar := isFlowSidebar(flow, page)

			if isSidebar {
				output.WriteString("***\n\n")
			}

			// Render the flow
			renderFlow(&output, flow, page, &firstH1, &firstH1Used)

			// Add separator after sidebar
			if isSidebar {
				output.WriteString("\n***\n")
			}

			// Add blank line between flows (but not after last flow on last page)
			if !isSidebar && !(pageIdx == len(doc.Pages)-1 && flowIdx == len(page.Flows)-1) {
				output.WriteString("\n")
			}
		}
	}

	// Write the document title if we found an h1
	if firstH1 != "" {
		if _, err := fmt.Fprintf(w, "# %s\n\n", firstH1); err != nil {
			return err
		}
	}

	// Write the rest of the document
	_, err := w.Write([]byte(output.String()))
	return err
}

// isFlowSidebar determines if a flow should be rendered as a sidebar.
func isFlowSidebar(flow model.Flow, page model.Page) bool {
	// Check width constraint: flow width < 70% of page width
	flowWidth := flow.XMax - flow.XMin
	if flowWidth >= page.Width*0.7 {
		return false
	}

	// Check position constraint: flow starts in right half (> 40% from left)
	if flow.XMin <= page.Width*0.4 {
		return false
	}

	// Check height constraint: flow height < 70% of page height
	flowHeight := flow.YMax - flow.YMin
	if flowHeight >= page.Height*0.7 {
		return false
	}

	// Check if first non-empty line is a heading
	for _, line := range flow.Lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if line.Role == model.RoleH1 || line.Role == model.RoleH2 || line.Role == model.RoleH3 {
			return true
		}
		// First non-empty line is not a heading
		return false
	}

	return false
}

// renderFlow processes a single flow and appends to the output.
func renderFlow(output *strings.Builder, flow model.Flow, page model.Page, firstH1 *string, firstH1Used *bool) {
	if len(flow.Lines) == 0 {
		return
	}

	// Calculate median body line height for paragraph merging
	bodyLineHeight := calculateMedianBodyLineHeight(flow)

	var paragraph strings.Builder
	var lastRole model.FontRole
	var lastYMax float64

	for i, line := range flow.Lines {
		// Skip excluded and unknown lines
		if line.Role == model.RoleExcluded || line.Role == model.RoleUnknown {
			continue
		}

		text := line.Text
		role := line.Role

		// Capture first h1 for document title (and skip rendering it in the flow)
		if role == model.RoleH1 && *firstH1 == "" {
			*firstH1 = text
			*firstH1Used = false
		}

		// Skip rendering the first h1 (it will be the document title)
		if role == model.RoleH1 && !*firstH1Used && text == *firstH1 {
			*firstH1Used = true
			lastRole = role
			lastYMax = line.YMax
			continue
		}

		// Check if we need to end the current paragraph
		shouldEndParagraph := false
		if i > 0 {
			// Headings always end the current paragraph
			if role == model.RoleH1 || role == model.RoleH2 || role == model.RoleH3 {
				shouldEndParagraph = true
			} else if lastRole == model.RoleH1 || lastRole == model.RoleH2 || lastRole == model.RoleH3 {
				// Previous line was a heading, start new paragraph
				shouldEndParagraph = true
			} else {
				// Calculate line gap
				lineGap := line.YMin - lastYMax
				if bodyLineHeight > 0 && lineGap > bodyLineHeight*1.5 {
					shouldEndParagraph = true
				}
			}
		}

		// Flush the current paragraph if needed
		if shouldEndParagraph && paragraph.Len() > 0 {
			flushParagraph(output, strings.TrimSpace(paragraph.String()), lastRole)
			paragraph.Reset()
		}

		// Handle heading lines
		if role == model.RoleH1 {
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()), lastRole)
				paragraph.Reset()
			}
			output.WriteString("# ")
			output.WriteString(text)
			output.WriteString("\n\n")
		} else if role == model.RoleH2 {
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()), lastRole)
				paragraph.Reset()
			}
			output.WriteString("## ")
			output.WriteString(text)
			output.WriteString("\n\n")
		} else if role == model.RoleH3 {
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()), lastRole)
				paragraph.Reset()
			}
			output.WriteString("### ")
			output.WriteString(text)
			output.WriteString("\n\n")
		} else {
			// Body or small line - add to paragraph
			if paragraph.Len() > 0 {
				paragraph.WriteString(" ")
			}

			if role == model.RoleSmall {
				paragraph.WriteString("<small>")
				paragraph.WriteString(text)
				paragraph.WriteString("</small>")
			} else {
				paragraph.WriteString(text)
			}
		}

		lastRole = role
		lastYMax = line.YMax
	}

	// Flush any remaining paragraph
	if paragraph.Len() > 0 {
		flushParagraph(output, strings.TrimSpace(paragraph.String()), lastRole)
	}
}

// flushParagraph writes the accumulated paragraph text to output.
func flushParagraph(output *strings.Builder, text string, role model.FontRole) {
	if text == "" {
		return
	}
	output.WriteString(text)
	output.WriteString("\n\n")
}

// calculateMedianBodyLineHeight computes the median line height for body text in a flow.
func calculateMedianBodyLineHeight(flow model.Flow) float64 {
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
