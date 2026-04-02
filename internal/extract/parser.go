package extract

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/user/pdf2md/internal/model"
)

// bboxDoc represents the root structure of pdftotext -bbox-layout output.
type bboxDoc struct {
	XMLName xml.Name `xml:"html"`
	Head    bboxHead `xml:"head"`
	Body    bboxBody `xml:"body"`
}

// bboxHead represents the <head> element with metadata.
type bboxHead struct {
	Metas []bboxMeta `xml:"meta"`
}

// bboxMeta represents a <meta> element.
type bboxMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// bboxBody represents the <body> element containing the doc.
type bboxBody struct {
	Doc bboxDocElem `xml:"doc"`
}

// bboxDocElem represents the <doc> element.
type bboxDocElem struct {
	Pages []bboxPage `xml:"page"`
}

// bboxPage represents a <page> element.
type bboxPage struct {
	Width  string     `xml:"width,attr"`
	Height string     `xml:"height,attr"`
	Flows  []bboxFlow `xml:"flow"`
}

// bboxFlow represents a <flow> element.
type bboxFlow struct {
	Blocks []bboxBlock `xml:"block"`
}

// bboxBlock represents a <block> element.
type bboxBlock struct {
	XMin  string     `xml:"xMin,attr"`
	YMin  string     `xml:"yMin,attr"`
	XMax  string     `xml:"xMax,attr"`
	YMax  string     `xml:"yMax,attr"`
	Lines []bboxLine `xml:"line"`
}

// bboxLine represents a <line> element.
type bboxLine struct {
	XMin  string     `xml:"xMin,attr"`
	YMin  string     `xml:"yMin,attr"`
	XMax  string     `xml:"xMax,attr"`
	YMax  string     `xml:"yMax,attr"`
	Words []bboxWord `xml:"word"`
}

// bboxWord represents a <word> element.
type bboxWord struct {
	XMin string `xml:"xMin,attr"`
	YMin string `xml:"yMin,attr"`
	XMax string `xml:"xMax,attr"`
	YMax string `xml:"yMax,attr"`
	Text string `xml:",chardata"`
}

// ParseBBoxHTML reads the pdftotext -bbox-layout HTML output and returns a model.Document.
func ParseBBoxHTML(htmlPath string) (*model.Document, error) {
	f, err := os.Open(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("opening HTML file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	return parseBBoxReader(f)
}

func parseBBoxReader(r io.Reader) (*model.Document, error) {
	// Read all content and sanitize illegal XML characters.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading HTML: %w", err)
	}

	// Remove illegal XML characters (control characters except tab, newline, carriage return).
	sanitized := make([]byte, 0, len(data))
	for _, b := range data {
		// Allow tab (0x09), newline (0x0A), carriage return (0x0D), and printable characters.
		if b == 0x09 || b == 0x0A || b == 0x0D || b >= 0x20 {
			sanitized = append(sanitized, b)
		}
	}

	var doc bboxDoc
	decoder := xml.NewDecoder(strings.NewReader(string(sanitized)))
	// pdftotext may produce non-UTF8 content; be lenient.
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding HTML: %w", err)
	}

	// Extract metadata from <meta> tags.
	meta := make(map[string]string)
	for _, m := range doc.Head.Metas {
		if m.Name != "" && m.Content != "" {
			meta[m.Name] = m.Content
		}
	}

	var pages []model.Page

	for pageNum, xp := range doc.Body.Doc.Pages {
		pageW, _ := strconv.ParseFloat(xp.Width, 64)
		pageH, _ := strconv.ParseFloat(xp.Height, 64)

		var flows []model.Flow
		for _, xf := range xp.Flows {
			var lines []model.Line
			var flowXMin, flowYMin, flowXMax, flowYMax float64
			firstBlock := true

			for _, xb := range xf.Blocks {
				blockXMin, _ := strconv.ParseFloat(xb.XMin, 64)
				blockYMin, _ := strconv.ParseFloat(xb.YMin, 64)
				blockXMax, _ := strconv.ParseFloat(xb.XMax, 64)
				blockYMax, _ := strconv.ParseFloat(xb.YMax, 64)

				// Update flow bounding box.
				if firstBlock {
					flowXMin = blockXMin
					flowYMin = blockYMin
					flowXMax = blockXMax
					flowYMax = blockYMax
					firstBlock = false
				} else {
					if blockXMin < flowXMin {
						flowXMin = blockXMin
					}
					if blockYMin < flowYMin {
						flowYMin = blockYMin
					}
					if blockXMax > flowXMax {
						flowXMax = blockXMax
					}
					if blockYMax > flowYMax {
						flowYMax = blockYMax
					}
				}

				for _, xl := range xb.Lines {
					lineXMin, _ := strconv.ParseFloat(xl.XMin, 64)
					lineYMin, _ := strconv.ParseFloat(xl.YMin, 64)
					lineXMax, _ := strconv.ParseFloat(xl.XMax, 64)
					lineYMax, _ := strconv.ParseFloat(xl.YMax, 64)

					var words []model.Word
					for _, xw := range xl.Words {
						wordXMin, _ := strconv.ParseFloat(xw.XMin, 64)
						wordYMin, _ := strconv.ParseFloat(xw.YMin, 64)
						wordXMax, _ := strconv.ParseFloat(xw.XMax, 64)
						wordYMax, _ := strconv.ParseFloat(xw.YMax, 64)

						words = append(words, model.Word{
							XMin: wordXMin,
							YMin: wordYMin,
							XMax: wordXMax,
							YMax: wordYMax,
							Text: strings.TrimSpace(xw.Text),
						})
					}

					// Compute font size as median word height.
					fontSize := computeMedianWordHeight(words)

					// Join words with single space.
					var textParts []string
					for _, w := range words {
						if w.Text != "" {
							textParts = append(textParts, w.Text)
						}
					}
					text := strings.Join(textParts, " ")

					lines = append(lines, model.Line{
						XMin:     lineXMin,
						YMin:     lineYMin,
						XMax:     lineXMax,
						YMax:     lineYMax,
						FontSize: fontSize,
						Text:     text,
					})
				}
			}

			// Only add flow if it has lines.
			if len(lines) > 0 {
				flows = append(flows, model.Flow{
					XMin:  flowXMin,
					YMin:  flowYMin,
					XMax:  flowXMax,
					YMax:  flowYMax,
					Lines: lines,
				})
			}
		}

		pages = append(pages, model.Page{
			Number: pageNum + 1, // 1-indexed
			Width:  pageW,
			Height: pageH,
			Flows:  flows,
		})
	}

	return &model.Document{
		Meta:    meta,
		FontMap: make(map[string]model.FontSpec), // Will be populated by headingmap
		Pages:   pages,
	}, nil
}

// computeMedianWordHeight computes the median height (yMax - yMin) of words.
func computeMedianWordHeight(words []model.Word) float64 {
	if len(words) == 0 {
		return 0
	}

	var heights []float64
	for _, w := range words {
		h := w.YMax - w.YMin
		if h > 0 {
			heights = append(heights, h)
		}
	}

	if len(heights) == 0 {
		return 0
	}

	// Sort heights to find median.
	// Simple bubble sort for small arrays.
	for i := 0; i < len(heights); i++ {
		for j := i + 1; j < len(heights); j++ {
			if heights[i] > heights[j] {
				heights[i], heights[j] = heights[j], heights[i]
			}
		}
	}

	mid := len(heights) / 2
	if len(heights)%2 == 0 {
		return (heights[mid-1] + heights[mid]) / 2
	}
	return heights[mid]
}
