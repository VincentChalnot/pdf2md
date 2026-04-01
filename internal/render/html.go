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
</style>
</head>
<body>
`, html.EscapeString(doc.Source)); err != nil {
		return err
	}

	for _, page := range doc.Pages {
		if _, err := fmt.Fprintf(w, "<div class=\"page\">\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%g\" height=\"%g\" viewBox=\"0 0 %g %g\">\n",
			page.Width, page.Height, page.Width, page.Height); err != nil {
			return err
		}

		for _, flow := range page.Flows {
			for _, line := range flow.Lines {
				// Y-coordinate is at the baseline (yMax).
				y := line.YMax
				fontSize := (line.YMax - line.YMin) * 0.9
				if _, err := fmt.Fprintf(w, "<text x=\"%g\" y=\"%g\" textLength=\"%g\" lengthAdjust=\"spacingAndGlyphs\" letter-spacing=\"0.02em\" font-size=\"%g\" class=\"%s\">%s</text>\n",
					line.XMin, y, line.XMax-line.XMin, fontSize, string(line.Role), html.EscapeString(line.Text)); err != nil {
					return err
				}
			}
		}

		if _, err := fmt.Fprint(w, "</svg>\n</div>\n"); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, "</body>\n</html>\n")
	return err
}
