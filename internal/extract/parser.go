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

// xmlDoc represents the root <pdf2xml> element.
type xmlDoc struct {
	XMLName xml.Name    `xml:"pdf2xml"`
	Pages   []xmlPage   `xml:"page"`
	Outline *xmlOutline `xml:"outline"`
}

// xmlPage represents a <page> element.
type xmlPage struct {
	Number   string       `xml:"number,attr"`
	Top      string       `xml:"top,attr"`
	Left     string       `xml:"left,attr"`
	Height   string       `xml:"height,attr"`
	Width    string       `xml:"width,attr"`
	Fonts    []xmlFont    `xml:"fontspec"`
	Elements []xmlElement `xml:"text"`
}

// xmlFont represents a <fontspec> element.
type xmlFont struct {
	ID     string `xml:"id,attr"`
	Size   string `xml:"size,attr"`
	Family string `xml:"family,attr"`
	Color  string `xml:"color,attr"`
}

// xmlElement represents a <text> element.
type xmlElement struct {
	Top    string `xml:"top,attr"`
	Left   string `xml:"left,attr"`
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
	Font   string `xml:"font,attr"`
	// InnerXML captures the raw content, including any inline tags like <b>, <i>.
	InnerXML string `xml:",innerxml"`
}

// xmlOutline represents an <outline> element which can be nested.
type xmlOutline struct {
	Items    []xmlOutlineItem `xml:"item"`
	Children []xmlOutline     `xml:"outline"`
}

// xmlOutlineItem represents an <item> element inside <outline>.
type xmlOutlineItem struct {
	Page  string `xml:"page,attr"`
	Title string `xml:",chardata"`
}

// ParseXML reads the pdftohtml XML output and returns a model.Document.
func ParseXML(xmlPath string) (*model.Document, error) {
	f, err := os.Open(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("opening XML file: %w", err)
	}
	defer f.Close()

	return parseXMLReader(f)
}

func parseXMLReader(r io.Reader) (*model.Document, error) {
	var doc xmlDoc
	decoder := xml.NewDecoder(r)
	// pdftohtml may produce non-UTF8 content; be lenient.
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding XML: %w", err)
	}

	fontMap := make(map[string]model.FontSpec)
	var pages []model.Page

	for _, xp := range doc.Pages {
		// Collect fonts from this page.
		for _, xf := range xp.Fonts {
			size, _ := strconv.ParseFloat(xf.Size, 64)
			fontMap[xf.ID] = model.FontSpec{
				ID:     xf.ID,
				Size:   size,
				Family: xf.Family,
				Color:  xf.Color,
				Role:   model.RoleUnknown,
			}
		}

		pageNum, _ := strconv.Atoi(xp.Number)
		pageW, _ := strconv.ParseFloat(xp.Width, 64)
		pageH, _ := strconv.ParseFloat(xp.Height, 64)

		var elements []model.Element
		for _, xe := range xp.Elements {
			top, _ := strconv.ParseFloat(xe.Top, 64)
			left, _ := strconv.ParseFloat(xe.Left, 64)
			width, _ := strconv.ParseFloat(xe.Width, 64)
			height, _ := strconv.ParseFloat(xe.Height, 64)

			text := stripXMLTags(xe.InnerXML)
			elements = append(elements, model.Element{
				Top:    top,
				Left:   left,
				Width:  width,
				Height: height,
				FontID: xe.Font,
				Text:   text,
			})
		}

		pages = append(pages, model.Page{
			Number:   pageNum,
			Width:    pageW,
			Height:   pageH,
			Elements: elements,
		})
	}

	var outline []model.OutlineItem
	if doc.Outline != nil {
		outline = parseOutlineLevel(doc.Outline)
	}

	return &model.Document{
		FontMap: fontMap,
		Outline: outline,
		Pages:   pages,
	}, nil
}

// parseOutlineLevel recursively parses nested outline elements.
func parseOutlineLevel(xo *xmlOutline) []model.OutlineItem {
	var items []model.OutlineItem

	for i, xi := range xo.Items {
		page, _ := strconv.Atoi(xi.Page)
		item := model.OutlineItem{
			Title: strings.TrimSpace(xi.Title),
			Page:  page,
		}

		// Each item's children come from the corresponding nested <outline> at the same index.
		if i < len(xo.Children) {
			item.Children = parseOutlineLevel(&xo.Children[i])
		}

		items = append(items, item)
	}

	// Handle any remaining nested outlines that don't correspond to items.
	for i := len(xo.Items); i < len(xo.Children); i++ {
		childItems := parseOutlineLevel(&xo.Children[i])
		items = append(items, childItems...)
	}

	return items
}

// stripXMLTags removes inline XML/HTML tags from a string (e.g. <b>, <i>).
func stripXMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}
