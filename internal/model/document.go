package model

// Document represents a parsed PDF document.
type Document struct {
	Source  string              `json:"source"`
	FontMap map[string]FontSpec `json:"font_map"`
	Outline []OutlineItem       `json:"outline,omitempty"`
	Pages   []Page              `json:"pages"`
}

// FontSpec describes a font used in the document.
type FontSpec struct {
	ID      string   `json:"id"`
	Size    float64  `json:"size"`
	Family  string   `json:"family"`
	Color   string   `json:"color"`
	Role    FontRole `json:"role"`
	NbChars int      `json:"nb_chars,omitempty"`
	NbElems int      `json:"nb_elems,omitempty"`
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
	Number   int       `json:"number"`
	Width    float64   `json:"width"`
	Height   float64   `json:"height"`
	Elements []Element `json:"elements"`
}

// Element represents a text element on a page.
type Element struct {
	Top    float64  `json:"top"`
	Left   float64  `json:"left"`
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
	FontID string   `json:"font_id"`
	Role   FontRole `json:"role,omitempty"`
	Text   string   `json:"text"`
}

// OutlineItem represents an entry in the document's table of contents.
type OutlineItem struct {
	Title    string        `json:"title"`
	Page     int           `json:"page"`
	Children []OutlineItem `json:"children,omitempty"`
}
