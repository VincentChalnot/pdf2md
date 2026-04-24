// Package post_process contains handlers for the PostProcess pipeline event.
//
// The PostProcess phase applies fixes to the final document representation
// before it is handed off to the renderer (e.g. table formatting, sidebar tagging).
package post_process

import (
	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// TableFormatHandler tags contiguous runs of RoleTable lines so that the renderer
// can wrap them in fenced code blocks without needing to detect the run boundaries
// itself.  Currently it performs a no-op walk — the RoleTable role is already set
// by the ReadingOrderHandler — but it serves as the designated place for any future
// table-specific post-processing (column alignment, ASCII borders, etc.).
//
// Priority: 10 (runs first in PostProcess).
type TableFormatHandler struct{}

// NewTableFormatHandler returns a TableFormatHandler.
func NewTableFormatHandler() *TableFormatHandler { return &TableFormatHandler{} }

func (h *TableFormatHandler) Event() event.Event { return event.PostProcess }
func (h *TableFormatHandler) Priority() int      { return 10 }

func (h *TableFormatHandler) Run(doc *model.Document) error {
	// RoleTable is already assigned by the ReadingOrderHandler (mergeBandFlows).
	// This handler is the designated location for any future table post-processing,
	// such as column-width normalisation or ASCII border drawing.
	//
	// Walk every flow to verify consistency (no-op for now).
	for pi := range doc.Pages {
		for fi := range doc.Pages[pi].Flows {
			_ = &doc.Pages[pi].Flows[fi]
		}
	}
	return nil
}
