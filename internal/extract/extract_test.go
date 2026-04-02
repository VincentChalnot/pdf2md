package extract

import (
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func TestParseBBox(t *testing.T) {
	bboxHTML := `<!DOCTYPE html>
<html>
<head><meta name="Creator" content="TestApp"/></head>
<body>
<doc>
<page width="904" height="1174">
<flow>
<block xMin="50" yMin="90" xMax="450" yMax="130">
<line xMin="50" yMin="90" xMax="450" yMax="130">
<word xMin="50" yMin="100" xMax="150" yMax="130">Hello</word>
<word xMin="160" yMin="100" xMax="350" yMax="130">World</word>
</line>
</block>
</flow>
</page>
<page width="904" height="1174">
<flow>
<block xMin="50" yMin="90" xMax="450" yMax="130">
<line xMin="50" yMin="90" xMax="450" yMax="130">
<word xMin="50" yMin="100" xMax="250" yMax="130">Page two</word>
</line>
</block>
</flow>
</page>
</doc>
</body>
</html>`

	doc, err := parseBBoxReader(strings.NewReader(bboxHTML))
	if err != nil {
		t.Fatalf("parseBBoxReader failed: %v", err)
	}

	// Check pages.
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

	// Check flows.
	if len(p1.Flows) != 1 {
		t.Fatalf("page 1: expected 1 flow, got %d", len(p1.Flows))
	}

	// Check lines.
	flow := p1.Flows[0]
	if len(flow.Lines) != 1 {
		t.Fatalf("flow: expected 1 line, got %d", len(flow.Lines))
	}

	line := flow.Lines[0]
	if line.Text != "Hello World" {
		t.Errorf("line text = %q, want %q", line.Text, "Hello World")
	}
	if line.XMin != 50 {
		t.Errorf("line XMin = %f, want 50", line.XMin)
	}

	// Check meta.
	if doc.Meta["Creator"] != "TestApp" {
		t.Errorf("meta Creator = %q, want %q", doc.Meta["Creator"], "TestApp")
	}
}

func TestParseBBoxMultipleLinesPerFlow(t *testing.T) {
	bboxHTML := `<!DOCTYPE html>
<html>
<body>
<doc>
<page width="500" height="700">
<flow>
<block xMin="10" yMin="10" xMax="400" yMax="100">
<line xMin="10" yMin="10" xMax="200" yMax="30">
<word xMin="10" yMin="10" xMax="200" yMax="30">First line</word>
</line>
<line xMin="10" yMin="40" xMax="200" yMax="60">
<word xMin="10" yMin="40" xMax="200" yMax="60">Second line</word>
</line>
</block>
</flow>
</page>
</doc>
</body>
</html>`

	doc, err := parseBBoxReader(strings.NewReader(bboxHTML))
	if err != nil {
		t.Fatalf("parseBBoxReader failed: %v", err)
	}

	if len(doc.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(doc.Pages))
	}

	flow := doc.Pages[0].Flows[0]
	if len(flow.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(flow.Lines))
	}

	if flow.Lines[0].Text != "First line" {
		t.Errorf("line 0 text = %q, want %q", flow.Lines[0].Text, "First line")
	}
	if flow.Lines[1].Text != "Second line" {
		t.Errorf("line 1 text = %q, want %q", flow.Lines[1].Text, "Second line")
	}
}

func TestCleanFiltersSmallText(t *testing.T) {
	doc := &model.Document{
		FontMap: make(map[string]model.FontSpec),
		Pages: []model.Page{
			{
				Number: 1, Width: 500, Height: 700,
				Flows: []model.Flow{
					{
						Lines: []model.Line{
							{FontSize: 12, Text: "Normal text"},
							{FontSize: 1, Text: "Tiny text"},
						},
					},
					{
						Lines: []model.Line{
							{FontSize: 0.5, Text: "Very tiny"},
						},
					},
				},
			},
		},
	}

	Clean(doc, 2.0)

	if len(doc.Pages[0].Flows) != 1 {
		t.Fatalf("expected 1 flow after cleaning, got %d", len(doc.Pages[0].Flows))
	}

	flow := doc.Pages[0].Flows[0]
	if len(flow.Lines) != 2 {
		t.Fatalf("expected 2 lines after cleaning, got %d", len(flow.Lines))
	}
}

