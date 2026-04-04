package model

// Document represents a parsed PDF document.
type Document struct {
	Source  string              `json:"source"`
	Meta    map[string]string   `json:"meta,omitempty"`
	FontMap map[string]FontSpec `json:"font_map"`
	Pages   []Page              `json:"pages"`
}

// FontSpec describes a font size bucket used in the document.
type FontSpec struct {
	Size    float64  `json:"size"`
	Role    FontRole `json:"role"`
	NbChars int      `json:"nb_chars,omitempty"`
	NbLines int      `json:"nb_lines,omitempty"`
}

// FontRole represents the semantic role assigned to a font.
type FontRole string

const (
	RoleH1       FontRole = "h1"
	RoleH2       FontRole = "h2"
	RoleH3       FontRole = "h3"
	RoleBody     FontRole = "body"
	RoleSmall    FontRole = "small"
	RoleExcluded FontRole = "excluded"
	RoleUnknown  FontRole = "unknown"
)

// Page represents a single page in the document.
type Page struct {
	Number int     `json:"number"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Flows  []Flow  `json:"flows"`
}

// Flow represents a grouped collection of text blocks.
type Flow struct {
	XMin   float64 `json:"xMin"`
	YMin   float64 `json:"yMin"`
	XMax   float64 `json:"xMax"`
	YMax   float64 `json:"yMax"`
	Blocks []Block `json:"blocks"`
	Lines  []Line  `json:"lines"` // Deprecated: kept for backward compatibility, use Blocks instead
}

// Block represents a text block from the bbox layout.
type Block struct {
	XMin  float64 `json:"xMin"`
	YMin  float64 `json:"yMin"`
	XMax  float64 `json:"xMax"`
	YMax  float64 `json:"yMax"`
	Lines []Line  `json:"lines"`
}

// Line represents a visual line of text.
type Line struct {
	XMin     float64  `json:"xMin"`
	YMin     float64  `json:"yMin"`
	XMax     float64  `json:"xMax"`
	YMax     float64  `json:"yMax"`
	FontSize float64  `json:"fontSize"`
	Role     FontRole `json:"role,omitempty"`
	Text     string   `json:"text"`
}

// Word represents a word parsed from bbox-layout (used during processing, not in final model).
type Word struct {
	XMin float64
	YMin float64
	XMax float64
	YMax float64
	Text string
}
