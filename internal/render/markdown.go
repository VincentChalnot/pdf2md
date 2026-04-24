package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/user/pdf2md/internal/model"
)

// ToMarkdown converts a processed Document to Markdown.
//
// The document is expected to have already been fully processed by the pipeline
// (PreProcess → Process → PostProcess), meaning:
//   - Flow.IsSidebar is set by the SidebarHandler.
//   - Line.StartsNewParagraph is set by the ReflowHandler.
//   - Line.Role is set by the FontRolesHandler.
//
// If pageSeparator is true, a horizontal rule (---) is inserted between pages.
func ToMarkdown(w io.Writer, doc *model.Document, pageSeparator bool) error {
	if len(doc.Pages) == 0 {
		return nil
	}

	var firstH1 string
	var firstH1Emitted bool
	var output strings.Builder

	for pageIdx, page := range doc.Pages {
		for flowIdx, flow := range page.Flows {
			if len(flow.Lines) == 0 {
				continue
			}

			if flow.IsSidebar {
				output.WriteString("***\n\n")
			}

			renderFlow(&output, flow, &firstH1, &firstH1Emitted)

			if flow.IsSidebar {
				output.WriteString("\n***\n")
			}

			isLastFlow := pageIdx == len(doc.Pages)-1 && flowIdx == len(page.Flows)-1
			if !flow.IsSidebar && !isLastFlow {
				output.WriteString("\n")
			}
		}

		if pageSeparator && pageIdx < len(doc.Pages)-1 {
			output.WriteString("\n---\n\n")
		}
	}

	// Emit the document title (first H1) at the very top.
	if firstH1 != "" {
		if _, err := fmt.Fprintf(w, "# %s\n\n", firstH1); err != nil {
			return err
		}
	}

	_, err := w.Write([]byte(output.String()))
	return err
}

// renderFlow formats a single flow and appends it to output.
func renderFlow(output *strings.Builder, flow model.Flow, firstH1 *string, firstH1Emitted *bool) {
	if len(flow.Lines) == 0 {
		return
	}

	var paragraph strings.Builder
	var lastRole model.FontRole
	inTable := false

	for _, line := range flow.Lines {
		if line.Role == model.RoleExcluded || line.Role == model.RoleUnknown {
			continue
		}

		// Capture the first H1 as the document title (suppress inline rendering).
		if line.Role == model.RoleH1 && *firstH1 == "" {
			*firstH1 = line.Text
		}
		if line.Role == model.RoleH1 && !*firstH1Emitted && line.Text == *firstH1 {
			*firstH1Emitted = true
			lastRole = line.Role
			continue
		}

		// Close an open table block if the role changes away from RoleTable.
		if inTable && line.Role != model.RoleTable {
			output.WriteString("```\n\n")
			inTable = false
		}

		// Flush accumulated paragraph when a new paragraph starts.
		if line.StartsNewParagraph && paragraph.Len() > 0 {
			flushParagraph(output, strings.TrimSpace(paragraph.String()))
			paragraph.Reset()
		}

		switch line.Role {
		case model.RoleTable:
			// Flush any non-table paragraph first.
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			if !inTable {
				output.WriteString("```text\n")
				inTable = true
			}
			output.WriteString(line.Text)
			output.WriteString("\n")

		case model.RoleH1:
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			output.WriteString("# ")
			output.WriteString(line.Text)
			output.WriteString("\n\n")

		case model.RoleH2:
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			output.WriteString("## ")
			output.WriteString(line.Text)
			output.WriteString("\n\n")

		case model.RoleH3:
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			output.WriteString("### ")
			output.WriteString(line.Text)
			output.WriteString("\n\n")

		case model.RoleH4:
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			output.WriteString("#### ")
			output.WriteString(line.Text)
			output.WriteString("\n\n")

		case model.RoleH5:
			if paragraph.Len() > 0 {
				flushParagraph(output, strings.TrimSpace(paragraph.String()))
				paragraph.Reset()
			}
			output.WriteString("##### ")
			output.WriteString(line.Text)
			output.WriteString("\n\n")

		default:
			// Body or small: accumulate into the current paragraph.
			if paragraph.Len() > 0 {
				paragraph.WriteString(" ")
			}
			if line.Role == model.RoleSmall {
				paragraph.WriteString("<small>")
				paragraph.WriteString(line.Text)
				paragraph.WriteString("</small>")
			} else {
				paragraph.WriteString(line.Text)
			}
		}

		lastRole = line.Role
	}

	// Close open table block.
	if inTable {
		output.WriteString("```\n\n")
	}

	// Flush trailing paragraph.
	if paragraph.Len() > 0 && lastRole != model.RoleTable {
		flushParagraph(output, strings.TrimSpace(paragraph.String()))
	}
}

// flushParagraph writes a paragraph to output followed by a blank line.
func flushParagraph(output *strings.Builder, text string) {
	if text == "" {
		return
	}
	output.WriteString(text)
	output.WriteString("\n\n")
}
