package extract

import (
	"fmt"
	"math"
	"sort"

	"github.com/user/pdf2md/internal/model"
)

// AssignFontRoles computes font size buckets from line font sizes,
// then assigns roles based on the heading-map algorithm.
func AssignFontRoles(doc *model.Document) {
	// 1. Aggregate font size statistics across all lines.
	type sizeStats struct {
		size    float64
		nbChars int
		nbLines int
	}

	statsMap := make(map[string]*sizeStats) // key: formatted size ("%.1f")

	for _, page := range doc.Pages {
		for _, flow := range page.Flows {
			for _, line := range flow.Lines {
				if line.FontSize <= 0 {
					continue
				}

				// Round to 0.5px bucket.
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

	// 2. Find body font: bucket with highest nbChars.
	var bodyKey string
	var bodyChars int
	for key, stats := range statsMap {
		if stats.nbChars > bodyChars {
			bodyChars = stats.nbChars
			bodyKey = key
		}
	}

	if bodyKey == "" {
		// No body font found — mark everything as unknown.
		doc.FontMap = make(map[string]model.FontSpec)
		return
	}

	bodySize := statsMap[bodyKey].size

	// 3. Collect buckets larger than body × 1.2, sorted by size DESC.
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
	sort.Slice(larger, func(i, j int) bool {
		return larger[i].size > larger[j].size
	})

	// 4. Build FontMap.
	fontMap := make(map[string]model.FontSpec)

	// Assign heading roles: h1, h2, h3 (cap at h3).
	headingRoles := []model.FontRole{model.RoleH1, model.RoleH2, model.RoleH3}
	for i, fe := range larger {
		role := model.RoleH3
		if i < len(headingRoles) {
			role = headingRoles[i]
		}
		fontMap[fe.key] = model.FontSpec{
			Size:    statsMap[fe.key].size,
			Role:    role,
			NbChars: statsMap[fe.key].nbChars,
			NbLines: statsMap[fe.key].nbLines,
		}
	}

	// Assign body.
	fontMap[bodyKey] = model.FontSpec{
		Size:    bodySize,
		Role:    model.RoleBody,
		NbChars: statsMap[bodyKey].nbChars,
		NbLines: statsMap[bodyKey].nbLines,
	}

	// Assign small and excluded for remaining buckets.
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

// ApplyRolesToLines sets the Role field on each line based on its font size bucket.
func ApplyRolesToLines(doc *model.Document) {
	for i := range doc.Pages {
		for j := range doc.Pages[i].Flows {
			for k := range doc.Pages[i].Flows[j].Lines {
				line := &doc.Pages[i].Flows[j].Lines[k]
				if line.FontSize <= 0 {
					line.Role = model.RoleUnknown
					continue
				}

				// Round to 0.5px bucket.
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
