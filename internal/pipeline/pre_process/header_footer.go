package pre_process

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/pipeline/event"
)

// Header/footer detection thresholds.
const (
	// hfZoneFraction is the fraction of page height that defines the header/footer zone
	// (top hfZoneFraction and bottom hfZoneFraction are candidate regions).
	hfZoneFraction = 0.15

	// hfPositionTol is the maximum difference in relative Y centre for two flows to be
	// considered at the same vertical position across pages.
	hfPositionTol = 0.04

	// hfTextSimilarity is the minimum normalised Levenshtein similarity [0,1] required
	// for two flow texts to be treated as the same header/footer.
	hfTextSimilarity = 0.75

	// hfMaxFlowHeightFrac is the maximum height (as a fraction of page height) that a
	// candidate flow may occupy. Taller flows are body content, not headers/footers.
	hfMaxFlowHeightFrac = 0.25

	// hfPageNumMaxLen is the maximum character length of a page-number text candidate.
	hfPageNumMaxLen = 30
)

var (
	hfDigitRe    = regexp.MustCompile(`\d+`)
	hfPageWordRe = regexp.MustCompile(`(?i)\b(pages?|of|p\.?|pg\.?|no\.?)\b`)
)

// HeaderFooterHandler detects and removes recurring headers, footers, and page numbers
// across pages.
//
// Headers appear near the top of the page at the same relative vertical position across
// multiple pages. The first occurrence of each detected header is kept in place; all
// subsequent occurrences on later pages are removed. Footers follow the mirror rule: the
// last occurrence is kept and all earlier ones are removed. Page numbers — identified by
// short texts containing a digit that match the same structural template (e.g. "Page #",
// "- # -", or plain "#") across pages — are removed entirely from all pages.
//
// Text matching uses a fuzzy Levenshtein similarity (≥ 0.75) so that OCR noise does not
// prevent detection. Page-number templates are compared after replacing every digit run
// with the placeholder "#", making "3" and "4", or "Page 3" and "Page 4", equivalent.
//
// For documents with fewer than 10 pages the minimum number of matching occurrences is 2;
// for larger documents it is 3 to reduce false positives.
//
// Priority: 25 (after FontRoles at 20, before ReadingOrder at 30).
type HeaderFooterHandler struct{}

// NewHeaderFooterHandler returns a HeaderFooterHandler.
func NewHeaderFooterHandler() *HeaderFooterHandler { return &HeaderFooterHandler{} }

func (h *HeaderFooterHandler) Event() event.Event { return event.PreProcess }
func (h *HeaderFooterHandler) Priority() int      { return 25 }

// hfCandidate holds the information needed to cluster and classify a single flow that
// sits inside the header or footer zone of its page.
type hfCandidate struct {
	pageIdx  int
	flowIdx  int
	relY     float64 // relative vertical centre: (YMin+YMax)/2 / page.Height
	text     string  // normalised concatenation of all line texts in the flow
	isTop    bool    // centre is in the top hfZoneFraction of the page
	isBottom bool    // centre is in the bottom hfZoneFraction of the page
}

