// Package event defines the pipeline event types used by all handlers.
package event

// Event identifies which pipeline phase a handler listens to.
type Event int

const (
	// PreProcess covers layout detection, line/block cleanup, splitting and grouping.
	PreProcess Event = 1
	// Process is the reflow step that puts all elements in proper reading order.
	Process Event = 2
	// PostProcess covers fixes to the final output (table formatting, sidebar tagging, …).
	PostProcess Event = 3
)
