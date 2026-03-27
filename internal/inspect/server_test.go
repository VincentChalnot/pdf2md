package inspect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/pdf2md/internal/model"
)

func testDoc() *model.Document {
	return &model.Document{
		Source: "test.pdf",
		FontMap: map[string]model.FontSpec{
			"0": {ID: "0", Size: 12, Family: "Times", Color: "#000", Role: model.RoleBody, NbChars: 100, NbElems: 5},
			"1": {ID: "1", Size: 24, Family: "Arial", Color: "#f00", Role: model.RoleH1, NbChars: 20, NbElems: 2},
		},
		Outline: []model.OutlineItem{
			{Title: "Chapter 1", Page: 1},
			{Title: "Chapter 2", Page: 2},
		},
		Pages: []model.Page{
			{
				Number: 1, Width: 800, Height: 600,
				Elements: []model.Element{
					{Top: 10, Left: 10, Width: 200, Height: 30, FontID: "1", Role: model.RoleH1, Text: "Title"},
					{Top: 50, Left: 10, Width: 400, Height: 20, FontID: "0", Role: model.RoleBody, Text: "Body text"},
				},
			},
			{
				Number: 2, Width: 800, Height: 600,
				Elements: []model.Element{
					{Top: 10, Left: 10, Width: 400, Height: 20, FontID: "0", Role: model.RoleBody, Text: "Page 2 text"},
				},
			},
		},
	}
}

func TestRootRedirect(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.handleRoot(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("root: status = %d, want %d", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc != "/page/1" {
		t.Errorf("root: location = %q, want %q", loc, "/page/1")
	}
}

func TestPageView(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/page/1", nil)
	w := httptest.NewRecorder()
	srv.handlePage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("page 1: status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Title") {
		t.Error("page 1: response should contain 'Title'")
	}
	if !strings.Contains(body, "Body text") {
		t.Error("page 1: response should contain 'Body text'")
	}
}

func TestPageRaw(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/page/1/raw", nil)
	w := httptest.NewRecorder()
	srv.handlePage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("page 1 raw: status = %d, want %d", w.Code, http.StatusOK)
	}

	var page model.Page
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("page 1 raw: invalid JSON: %v", err)
	}
	if page.Number != 1 {
		t.Errorf("page number = %d, want 1", page.Number)
	}
}

func TestPageInvalidNumber(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	tests := []string{"/page/0", "/page/99", "/page/abc"}
	for _, path := range tests {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		srv.handlePage(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", path, w.Code, http.StatusBadRequest)
		}
	}
}

func TestFontsPage(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/fonts", nil)
	w := httptest.NewRecorder()
	srv.handleFonts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("fonts: status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Times") {
		t.Error("fonts: response should contain 'Times'")
	}
	if !strings.Contains(body, "Arial") {
		t.Error("fonts: response should contain 'Arial'")
	}
}

func TestOutlinePage(t *testing.T) {
	srv, err := NewServer(testDoc(), 8080)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/outline", nil)
	w := httptest.NewRecorder()
	srv.handleOutline(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("outline: status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Chapter 1") {
		t.Error("outline: response should contain 'Chapter 1'")
	}
}