func (h *HeaderFooterHandler) Run(doc *model.Document) error {
	totalPages := len(doc.Pages)
	if totalPages < 2 {
		return nil
	}

	// Require more occurrences on larger documents to cut down on false positives.
	minOccurrences := 2
	if totalPages >= 10 {
		minOccurrences = 3
	}

	// ── Step 1: collect candidate flows ──────────────────────────────────────────
	var candidates []hfCandidate

	for pi := range doc.Pages {
		page := &doc.Pages[pi]
		if page.Height <= 0 {
			continue
		}
		for fi, flow := range page.Flows {
			flowHeight := flow.YMax - flow.YMin
			if flowHeight/page.Height > hfMaxFlowHeightFrac {
				continue
			}
			relY := (flow.YMin + flow.YMax) / 2.0 / page.Height
			isTop := relY < hfZoneFraction
			isBottom := relY > 1.0-hfZoneFraction
			if !isTop && !isBottom {
				continue
			}
			text := hfFlowText(flow)
			if strings.TrimSpace(text) == "" {
				continue
			}
			candidates = append(candidates, hfCandidate{
				pageIdx:  pi,
				flowIdx:  fi,
				relY:     relY,
				text:     hfNormalizeText(text),
				isTop:    isTop,
				isBottom: isBottom,
			})
		}
	}

	if len(candidates) < 2 {
		return nil
	}

	// ── Step 2: cluster via union-find ────────────────────────────────────────────
	// Two candidates are linked when they are on different pages, in the same zone
	// (both top or both bottom), within hfPositionTol of each other in relative Y, and
	// either have similar text OR both look like page numbers with a matching template.
	n := len(candidates)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var findRoot func(int) int
	findRoot = func(x int) int {
		if parent[x] != x {
			parent[x] = findRoot(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		pa, pb := findRoot(a), findRoot(b)
		if pa != pb {
			parent[pa] = pb
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			ci, cj := candidates[i], candidates[j]
			if ci.pageIdx == cj.pageIdx {
				continue
			}
			// Must be in the same zone.
			if ci.isTop != cj.isTop || ci.isBottom != cj.isBottom {
				continue
			}
			if math.Abs(ci.relY-cj.relY) > hfPositionTol {
				continue
			}
			similar := hfTextsSimilar(ci.text, cj.text)
			pageNums := !similar &&
				hfIsPageNum(ci.text) && hfIsPageNum(cj.text) &&
				hfPageNumsSimilar(ci.text, cj.text)
			if similar || pageNums {
				union(i, j)
			}
		}
	}

	// ── Step 3: group into clusters ───────────────────────────────────────────────
	clusterMap := make(map[int][]int) // root index → slice of candidate indices
	for i := range candidates {
		root := findRoot(i)
		clusterMap[root] = append(clusterMap[root], i)
	}

	// ── Step 4: decide which flows to remove ──────────────────────────────────────
	// toRemove[pageIdx][flowIdx] = true  →  drop that flow from the page.
	toRemove := make(map[int]map[int]bool)
	markRemove := func(c hfCandidate) {
		if toRemove[c.pageIdx] == nil {
			toRemove[c.pageIdx] = make(map[int]bool)
		}
		toRemove[c.pageIdx][c.flowIdx] = true
	}

	for _, indices := range clusterMap {
		// Count distinct pages covered by this cluster.
		pageSet := make(map[int]bool)
		for _, idx := range indices {
			pageSet[candidates[idx].pageIdx] = true
		}
		if len(pageSet) < minOccurrences {
			continue
		}

		// Check whether all members look like page numbers.
		allPageNum := true
		for _, idx := range indices {
			if !hfIsPageNum(candidates[idx].text) {
				allPageNum = false
				break
			}
		}

		// Sort by page order so "first" and "last" are well-defined.
		sort.Slice(indices, func(a, b int) bool {
			return candidates[indices[a]].pageIdx < candidates[indices[b]].pageIdx
		})

		c0 := candidates[indices[0]]
		switch {
		case allPageNum:
			// Page numbers: remove from every page.
			for _, idx := range indices {
				markRemove(candidates[idx])
			}
		case c0.isTop:
			// Header: keep the first occurrence, remove all later ones.
			for i, idx := range indices {
				if i == 0 {
					continue
				}
				markRemove(candidates[idx])
			}
		default:
			// Footer: keep the last occurrence, remove all earlier ones.
			last := len(indices) - 1
			for i, idx := range indices {
				if i == last {
					continue
				}
				markRemove(candidates[idx])
			}
		}
	}

	// ── Step 5: apply removals ────────────────────────────────────────────────────
	for pi := range doc.Pages {
		if len(toRemove[pi]) == 0 {
			continue
		}
		page := &doc.Pages[pi]
		kept := page.Flows[:0]
		for fi, flow := range page.Flows {
			if !toRemove[pi][fi] {
				kept = append(kept, flow)
			}
		}
		page.Flows = kept
	}

	return nil
}

// hfFlowText returns the concatenated, trimmed text of every non-empty line in the flow.
func hfFlowText(flow model.Flow) string {
	var parts []string
	for _, line := range flow.Lines {
		if t := strings.TrimSpace(line.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// hfNormalizeText lowercases the text and collapses all whitespace runs to a single space.
func hfNormalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// hfTextsSimilar reports whether two normalised texts are the same or have a
// Levenshtein similarity of at least hfTextSimilarity.
func hfTextsSimilar(a, b string) bool {
	return a == b || hfSimilarity(a, b) >= hfTextSimilarity
}

// hfSimilarity computes the normalised Levenshtein similarity in [0, 1].
func hfSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	maxLen := len(ra)
	if lb := len(rb); lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := hfLevenshtein(ra, rb)
	return 1.0 - float64(dist)/float64(maxLen)
}

// hfLevenshtein computes the Levenshtein edit distance between two rune slices.
func hfLevenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	row := make([]int, lb+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= lb; j++ {
			tmp := row[j]
			if a[i-1] == b[j-1] {
				row[j] = prev
			} else {
				m := prev
				if row[j] < m {
					m = row[j]
				}
				if row[j-1] < m {
					m = row[j-1]
				}
				row[j] = 1 + m
			}
			prev = tmp
		}
	}
	return row[lb]
}

// hfIsPageNum reports whether text is a plausible page number.
//
// A text is considered a page number when:
//  1. Its length does not exceed hfPageNumMaxLen characters.
//  2. It contains at least one digit.
//  3. After removing recognised page-number words (page, of, p, pg, no),
//     all digit sequences, and common surrounding punctuation characters,
//     the remaining non-space content is at most 5 characters long.
func hfIsPageNum(text string) bool {
	if len(text) > hfPageNumMaxLen {
		return false
	}
	if !hfDigitRe.MatchString(text) {
		return false
	}
	stripped := hfPageWordRe.ReplaceAllString(text, "")
	stripped = hfDigitRe.ReplaceAllString(stripped, "")
	stripped = strings.TrimFunc(stripped, func(r rune) bool {
		switch r {
		case ' ', '-', '.', '/', '|', '(', ')', '[', ']', '–', '—', '·':
			return true
		}
		return false
	})
	return len([]rune(stripped)) <= 5
}

// hfPageNumsSimilar reports whether two page-number texts share the same structural
// template after replacing every digit run with the placeholder "#".
func hfPageNumsSimilar(a, b string) bool {
	na := hfDigitRe.ReplaceAllString(a, "#")
	nb := hfDigitRe.ReplaceAllString(b, "#")
	return na == nb || hfSimilarity(na, nb) >= hfTextSimilarity
}
