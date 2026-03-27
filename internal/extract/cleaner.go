package extract

import (
	"sort"

	"github.com/user/pdf2md/internal/model"
)

// Clean applies the cleaning pipeline to the document:
// 1. Exclude fonts
// 2. Dedup elements by (top, left, text)
// 3. Sort elements by top ASC, then left ASC
func Clean(doc *model.Document, excludeFonts map[string]bool) {
	for i := range doc.Pages {
		page := &doc.Pages[i]

		// 1. Exclude fonts: remove elements whose font is in the exclude list.
		if len(excludeFonts) > 0 {
			var filtered []model.Element
			for _, e := range page.Elements {
				if !excludeFonts[e.FontID] {
					filtered = append(filtered, e)
				}
			}
			page.Elements = filtered
		}

		// 2. Dedup: remove duplicate elements by (top, left, text) — keep first occurrence.
		seen := make(map[dedupKey]bool)
		var deduped []model.Element
		for _, e := range page.Elements {
			key := dedupKey{top: e.Top, left: e.Left, text: e.Text}
			if !seen[key] {
				seen[key] = true
				deduped = append(deduped, e)
			}
		}
		page.Elements = deduped

		// 3. Sort: by top ASC, then left ASC.
		sort.Slice(page.Elements, func(a, b int) bool {
			if page.Elements[a].Top != page.Elements[b].Top {
				return page.Elements[a].Top < page.Elements[b].Top
			}
			return page.Elements[a].Left < page.Elements[b].Left
		})
	}
}

type dedupKey struct {
	top  float64
	left float64
	text string
}