func TestCleanDedupLines(t *testing.T) {
	doc := &model.Document{
		FontMap: make(map[string]model.FontSpec),
		Pages: []model.Page{
			{
				Number: 1, Width: 500, Height: 700,
				Flows: []model.Flow{
					{
						Lines: []model.Line{
							{FontSize: 12, XMin: 10, YMin: 100, Text: "duplicate"},
							{FontSize: 12, XMin: 10, YMin: 100, Text: "duplicate"},
							{FontSize: 12, XMin: 10, YMin: 200, Text: "unique"},
						},
					},
				},
			},
		},
	}

	Clean(doc, 2.0)

	lines := doc.Pages[0].Flows[0].Lines
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines after dedup, got %d", len(lines))
	}
}

func TestAssignFontRoles(t *testing.T) {
	doc := &model.Document{
		FontMap: make(map[string]model.FontSpec),
		Pages: []model.Page{
			{
				Number: 1,
				Flows: []model.Flow{
					{
						Lines: []model.Line{
							{FontSize: 12, Text: "This is body text that is quite long and has many characters in it"},
							{FontSize: 12, Text: "More body text here too"},
							{FontSize: 30, Text: "Big Title"},
							{FontSize: 24, Text: "Heading"},
							{FontSize: 18, Text: "Subheading"},
							{FontSize: 6, Text: "small"},
						},
					},
				},
			},
		},
	}

	AssignFontRoles(doc)

	// Font size 12 has the most chars -> body
	if fs, ok := doc.FontMap["12.0"]; !ok {
		t.Fatal("font bucket 12.0 not found")
	} else if fs.Role != model.RoleBody {
		t.Errorf("font 12.0: role = %q, want %q", fs.Role, model.RoleBody)
	}

	// Font size 30 -> h1 (largest)
	if fs, ok := doc.FontMap["30.0"]; !ok {
		t.Fatal("font bucket 30.0 not found")
	} else if fs.Role != model.RoleH1 {
		t.Errorf("font 30.0: role = %q, want %q", fs.Role, model.RoleH1)
	}

	// Font size 6 < body * 0.85 -> small
	if fs, ok := doc.FontMap["6.0"]; !ok {
		t.Fatal("font bucket 6.0 not found")
	} else if fs.Role != model.RoleSmall {
		t.Errorf("font 6.0: role = %q, want %q", fs.Role, model.RoleSmall)
	}
}

func TestApplyRolesToLines(t *testing.T) {
	doc := &model.Document{
		FontMap: map[string]model.FontSpec{
			"12.0": {Size: 12, Role: model.RoleBody},
			"24.0": {Size: 24, Role: model.RoleH1},
		},
		Pages: []model.Page{
			{
				Flows: []model.Flow{
					{
						Lines: []model.Line{
							{FontSize: 12, Text: "body"},
							{FontSize: 24, Text: "heading"},
							{FontSize: 5, Text: "unknown size"},
						},
					},
				},
			},
		},
	}

	ApplyRolesToLines(doc)

	lines := doc.Pages[0].Flows[0].Lines
	if lines[0].Role != model.RoleBody {
		t.Errorf("line 0 role = %q, want %q", lines[0].Role, model.RoleBody)
	}
	if lines[1].Role != model.RoleH1 {
		t.Errorf("line 1 role = %q, want %q", lines[1].Role, model.RoleH1)
	}
	if lines[2].Role != model.RoleUnknown {
		t.Errorf("line 2 role = %q, want %q", lines[2].Role, model.RoleUnknown)
	}
}

func TestIsPurelyNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"12.34", true},
		{"1,000", true},
		{"1-2-3", true},
		{"abc", false},
		{"", false},
		{"12a34", false},
	}
	for _, tt := range tests {
		got := isPurelyNumeric(tt.input)
		if got != tt.want {
			t.Errorf("isPurelyNumeric(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
