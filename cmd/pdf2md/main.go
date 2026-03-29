package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/pdf2md/internal/extract"
	"github.com/user/pdf2md/internal/model"
	"github.com/user/pdf2md/internal/render"
)

func main() {
	format := flag.String("format", "markdown", "Output format: xml, json, html, markdown")
	cacheDir := flag.String("cache-dir", "", "Directory for intermediate files (kept after run)")
	excludeFonts := flag.String("exclude-fonts", "", "Comma-separated font IDs to exclude")
	tocSource := flag.String("toc-source", "auto", `TOC source: "auto", "outline", or "headings"`)
	pretty := flag.Bool("pretty", false, "Pretty-print JSON output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pdf2md [flags] <filepath>...\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	for _, inputPath := range flag.Args() {
		if err := processFile(inputPath, *format, *cacheDir, *excludeFonts, *tocSource, *pretty); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func processFile(inputPath, format, cacheDir, excludeFontsStr, tocSource string, pretty bool) error {
	ext := strings.ToLower(filepath.Ext(inputPath))

	// Validate format value.
	switch format {
	case "xml", "json", "html", "markdown":
	default:
		return fmt.Errorf("unsupported format: %s (must be xml, json, html, or markdown)", format)
	}

	// Validate format compatibility with input type.
	switch ext {
	case ".pdf":
		// PDF can output any format.
	case ".xml":
		if format == "xml" {
			return fmt.Errorf("cannot output xml from xml input (input is already xml)")
		}
	case ".json":
		if format == "xml" || format == "json" {
			return fmt.Errorf("cannot output %s from json input (pipeline only moves forward)", format)
		}
	default:
		return fmt.Errorf("unsupported input file extension: %s (expected .pdf, .xml, or .json)", ext)
	}

	if format == "markdown" {
		fmt.Println("markdown output not yet implemented")
		return nil
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	// Step 1: PDF → XML
	var xmlPath string
	var xmlCleanup func()

	if ext == ".pdf" {
		var err error
		xmlPath, xmlCleanup, err = pdfToXML(inputPath, baseName, cacheDir)
		if err != nil {
			return err
		}
		if xmlCleanup != nil {
			defer xmlCleanup()
		}
	} else if ext == ".xml" {
		xmlPath = inputPath
	}

	// If format is xml, output the raw XML and stop.
	if format == "xml" {
		return outputFile(xmlPath)
	}

	// Step 2: XML → JSON (Document)
	var doc *model.Document

	if ext == ".json" {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("reading JSON file: %w", err)
		}
		doc = &model.Document{}
		if err := json.Unmarshal(data, doc); err != nil {
			return fmt.Errorf("parsing JSON file: %w", err)
		}
	} else {
		var err error
		doc, err = xmlToDocument(xmlPath, inputPath, excludeFontsStr, tocSource)
		if err != nil {
			return err
		}

		// Cache JSON if cache-dir is set.
		if cacheDir != "" {
			jsonPath := filepath.Join(cacheDir, baseName+".json")
			if err := writeJSONFile(doc, jsonPath); err != nil {
				return fmt.Errorf("caching JSON: %w", err)
			}
		}
	}

	// If format is json, output JSON and stop.
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		if pretty {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(doc)
	}

	// Step 3: JSON → HTML
	if format == "html" {
		return render.HTML(os.Stdout, doc)
	}

	return nil
}

// pdfToXML runs pdftohtml to convert a PDF to XML.
// If cacheDir is set, the XML is cached there and reused on subsequent runs.
func pdfToXML(pdfPath, baseName, cacheDir string) (string, func(), error) {
	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return "", nil, fmt.Errorf("creating cache dir: %w", err)
		}
		cachedXML := filepath.Join(cacheDir, baseName+".xml")

		// If cached XML already exists, use it directly.
		if _, err := os.Stat(cachedXML); err == nil {
			return cachedXML, nil, nil
		}

		// Run pdftohtml to temp, then copy to cache.
		tmpXML, tmpCleanup, err := extract.RunPdfToHTML(pdfPath, "")
		if err != nil {
			return "", nil, err
		}
		defer tmpCleanup()

		if err := copyFile(tmpXML, cachedXML); err != nil {
			return "", nil, fmt.Errorf("caching XML: %w", err)
		}
		return cachedXML, nil, nil
	}

	// No cache dir: use temp files with cleanup.
	return extract.RunPdfToHTML(pdfPath, "")
}

// xmlToDocument parses XML and applies the full extraction pipeline.
func xmlToDocument(xmlPath, inputPath, excludeFontsStr, tocSource string) (*model.Document, error) {
	excludeFonts := parseExcludeFonts(excludeFontsStr)

	doc, err := extract.ParseXML(xmlPath)
	if err != nil {
		return nil, err
	}

	doc.Source = filepath.Base(inputPath)
	extract.Clean(doc, excludeFonts)
	extract.AssignFontRoles(doc, excludeFonts)
	extract.ApplyRolesToElements(doc)
	extract.ResolveTOC(doc, tocSource)

	return doc, nil
}

func parseExcludeFonts(s string) map[string]bool {
	fonts := make(map[string]bool)
	if s != "" {
		for _, id := range strings.Split(s, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				fonts[id] = true
			}
		}
	}
	return fonts
}

func outputFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func writeJSONFile(doc *model.Document, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
