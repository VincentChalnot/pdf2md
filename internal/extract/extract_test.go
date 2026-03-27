package extract

import (
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func TestParseXML(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pdf2xml SYSTEM "pdf2xml.dtd">
<pdf2xml>
  <page number="1" position="absolute" top="0" left="0" height="1174" width="904">
    <fontspec id="0" size="12" family="Times" color="#000000"/>
    <fontspec id="1" size="24" family="Arial" color="#ff0000"/>
    <text top="100" left="50" width="400" height="30" font="0">Hello World</text>
    <text top="200" left="50" width="400" height="40" font="1"><b>Title</b></text>
  </page>
  <page number="2" position="absolute" top="0" left="0" height="1174" width="904">
    <text top="100" left="50" width="400" height="30" font="0">Page two text</text>
  </page>
  <outline>
    <item page="1">Chapter 1</item>
    <item page="2">Chapter 2</item>
  </outline>
</pdf2xml>`

	doc, err := parseXMLReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("parseXMLReader failed: %v", err)
	}

	// Check pages
	if len(doc.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(doc.Pages))
	}

	p1 := doc.Pages[0]
	if p1.Number != 1 {
		t.Errorf("page 1 number = %d, want 1", p1.Number)
	}
	if p1.Width != 904 {
		t.Errorf("page 1 width = %f, want 904", p1.Width)
	}
	if p1.Height != 1174 {
		t.Errorf("page 1 height = %f, want 1174", p1.Height)
	}
	if len(p1.Elements) != 2 {
		t.Fatalf("page 1: expected 2 elements, got %d", len(p1.Elements))
	}

	// Check first element
	e1 := p1.Elements[0]
	if e1.Text != "Hello World" {
		t.Errorf("element 1 text = %q, want %q", e1.Text, "Hello World")
	}
	if e1.FontID != "0" {
		t.Errorf("element 1 fontID = %q, want %q", e1.FontID, "0")
	}
	if e1.Top != 100 {
		t.Errorf("element 1 top = %f, want 100", e1.Top)
	}

	// Check inline tag stripping
	e2 := p1.Elements[1]
	if e2.Text != "Title" {
		t.Errorf("element 2 text = %q, want %q (tags should be stripped)", e2.Text, "Title")
	}

	// Check fonts
	if len(doc.FontMap) != 2 {
		t.Fatalf("expected 2 fonts, got %d", len(doc.FontMap))
	}
	f0 := doc.FontMap["0"]
	if f0.Size != 12 {
		t.Errorf("font 0 size = %f, want 12", f0.Size)
	}
	if f0.Family != "Times" {
		t.Errorf("font 0 family = %q, want %q", f0.Family, "Times")
	}

	// Check outline
	if len(doc.Outline) != 2 {
		t.Fatalf("expected 2 outline items, got %d", len(doc.Outline))
	}
	if doc.Outline[0].Title != "Chapter 1" {
		t.Errorf("outline[0] title = %q, want %q", doc.Outline[0].Title, "Chapter 1")
	}
	if doc.Outline[0].Page != 1 {
		t.Errorf("outline[0] page = %d, want 1", doc.Outline[0].Page)
	}
}

func TestParseXMLNestedOutline(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
  <page number="1" position="absolute" top="0" left="0" height="100" width="100">
    <fontspec id="0" size="12" family="Times" color="#000000"/>
    <text top="10" left="10" width="80" height="15" font="0">Test</text>
  </page>
  <outline>
    <item page="1">Parent</item>
    <outline>
      <item page="2">Child 1</item>
      <item page="3">Child 2</item>
    </outline>
  </outline>
</pdf2xml>`

	doc, err := parseXMLReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("parseXMLReader failed: %v", err)
	}

	if len(doc.Outline) != 1 {
		t.Fatalf("expected 1 top-level outline item, got %d", len(doc.Outline))
	}
	parent := doc.Outline[0]
	if parent.Title != "Parent" {
		t.Errorf("parent title = %q, want %q", parent.Title, "Parent")
	}
	if len(parent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(parent.Children))
	}
	if parent.Children[0].Title != "Child 1" {
		t.Errorf("child 1 title = %q, want %q", parent.Children[0].Title, "Child 1")
	}
}

func TestStripXMLTags(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<b>bold</b>", "bold"},
		{"no tags", "no tags"},
		{"<i>italic</i> and <b>bold</b>", "italic and bold"},
		{"", ""},
		{"<a href=\"x\">link</a>", "link"},
	}
	for _, tt := range tests {
		got := stripXMLTags(tt.input)
		if got != tt.want {
			t.Errorf("stripXMLTags(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClean(t *testing.T) {
	doc := &model.Document{
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12},
			"1": {ID: "1", Size: 24},
		},
		Pages: []model.Page{
			{
				Number: 1,
				Elements: []model.Element{
					{Top: 200, Left: 50, FontID: "0", Text: "second"},
					{Top: 100, Left: 50, FontID: "0", Text: "first"},
					{Top: 100, Left: 50, FontID: "0", Text: "first"}, // duplicate
					{Top: 100, Left: 100, FontID: "1", Text: "excluded"},
				},
			},
		},
	}

	exclude := map[string]bool{"1": true}
	Clean(doc, exclude)

	elems := doc.Pages[0].Elements
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements after cleaning, got %d", len(elems))
	}

	// Check sort order: top=100 should come before top=200
	if elems[0].Text != "first" {
		t.Errorf("elems[0].Text = %q, want %q", elems[0].Text, "first")
	}
	if elems[1].Text != "second" {
		t.Errorf("elems[1].Text = %q, want %q", elems[1].Text, "second")
	}
}

