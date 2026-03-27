package extract

import (
	"sort"

	"github.com/user/pdf2md/internal/model"
)

// AssignFontRoles computes NbChars and NbElems for each font,
// then assigns roles based on the heading-map algorithm.
func AssignFontRoles(doc *model.Document, excludeFonts map[string]bool) {
	// Initialize all fonts to unknown, then mark excluded.
	for id, fs := range doc.FontMap {
		if excludeFonts[id] {
			fs.Role = model.RoleExcluded
		} else {
			fs.Role = model.RoleUnknown
		}
		doc.FontMap[id] = fs
	}

	// 1. Count chars and elements per font across all pages.
	charCounts := make(map[string]int)
	elemCounts := make(map[string]int)
	for _, page := range doc.Pages {
		for _, el := range page.Elements {
			charCounts[el.FontID] += len(el.Text)
			elemCounts[el.FontID]++
		}
	}

	// Update FontMap with counts.
	for id, fs := range doc.FontMap {
		fs.NbChars = charCounts[id]
		fs.NbElems = elemCounts[id]
		doc.FontMap[id] = fs
	}

	// 2. Find body font: non-excluded font with highest NbChars.
	var bodyID string
	var bodyChars int
	for id, fs := range doc.FontMap {
		if fs.Role == model.RoleExcluded {
			continue
		}
		if fs.NbChars > bodyChars {
			bodyChars = fs.NbChars
			bodyID = id
		}
	}

	if bodyID == "" {
		// No body font found — mark everything as unknown.
		for id, fs := range doc.FontMap {
			if fs.Role != model.RoleExcluded {
				fs.Role = model.RoleUnknown
				doc.FontMap[id] = fs
			}
		}
		return
	}

	bodySize := doc.FontMap[bodyID].Size

	// 3. Collect fonts larger than body, sorted by size DESC.
	type fontEntry struct {
		id   string
		size float64
	}
	var larger []fontEntry
	for id, fs := range doc.FontMap {
		if fs.Role == model.RoleExcluded {
			continue
		}
		if id == bodyID {
			continue
		}
		if fs.Size > bodySize {
			larger = append(larger, fontEntry{id: id, size: fs.Size})
		}
	}
	sort.Slice(larger, func(i, j int) bool {
		return larger[i].size > larger[j].size
	})

	// 4. Assign heading roles: h1, h2, h3 (cap at h3).
	headingRoles := []model.FontRole{model.RoleH1, model.RoleH2, model.RoleH3}
	for i, fe := range larger {
		fs := doc.FontMap[fe.id]
		if i < len(headingRoles) {
			fs.Role = headingRoles[i]
		} else {
			fs.Role = model.RoleH3
		}
		doc.FontMap[fe.id] = fs
	}

	// 5. Assign body.
	{
		fs := doc.FontMap[bodyID]
		fs.Role = model.RoleBody
		doc.FontMap[bodyID] = fs
	}

	// 6. Assign small and unknown for remaining fonts.
	for id, fs := range doc.FontMap {
		if fs.Role != model.RoleUnknown {
			continue
		}
		if fs.NbChars == 0 {
			fs.Role = model.RoleUnknown
		} else if fs.Size < bodySize {
			fs.Role = model.RoleSmall
		} else {
			// Same size as body but not the body font itself.
			fs.Role = model.RoleBody
		}
		doc.FontMap[id] = fs
	}
}

// ApplyRolesToElements sets the Role field on each element based on its font's role.
func ApplyRolesToElements(doc *model.Document) {
	for i := range doc.Pages {
		for j := range doc.Pages[i].Elements {
			el := &doc.Pages[i].Elements[j]
			if fs, ok := doc.FontMap[el.FontID]; ok {
				el.Role = fs.Role
			} else {
				el.Role = model.RoleUnknown
			}
		}
	}
}

// BuildHeadingsTOC reconstructs a TOC from elements with heading roles.
func BuildHeadingsTOC(doc *model.Document) []model.OutlineItem {
	var items []model.OutlineItem
	for _, page := range doc.Pages {
		for _, el := range page.Elements {
			if el.Role == model.RoleH1 || el.Role == model.RoleH2 {
				items = append(items, model.OutlineItem{
					Title: el.Text,
					Page:  page.Number,
				})
			}
		}
	}
	return items
}

// ResolveTOC picks the appropriate TOC source based on the --toc-source flag.
func ResolveTOC(doc *model.Document, tocSource string) {
	switch tocSource {
	case "outline":
		// Use the parsed outline as-is.
		return
	case "headings":
		doc.Outline = BuildHeadingsTOC(doc)
	default: // "auto"
		if len(doc.Outline) >= 2 {
			// outline has enough items, use it.
			return
		}
		doc.Outline = BuildHeadingsTOC(doc)
	}
}
