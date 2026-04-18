package render

import (
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/user/pdf2md/internal/layout"
	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/normalization"
)

// HTML writes a self-contained HTML document with SVG-based page rendering.
// Each page is rendered as an inline SVG with exact dimensions from the document.
// Lines use textLength and lengthAdjust to fit within their bounding boxes.
// Debug overlays show flow, block, and line boundaries.
// When debugNorm is true and normalization debug data is available, an additional
// normalization overlay layer is rendered.
func HTML(w io.Writer, doc *model.Document, debugNorm ...bool) error {
	showNormalization := len(debugNorm) > 0 && debugNorm[0]
	if _, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — pdf2md</title>
<style>
  body { margin: 20px; font-family: 'Consolas', 'Monaco', 'Menlo', monospace; }
  .page { margin-bottom: 40px; }
  .page-info { font-size: 14px; color: #333; margin: 0 0 8px 0; }
  svg { border: 1px solid #666; display: block; width: 100%%; height: auto; }
  .h1 { fill: #e74c3c; font-weight: bold; }
  .h2 { fill: #e67e22; font-weight: bold; }
  .h3 { fill: #f1c40f; }
  .body { fill: #333; }
  .table { fill: #2980b9; font-family: "Courier New", Courier, monospace; white-space: pre; }
  .small { fill: #7f8c8d; }
  .excluded { fill: #c0392b; text-decoration: line-through; }
  .unknown { fill: #95a5a6; }
  .debug-flow { stroke-dasharray: 4 3; stroke-width: 1; }
  .debug-block { stroke: #e05c00; stroke-width: 1; fill: rgba(224, 92, 0, 0.04); }
  .debug-heading { stroke: rgba(180, 120, 0, 0.35); stroke-width: 0.5; stroke-dasharray: 3 3; fill: none; }
  .debug-line { stroke: rgba(0, 80, 200, 0.35); stroke-width: 0.5; fill: none; }
  .debug-label { font-size: 9px; fill: #333; font-family: monospace; }
  .debug-layout-zone { stroke-dasharray: 6 3; stroke-width: 1.5; }
  .debug-layout-label { font-size: 10px; fill: #000; font-family: monospace; font-weight: bold; }
  .debug-band-outline { stroke: rgba(100, 100, 100, 0.2); stroke-width: 0.5; fill: none; }
  .debug-horizontal-cut { stroke: rgba(200, 80, 0, 0.6); stroke-width: 1; stroke-dasharray: 8 4; }
  .debug-vertical-cut { stroke: rgba(0, 100, 200, 0.5); stroke-width: 1; stroke-dasharray: 4 4; }
  .debug-norm-gap-structural { stroke: red; stroke-width: 1; fill: rgba(220, 0, 0, 0.20); }
  .debug-norm-gap-isolated { stroke: orange; stroke-width: 1; stroke-dasharray: 4 4; fill: rgba(255, 160, 0, 0.15); }
  .debug-norm-lblock-text { stroke: rgba(100, 100, 100, 0.4); stroke-width: 1; fill: none; }
  .debug-norm-lblock-structural { stroke: rgba(255, 140, 0, 0.6); stroke-width: 1; fill: none; }
</style>
</head>
<body>
`, html.EscapeString(doc.Source)); err != nil {
		return err
	}

	// Flow colors for visual differentiation
	flowColors := []string{
		"rgba(0, 180, 160, 0.06)",  // Teal
		"rgba(147, 51, 234, 0.06)", // Purple
		"rgba(34, 197, 94, 0.06)",  // Green
		"rgba(251, 191, 36, 0.06)", // Gold
	}
	flowStrokeColors := []string{
		"rgba(0, 180, 160, 0.4)",  // Teal
		"rgba(147, 51, 234, 0.4)", // Purple
		"rgba(34, 197, 94, 0.4)",  // Green
		"rgba(251, 191, 36, 0.4)", // Gold
	}

	// Zone colors for visual differentiation
	zoneColors := []string{
		"rgba(255, 200, 0, 0.07)", // Yellow
		"rgba(0, 200, 100, 0.07)", // Green
		"rgba(200, 0, 200, 0.07)", // Magenta
		"rgba(0, 150, 255, 0.07)", // Blue
		"rgba(255, 100, 0, 0.07)", // Orange
	}
	zoneStrokeColors := []string{
		"rgba(255, 200, 0, 0.5)", // Yellow
		"rgba(0, 200, 100, 0.5)", // Green
		"rgba(200, 0, 200, 0.5)", // Magenta
		"rgba(0, 150, 255, 0.5)", // Blue
		"rgba(255, 100, 0, 0.5)", // Orange
	}

	for _, page := range doc.Pages {
		// Detect layout zones for this page
		pageLayout := layout.DetectLayout(&page, getBodyLineHeight(doc))

		// Build page summary: collect per-band column counts
		var bandColumns []string
		for _, zone := range pageLayout.Zones {
			for _, band := range zone.Bands {
				// Don't count single line bands toward the maximum amount of bands allowed in a page
				if layout.BandLineCount(band) == 1 {
					continue
				}
				cols := len(band.VerticalCuts) + 1
				bandColumns = append(bandColumns, fmt.Sprintf("%d", cols))
			}
		}

		// Page info in plain HTML above the SVG
		totalBands := len(bandColumns)
		if totalBands > 0 {
			if _, err := fmt.Fprintf(w, "<div class=\"page\">\n<p class=\"page-info\">Page: %d - Bands: %d - Columns: %s</p>\n",
				page.Number, totalBands, strings.Join(bandColumns, ", ")); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "<div class=\"page\">\n<p class=\"page-info\">Page: %d - Bands: 0</p>\n",
				page.Number); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%g\" height=\"%g\" viewBox=\"0 0 %g %g\">\n",
			page.Width, page.Height, page.Width, page.Height); err != nil {
			return err
		}

		// Layer 0a: Layout zone rectangles (bottom layer)
		if _, err := fmt.Fprint(w, "<g class=\"debug-layout\">\n"); err != nil {
			return err
		}
		for _, zone := range pageLayout.Zones {
			colorIdx := zone.Index % len(zoneColors)
			if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" fill=\"%s\" stroke=\"%s\" class=\"debug-layout-zone\"/>\n",
				zone.XMin, zone.YMin, zone.XMax-zone.XMin, zone.YMax-zone.YMin,
				zoneColors[colorIdx], zoneStrokeColors[colorIdx]); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 0b: Band outlines
		if _, err := fmt.Fprint(w, "<g class=\"debug-band-layer\">\n"); err != nil {
			return err
		}
		for _, zone := range pageLayout.Zones {
			for _, band := range zone.Bands {
				if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"debug-band-outline\"/>\n",
					band.XMin, band.YMin, band.XMax-band.XMin, band.YMax-band.YMin); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 0c: Horizontal cut lines
		if _, err := fmt.Fprint(w, "<g class=\"debug-horizontal-cuts\">\n"); err != nil {
			return err
		}
		for _, cut := range pageLayout.HorizontalCuts {
			if _, err := fmt.Fprintf(w, "<line x1=\"%g\" y1=\"%g\" x2=\"%g\" y2=\"%g\" class=\"debug-horizontal-cut\"/>\n",
				cut.XMin, cut.Y, cut.XMax, cut.Y); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 0d: Vertical cut lines (per band)
		if _, err := fmt.Fprint(w, "<g class=\"debug-vertical-cuts\">\n"); err != nil {
			return err
		}
		for _, zone := range pageLayout.Zones {
			for _, band := range zone.Bands {
				for _, cutX := range band.VerticalCuts {
					if _, err := fmt.Fprintf(w, "<line x1=\"%g\" y1=\"%g\" x2=\"%g\" y2=\"%g\" class=\"debug-vertical-cut\"/>\n",
						cutX, band.YMin, cutX, band.YMax); err != nil {
						return err
					}
				}
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 1: Flow rectangles (bottom layer, lowest opacity)
		if _, err := fmt.Fprint(w, "<g class=\"debug-flow-layer\">\n"); err != nil {
			return err
		}
		for flowIdx, flow := range page.Flows {
			colorIdx := flowIdx % len(flowColors)
			if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" fill=\"%s\" stroke=\"%s\" class=\"debug-flow\"/>\n",
				flow.XMin, flow.YMin, flow.XMax-flow.XMin, flow.YMax-flow.YMin,
				flowColors[colorIdx], flowStrokeColors[colorIdx]); err != nil {
				return err
			}
			// Flow label
			labelX := flow.XMin + 2
			labelY := flow.YMin + 9
			if _, err := fmt.Fprintf(w, "<text x=\"%g\" y=\"%g\" class=\"debug-label\">F%d</text>\n",
				labelX, labelY, flowIdx); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 2: Block rectangles
		if _, err := fmt.Fprint(w, "<g class=\"debug-block-layer\">\n"); err != nil {
			return err
		}
		blockIdx := 0
		for _, flow := range page.Flows {
			for _, block := range flow.Blocks {
				// Use different style for heading blocks
				blockClass := "debug-block"
				labelPrefix := "B"
				if block.IsHeading {
					blockClass = "debug-heading"
					labelPrefix = "H"
				}
				if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"%s\"/>\n",
					block.XMin, block.YMin, block.XMax-block.XMin, block.YMax-block.YMin, blockClass); err != nil {
					return err
				}
				// Block label
				labelX := block.XMin + 2
				labelY := block.YMin + 9
				if _, err := fmt.Fprintf(w, "<text x=\"%g\" y=\"%g\" class=\"debug-label\">%s%d</text>\n",
					labelX, labelY, labelPrefix, blockIdx); err != nil {
					return err
				}
				blockIdx++
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 3: Line rectangles
		if _, err := fmt.Fprint(w, "<g class=\"debug-line-layer\">\n"); err != nil {
			return err
		}
		for _, flow := range page.Flows {
			for _, block := range flow.Blocks {
				for _, line := range block.Lines {
					if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"debug-line\"/>\n",
						line.XMin, line.YMin, line.XMax-line.XMin, line.YMax-line.YMin); err != nil {
						return err
					}
				}
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		// Layer 3.5: Normalization debug overlays (between line rects and text content)
		if showNormalization {
			if err := renderNormalizationDebug(w, &page); err != nil {
				return err
			}
		}

		// Layer 4: Text content (top layer)
		if _, err := fmt.Fprint(w, "<g class=\"text-layer\">\n"); err != nil {
			return err
		}
		for _, flow := range page.Flows {
			for _, block := range flow.Blocks {
				for _, line := range block.Lines {
					// Y-coordinate is at the baseline (yMax).
					y := line.YMax
					fontSize := (line.YMax - line.YMin) * 0.9
					if _, err := fmt.Fprintf(w, "<text x=\"%g\" y=\"%g\" textLength=\"%g\" lengthAdjust=\"spacingAndGlyphs\" letter-spacing=\"0.02em\" font-size=\"%g\" class=\"%s\">%s</text>\n",
						line.XMin, y, line.XMax-line.XMin, fontSize, string(line.Role), html.EscapeString(line.Text)); err != nil {
						return err
					}
				}
			}
		}
		if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
			return err
		}

		if _, err := fmt.Fprint(w, "</svg>\n</div>\n"); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, "</body>\n</html>\n")
	return err
}

// getBodyLineHeight returns the body font size from the document's FontMap.
func getBodyLineHeight(doc *model.Document) float64 {
	for _, fs := range doc.FontMap {
		if fs.Role == model.RoleBody {
			return fs.Size
		}
	}
	return 10.0 // typical body text size in PDF points
}

// renderNormalizationDebug renders the normalization debug SVG layer for a page.
func renderNormalizationDebug(w io.Writer, page *model.Page) error {
	dd, ok := page.NormDebugData.(*normalization.DebugData)
	if !ok || dd == nil {
		return nil
	}

	if _, err := fmt.Fprint(w, "<g class=\"debug-normalization\">\n"); err != nil {
		return err
	}

	// Sub-layer 1: LogicalBlock outlines.
	for _, lb := range dd.LogicalBlocks {
		class := "debug-norm-lblock-text"
		bType := "TEXT"
		if lb.Type == normalization.BlockStructural {
			class = "debug-norm-lblock-structural"
			bType = "STRUCTURAL"
		}
		if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"%s\"><title>%s | lines:%d | sourceBlocks:%d</title></rect>\n",
			lb.XMin, lb.YMin, lb.XMax-lb.XMin, lb.YMax-lb.YMin, class,
			bType, len(lb.Lines), len(lb.SourceBlocks)); err != nil {
			return err
		}
	}

	// Sub-layer 2: VirtualLine rects (color by classification).
	for _, vl := range dd.VirtualLines {
		var fill, stroke string
		switch vl.Classification {
		case normalization.LineLeft:
			fill = "rgba(0, 180, 0, 0.10)"
			stroke = "green"
		case normalization.LineRight:
			fill = "rgba(0, 100, 255, 0.10)"
			stroke = "blue"
		case normalization.LineJustified:
			fill = "rgba(150, 0, 255, 0.10)"
			stroke = "purple"
		case normalization.LineStructural:
			fill = "rgba(255, 140, 0, 0.14)"
			stroke = "orange"
		default:
			fill = "rgba(128, 128, 128, 0.05)"
			stroke = "gray"
		}

		// Count structural and isolated gaps.
		structGaps, isoGaps := 0, 0
		for _, gc := range vl.GapCandidates {
			if gc.Type == normalization.GapIsolated {
				isoGaps++
			} else {
				structGaps++
			}
		}

		titleText := fmt.Sprintf("%s | words:%d | emptyL:%.1fpt emptyR:%.1fpt | coverage:%.2f | σ:%.1fpt | gaps:%dstructural/%disolated",
			vl.Classification.String(), len(vl.Words),
			vl.EmptyLeft, vl.EmptyRight, vl.Coverage, vl.GapSigma,
			structGaps, isoGaps)

		if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" fill=\"%s\" stroke=\"%s\" stroke-width=\"0.5\"><title>%s</title></rect>\n",
			vl.XMin, vl.YMin, vl.XMax-vl.XMin, vl.YMax-vl.YMin, fill, stroke,
			html.EscapeString(titleText)); err != nil {
			return err
		}
	}

	// Sub-layer 3: GapCandidate rects.
	for _, gc := range dd.GapCandidates {
		if gc.ParentLine == nil {
			continue
		}

		var class string
		colSize := 0
		switch gc.Type {
		case normalization.GapColumnType, normalization.GapTableType:
			class = "debug-norm-gap-structural"
		default:
			class = "debug-norm-gap-isolated"
		}
		if gc.VerticalGroup != nil {
			colSize = len(gc.VerticalGroup.Gaps)
		}

		titleText := fmt.Sprintf("%s | width:%.1fpt | column_size:%dlines | xCenter:%.1f",
			gc.Type.String(), gc.Width, colSize, gc.XCenter)

		if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"%s\"><title>%s</title></rect>\n",
			gc.XLeft, gc.ParentLine.YMin, gc.XRight-gc.XLeft, gc.ParentLine.YMax-gc.ParentLine.YMin,
			class, html.EscapeString(titleText)); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w, "</g>\n"); err != nil {
		return err
	}

	return nil
}