func TestAssignFontRoles(t *testing.T) {
	doc := &model.Document{
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12, Family: "Times"},
			"1": {ID: "1", Size: 24, Family: "Arial"},
			"2": {ID: "2", Size: 18, Family: "Helvetica"},
			"3": {ID: "3", Size: 8, Family: "Courier"},
			"4": {ID: "4", Size: 30, Family: "Georgia"},
		},
		Pages: []model.Page{
			{
				Number: 1,
				Elements: []model.Element{
					// Font 0 has the most chars -> body
					{FontID: "0", Text: "This is body text that is quite long and has many characters in it"},
					{FontID: "0", Text: "More body text here"},
					{FontID: "1", Text: "Heading"},
					{FontID: "2", Text: "Subheading"},
					{FontID: "3", Text: "small"},
					{FontID: "4", Text: "Big Title"},
				},
			},
		},
	}

	AssignFontRoles(doc, map[string]bool{})

	expectations := map[string]model.FontRole{
		"0": model.RoleBody,
		"4": model.RoleH1, // size 30, largest
		"1": model.RoleH2, // size 24
		"2": model.RoleH3, // size 18
		"3": model.RoleSmall,
	}

	for id, wantRole := range expectations {
		fs := doc.FontMap[id]
		if fs.Role != wantRole {
			t.Errorf("font %s: role = %q, want %q", id, fs.Role, wantRole)
		}
	}
}

func TestAssignFontRolesExcluded(t *testing.T) {
	doc := &model.Document{
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12},
			"1": {ID: "1", Size: 24},
		},
		Pages: []model.Page{
			{
				Number: 1,
				Elements: []model.Element{
					{FontID: "0", Text: "body text"},
					{FontID: "1", Text: "heading"},
				},
			},
		},
	}

	AssignFontRoles(doc, map[string]bool{"1": true})

	if doc.FontMap["1"].Role != model.RoleExcluded {
		t.Errorf("font 1: role = %q, want %q", doc.FontMap["1"].Role, model.RoleExcluded)
	}
	if doc.FontMap["0"].Role != model.RoleBody {
		t.Errorf("font 0: role = %q, want %q", doc.FontMap["0"].Role, model.RoleBody)
	}
}

func TestResolveTOCAutoWithOutline(t *testing.T) {
	doc := &model.Document{
		Outline: []model.OutlineItem{
			{Title: "Ch1", Page: 1},
			{Title: "Ch2", Page: 2},
		},
	}
	ResolveTOC(doc, "auto")
	if len(doc.Outline) != 2 || doc.Outline[0].Title != "Ch1" {
		t.Errorf("auto with >=2 outline items should keep outline")
	}
}

func TestResolveTOCAutoFallback(t *testing.T) {
	doc := &model.Document{
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12, Role: model.RoleBody},
			"1": {ID: "1", Size: 24, Role: model.RoleH1},
		},
		Outline: []model.OutlineItem{
			{Title: "Only one", Page: 1},
		},
		Pages: []model.Page{
			{
				Number: 1,
				Elements: []model.Element{
					{FontID: "1", Role: model.RoleH1, Text: "Title One"},
					{FontID: "0", Role: model.RoleBody, Text: "Body text"},
				},
			},
			{
				Number: 2,
				Elements: []model.Element{
					{FontID: "1", Role: model.RoleH1, Text: "Title Two"},
				},
			},
		},
	}
	ResolveTOC(doc, "auto")
	if len(doc.Outline) != 2 {
		t.Fatalf("auto fallback: expected 2 heading items, got %d", len(doc.Outline))
	}
	if doc.Outline[0].Title != "Title One" {
		t.Errorf("auto fallback outline[0] = %q, want %q", doc.Outline[0].Title, "Title One")
	}
}

func TestBuildHeadingsTOC(t *testing.T) {
	doc := &model.Document{
		Pages: []model.Page{
			{
				Number: 1,
				Elements: []model.Element{
					{Role: model.RoleH1, Text: "Main Title"},
					{Role: model.RoleBody, Text: "body text"},
					{Role: model.RoleH2, Text: "Section 1"},
				},
			},
			{
				Number: 2,
				Elements: []model.Element{
					{Role: model.RoleH1, Text: "Chapter 2"},
					{Role: model.RoleSmall, Text: "footer"},
				},
			},
		},
	}

	items := BuildHeadingsTOC(doc)
	if len(items) != 3 {
		t.Fatalf("expected 3 TOC items, got %d", len(items))
	}
	if items[0].Title != "Main Title" || items[0].Page != 1 {
		t.Errorf("item 0: got %+v", items[0])
	}
	if items[1].Title != "Section 1" || items[1].Page != 1 {
		t.Errorf("item 1: got %+v", items[1])
	}
	if items[2].Title != "Chapter 2" || items[2].Page != 2 {
		t.Errorf("item 2: got %+v", items[2])
	}
}
