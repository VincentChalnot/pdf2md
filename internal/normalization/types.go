package normalization

import (
	"github.com/user/pdf2md/internal/model"
)

// LineType classifies a VirtualLine's alignment or structural role.
type LineType int

const (
	LineUnclassified LineType = iota
	LineLeft                  // left-aligned text
	LineRight                 // right-aligned text
	LineJustified             // justified text
	LineStructural            // shares Y-band with other lines in the layout zone
)

// String returns a human-readable name for LineType.
func (lt LineType) String() string {
	switch lt {
	case LineUnclassified:
		return "UNCLASSIFIED"
	case LineLeft:
		return "LEFT"
	case LineRight:
		return "RIGHT"
	case LineJustified:
		return "JUSTIFIED"
	case LineStructural:
		return "STRUCTURAL"
	default:
		return "UNKNOWN"
	}
}

// GapType classifies an inter-word gap.
type GapType int

const (
	GapUnclassified  GapType = iota
	GapColumnType            // separates text columns
	GapTableType             // separates table cells
	GapIsolated              // not vertically confirmed, likely justified text
)

// String returns a human-readable name for GapType.
func (gt GapType) String() string {
	switch gt {
	case GapUnclassified:
		return "UNCLASSIFIED"
	case GapColumnType:
		return "COLUMN"
	case GapTableType:
		return "TABLE"
	case GapIsolated:
		return "ISOLATED"
	default:
		return "UNKNOWN"
	}
}

// BlockType classifies a LogicalBlock.
type BlockType int

const (
	BlockText       BlockType = iota // all lines are text family
	BlockStructural                  // any line is LINE_STRUCTURAL
)

// String returns a human-readable name for BlockType.
func (bt BlockType) String() string {
	switch bt {
	case BlockText:
		return "TEXT"
	case BlockStructural:
		return "STRUCTURAL"
	default:
		return "UNKNOWN"
	}
}

// VirtualLine replaces a Poppler <line>. May be reconstructed from word clusters.
type VirtualLine struct {
	Words          []model.Word
	XMin, YMin     float64
	XMax, YMax     float64
	FontSize       float64
	Role           model.FontRole
	Text           string
	Classification LineType
	GapCandidates  []*GapCandidate
	SourceLineRef  *model.Line // original Poppler line (nil if synthesized)
	SourceBlock    *model.Block
	Virtual        bool

	// Debug info for SVG rendering
	EmptyLeft  float64
	EmptyRight float64
	Coverage   float64
	GapSigma   float64
}

// GapCandidate is a large inter-word gap within a VirtualLine, candidate for a structural cut.
type GapCandidate struct {
	XLeft, XRight float64  // exact gap boundaries
	XCenter       float64  // center of the gap
	YLine         float64  // yCenter of the parent line
	Width         float64  // gap width
	Type          GapType  // classification
	VerticalGroup *GapColumn // nil until Pass B assigns it
	ParentLine    *VirtualLine
}

// GapColumn is a vertical group of GapCandidates aligned on the X axis.
type GapColumn struct {
	XCenter float64
	Gaps    []*GapCandidate
	Type    GapType
}

// LogicalBlock is the output unit consumed by Steps 3–6.
type LogicalBlock struct {
	Lines        []VirtualLine
	XMin, YMin   float64
	XMax, YMax   float64
	Type         BlockType
	SourceBlocks []*model.Block // Poppler blocks this was derived from (weak signal, for debug)
	Virtual      bool
}

// DebugData holds normalization results for SVG debug rendering.
type DebugData struct {
	VirtualLines  []*VirtualLine
	GapCandidates []*GapCandidate
	GapColumns    []*GapColumn
	LogicalBlocks []LogicalBlock
}

// Options controls normalization behavior.
type Options struct {
	LineSplitRatio float64 // override for LineSplitRatio constant (0 = use default)
}
