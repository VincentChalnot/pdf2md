package pre_process

import (
	"fmt"
	"math"
	"sort"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// FontRolesHandler performs two sub-steps:
//  1. AssignFontRoles: aggregates line font sizes into size buckets, identifies the
//     body font (highest character count), and maps every bucket to a FontRole
//     (h1–h5, body, small, unknown).
//  2. ApplyRolesToLines: stamps each line's Role field from the computed FontMap.
//
// Priority: 20 (runs after Clean in PreProcess).
type FontRolesHandler struct{}

// NewFontRolesHandler returns a FontRolesHandler.
func NewFontRolesHandler() *FontRolesHandler { return &FontRolesHandler{} }

func (h *FontRolesHandler) Event() event.Event { return event.PreProcess }
func (h *FontRolesHandler) Priority() int      { return 20 }

func (h *FontRolesHandler) Run(doc *model.Document) error {
	assignFontRoles(doc)
	applyRolesToLines(doc)
	return nil
}

// assignFontRoles computes font size buckets and populates doc.FontMap.
func assignFontRoles(doc *model.Document) {
	type sizeStats struct {
		size    float64
		nbChars int
		nbLines int
	}

	statsMap := make(map[string]*sizeStats)

	for _, page := range doc.Pages {
		for _, flow := range page.Flows {
			for _, line := range flow.Lines {
				if line.FontSize <= 0 {
					continue
				}
				bucket := math.Round(line.FontSize*2) / 2
				key := fmt.Sprintf("%.1f", bucket)
				if _, exists := statsMap[key]; !exists {
					statsMap[key] = &sizeStats{size: bucket}
				}
				statsMap[key].nbChars += len(line.Text)
				statsMap[key].nbLines++
			}
		}
	}

	// Find body font: bucket with the most total characters.
	var bodyKey string
	var bodyChars int
	for key, stats := range statsMap {
		if stats.nbChars > bodyChars {
			bodyChars = stats.nbChars
			bodyKey = key
		}
	}

	if bodyKey == "" {
		doc.FontMap = make(map[string]model.FontSpec)
		return
	}

	bodySize := statsMap[bodyKey].size

	// Collect buckets larger than body × 1.2, sorted DESC.
	type fontEntry struct {
		key  string
		size float64
	}
	var larger []fontEntry
	for key, stats := range statsMap {
		if stats.size > bodySize*1.2 {
			larger = append(larger, fontEntry{key: key, size: stats.size})
		}
	}
	sort.Slice(larger, func(i, j int) bool { return larger[i].size > larger[j].size })

	fontMap := make(map[string]model.FontSpec)

	// Map heading sizes to h1–h5 by evenly splitting the range.
	if len(larger) > 0 {
		maxSize := larger[0].size
		minHeadingSize := bodySize * 1.2
		span := maxSize - minHeadingSize

		for _, fe := range larger {
			role := model.RoleH1
			if span > 0 {
				pos := (fe.size - minHeadingSize) / span
				level := 5 - int(math.Ceil(pos*5))
				if level < 1 {
					level = 1
				} else if level > 5 {
					level = 5
				}
				switch level {
				case 1:
					role = model.RoleH1
				case 2:
					role = model.RoleH2
				case 3:
					role = model.RoleH3
				case 4:
					role = model.RoleH4
				case 5:
					role = model.RoleH5
				}
			}
			fontMap[fe.key] = model.FontSpec{
				Size:    statsMap[fe.key].size,
				Role:    role,
				NbChars: statsMap[fe.key].nbChars,
				NbLines: statsMap[fe.key].nbLines,
			}
		}
	}

	// Body bucket.
	fontMap[bodyKey] = model.FontSpec{
		Size:    bodySize,
		Role:    model.RoleBody,
		NbChars: statsMap[bodyKey].nbChars,
		NbLines: statsMap[bodyKey].nbLines,
	}

	// Remaining buckets: small or body.
	for key, stats := range statsMap {
		if _, exists := fontMap[key]; exists {
			continue
		}
		role := model.RoleUnknown
		if stats.size < bodySize*0.85 {
			role = model.RoleSmall
		} else if stats.size >= bodySize*0.85 && stats.size <= bodySize*1.2 {
			role = model.RoleBody
		}
		fontMap[key] = model.FontSpec{
			Size:    stats.size,
			Role:    role,
			NbChars: stats.nbChars,
			NbLines: stats.nbLines,
		}
	}

	doc.FontMap = fontMap
}

// applyRolesToLines stamps each Line.Role from the FontMap.
func applyRolesToLines(doc *model.Document) {
	for i := range doc.Pages {
		for j := range doc.Pages[i].Flows {
			for k := range doc.Pages[i].Flows[j].Lines {
				line := &doc.Pages[i].Flows[j].Lines[k]
				if line.FontSize <= 0 {
					line.Role = model.RoleUnknown
					continue
				}
				bucket := math.Round(line.FontSize*2) / 2
				key := fmt.Sprintf("%.1f", bucket)
				if fs, ok := doc.FontMap[key]; ok {
					line.Role = fs.Role
				} else {
					line.Role = model.RoleUnknown
				}
			}
		}
	}
}
