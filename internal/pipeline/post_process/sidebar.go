package post_process

import (
	"strings"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// SidebarHandler detects flows that look like sidebars and sets Flow.IsSidebar = true.
//
// A flow is considered a sidebar when ALL of the following are true:
//   - Its width is less than 70% of the page width.
//   - Its left edge starts beyond 40% of the page width (positioned on the right side).
//   - Its height is less than 70% of the page height.
//   - Its first non-empty line carries a heading role (h1–h5).
//
// Priority: 20 (runs after TableFormat in PostProcess).
type SidebarHandler struct{}

// NewSidebarHandler returns a SidebarHandler.
func NewSidebarHandler() *SidebarHandler { return &SidebarHandler{} }

func (h *SidebarHandler) Event() event.Event { return event.PostProcess }
func (h *SidebarHandler) Priority() int      { return 20 }

func (h *SidebarHandler) Run(doc *model.Document) error {
	for pi := range doc.Pages {
		page := &doc.Pages[pi]
		for fi := range page.Flows {
			flow := &page.Flows[fi]
			flow.IsSidebar = isSidebar(flow, page)
		}
	}
	return nil
}

// isSidebar returns true when the flow matches sidebar geometry and content heuristics.
func isSidebar(flow *model.Flow, page *model.Page) bool {
	flowWidth := flow.XMax - flow.XMin
	if flowWidth >= page.Width*0.7 {
		return false
	}
	if flow.XMin <= page.Width*0.4 {
		return false
	}
	flowHeight := flow.YMax - flow.YMin
	if flowHeight >= page.Height*0.7 {
		return false
	}
	// First non-empty line must be a heading.
	for _, line := range flow.Lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		return isHeadingRole(line.Role)
	}
	return false
}

func isHeadingRole(r model.FontRole) bool {
	return r == model.RoleH1 || r == model.RoleH2 || r == model.RoleH3 ||
		r == model.RoleH4 || r == model.RoleH5
}
