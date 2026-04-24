// Package pipeline implements a three-phase, event-based document processing system.
//
// The three events fired in sequence are:
//
//  1. PreProcess  – layout detection, line/block cleanup, splitting and grouping.
//  2. Process     – the reflow step that puts all elements in proper reading order.
//  3. PostProcess – fixes applied to the final output (table formatting, sidebar tagging, …).
//
// Handlers register for one event and declare a numeric priority.
// Within an event, handlers run in ascending priority order (lower number = runs first).
package pipeline

import (
	"sort"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
	"github.com/user/pdf2md/internal/pipeline/post_process"
	"github.com/user/pdf2md/internal/pipeline/pre_process"
	"github.com/user/pdf2md/internal/pipeline/process"
)

// Handler is implemented by every processing step.
type Handler interface {
	// Event returns the pipeline phase this handler belongs to.
	Event() event.Event
	// Priority controls execution order within a phase (lower = earlier).
	Priority() int
	// Run executes the handler against the document.
	Run(doc *model.Document) error
}

// Dispatcher holds all registered handlers and runs them in event/priority order.
type Dispatcher struct {
	handlers []Handler
}

// Register adds a handler to the dispatcher.
func (d *Dispatcher) Register(h Handler) {
	d.handlers = append(d.handlers, h)
}

// Run fires the three events in sequence, executing all handlers registered for
// each event in ascending priority order.
func (d *Dispatcher) Run(doc *model.Document) error {
	for _, ev := range []event.Event{event.PreProcess, event.Process, event.PostProcess} {
		var phase []Handler
		for _, h := range d.handlers {
			if h.Event() == ev {
				phase = append(phase, h)
			}
		}

		sort.SliceStable(phase, func(i, j int) bool {
			return phase[i].Priority() < phase[j].Priority()
		})

		for _, h := range phase {
			if err := h.Run(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

// DefaultDispatcher returns a Dispatcher pre-loaded with all built-in handlers.
//
// PreProcess phase (layout detection, cleanup, grouping):
//
//	priority 10 – Clean          (noise/watermark removal, line deduplication)
//	priority 20 – FontRoles      (font size bucket analysis and role assignment)
//	priority 25 – HeaderFooter   (cross-page header/footer/page-number filtering)
//	priority 30 – ReadingOrder   (column detection, drop-cap merging, band ordering)
//
// Process phase (reflow):
//
//	priority 10 – Reflow         (paragraph boundary annotation)
//
// PostProcess phase (output fixes):
//
//	priority 10 – TableFormat    (designated place for table post-processing)
//	priority 20 – Sidebar        (detect and tag sidebar flows)
//
// The minTextHeight parameter is forwarded to the Clean handler.
func DefaultDispatcher(minTextHeight float64) *Dispatcher {
	d := &Dispatcher{}

	// PreProcess
	d.Register(pre_process.NewCleanHandler(minTextHeight))
	d.Register(pre_process.NewFontRolesHandler())
	d.Register(pre_process.NewHeaderFooterHandler())
	d.Register(pre_process.NewReadingOrderHandler())

	// Process
	d.Register(process.NewReflowHandler())

	// PostProcess
	d.Register(post_process.NewTableFormatHandler())
	d.Register(post_process.NewSidebarHandler())

	return d
}
