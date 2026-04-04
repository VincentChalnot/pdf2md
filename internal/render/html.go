package render

import (
	"fmt"
	"html"
	"io"

	"github.com/user/pdf2md/internal/model"
)

// HTML writes a self-contained HTML document with SVG-based page rendering.
// Each page is rendered as an inline SVG with exact dimensions from the document.
// Lines use textLength and lengthAdjust to fit within their bounding boxes.
// Debug overlays show flow, block, and line boundaries.
func HTML(w io.Writer, doc *model.Document) error {
	if _, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — pdf2md</title>
<style>
  body { margin: 20px; font-family: 'Consolas', 'Monaco', 'Menlo', monospace; }
  .page { margin-bottom: 40px; }
  svg { border: 1px solid #ccc; display: block; width: 100%%; height: auto; }
  .h1 { fill: #e74c3c; font-weight: bold; }
  .h2 { fill: #e67e22; font-weight: bold; }
  .h3 { fill: #f1c40f; }
  .body { fill: #333; }
  .small { fill: #7f8c8d; }
  .excluded { fill: #c0392b; text-decoration: line-through; }
  .unknown { fill: #95a5a6; }
  .debug-flow { stroke-dasharray: 4 3; stroke-width: 1; }
  .debug-block { stroke: #e05c00; stroke-width: 1; fill: rgba(224, 92, 0, 0.04); }
  .debug-line { stroke: rgba(0, 80, 200, 0.35); stroke-width: 0.5; fill: none; }
  .debug-label { font-size: 9px; fill: #333; font-family: monospace; }
</style>
</head>
<body>
`, html.EscapeString(doc.Source)); err != nil {
		return err
	}

	// Flow colors for visual differentiation
	flowColors := []string{
		"rgba(0, 180, 160, 0.06)",   // Teal
		"rgba(147, 51, 234, 0.06)",  // Purple
		"rgba(34, 197, 94, 0.06)",   // Green
		"rgba(251, 191, 36, 0.06)",  // Gold
	}
	flowStrokeColors := []string{
		"rgba(0, 180, 160, 0.4)",   // Teal
		"rgba(147, 51, 234, 0.4)",  // Purple
		"rgba(34, 197, 94, 0.4)",   // Green
		"rgba(251, 191, 36, 0.4)",  // Gold
	}

	for _, page := range doc.Pages {
		if _, err := fmt.Fprintf(w, "<div class=\"page\">\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%g\" height=\"%g\" viewBox=\"0 0 %g %g\">\n",
			page.Width, page.Height, page.Width, page.Height); err != nil {
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
				if _, err := fmt.Fprintf(w, "<rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\" class=\"debug-block\"/>\n",
					block.XMin, block.YMin, block.XMax-block.XMin, block.YMax-block.YMin); err != nil {
					return err
				}
				// Block label
				labelX := block.XMin + 2
				labelY := block.YMin + 9
				if _, err := fmt.Fprintf(w, "<text x=\"%g\" y=\"%g\" class=\"debug-label\">B%d (%.1f,%.1f)→(%.1f,%.1f)</text>\n",
					labelX, labelY, blockIdx, block.XMin, block.YMin, block.XMax, block.YMax); err != nil {
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
