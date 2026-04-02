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
	format := flag.String("format", "markdown", "Output format: bbox, json, html, markdown")
	cacheDir := flag.String("cache-dir", "", "Directory for intermediate files (kept after run)")
	bboxCache := flag.String("bbox-cache", "", "Path to existing bbox-layout HTML file to use instead of running pdftotext")
	minTextHeight := flag.Float64("min-text-height", 2.0, "Minimum word height in PDF units to keep")
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
		if err := processFile(inputPath, *format, *cacheDir, *bboxCache, *minTextHeight, *pretty); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func processFile(inputPath, format, cacheDir, bboxCache string, minTextHeight float64, pretty bool) error {
	ext := strings.ToLower(filepath.Ext(inputPath))

	// Validate format value.
	switch format {
	case "bbox", "json", "html", "markdown":
	default:
		return fmt.Errorf("unsupported format: %s (must be bbox, json, html, or markdown)", format)
	}

	// Validate format compatibility with input type.
	switch ext {
	case ".pdf":
		// PDF can output any format.
	case ".bbox", ".html":
		if format == "bbox" {
			return fmt.Errorf("cannot output bbox from bbox input (input is already bbox)")
		}
	case ".json":
		if format == "bbox" || format == "json" {
			return fmt.Errorf("cannot output %s from json input (pipeline only moves forward)", format)
		}
	default:
		return fmt.Errorf("unsupported input file extension: %s (expected .pdf, .bbox, .html, or .json)", ext)
	}

	if format == "markdown" {
		fmt.Println("markdown output not yet implemented")
		return nil
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	// Step 1: PDF → bbox HTML
	var bboxPath string
	var bboxCleanup func()

	switch ext {
	case ".pdf":
		var err error
		bboxPath, bboxCleanup, err = pdfToBBox(inputPath, baseName, cacheDir, bboxCache)
		if err != nil {
			return err
		}
		if bboxCleanup != nil {
			defer bboxCleanup()
		}
	case ".bbox", ".html":
		bboxPath = inputPath
	}

	// If format is bbox, output the raw bbox HTML and stop.
	if format == "bbox" {
		return outputFile(bboxPath)
	}

	// Step 2: bbox HTML → JSON (Document)
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
		doc, err = bboxToDocument(bboxPath, inputPath, minTextHeight)
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

// pdfToBBox runs pdftotext to convert a PDF to bbox-layout HTML.
// If cacheDir is set, the HTML is cached there and reused on subsequent runs.
// If bboxCache is set, it is used directly instead of running pdftotext.
func pdfToBBox(pdfPath, baseName, cacheDir, bboxCache string) (string, func(), error) {
	// If bboxCache is explicitly provided via flag, use it.
	if bboxCache != "" {
		if _, err := os.Stat(bboxCache); err != nil {
			return "", nil, fmt.Errorf("bbox-cache file not found: %w", err)
		}
		return bboxCache, func() {}, nil
	}

	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return "", nil, fmt.Errorf("creating cache dir: %w", err)
		}
		cachedBBox := filepath.Join(cacheDir, baseName+".bbox")

		// If cached bbox already exists, use it directly.
		if _, err := os.Stat(cachedBBox); err == nil {
			return cachedBBox, nil, nil
		}

		// Run pdftotext to temp, then copy to cache.
		tmpBBox, tmpCleanup, err := extract.RunPdfToText(pdfPath, "")
		if err != nil {
			return "", nil, err
		}
		defer tmpCleanup()

		if err := copyFile(tmpBBox, cachedBBox); err != nil {
			return "", nil, fmt.Errorf("caching bbox: %w", err)
		}
		return cachedBBox, nil, nil
	}

	// No cache dir: use temp files with cleanup.
	return extract.RunPdfToText(pdfPath, "")
}

// bboxToDocument parses bbox HTML and applies the full extraction pipeline.
func bboxToDocument(bboxPath, inputPath string, minTextHeight float64) (*model.Document, error) {
	doc, err := extract.ParseBBoxHTML(bboxPath)
	if err != nil {
		return nil, err
	}

	doc.Source = filepath.Base(inputPath)
	extract.Clean(doc, minTextHeight)
	extract.AssignFontRoles(doc)
	extract.ApplyRolesToLines(doc)
	extract.EstablishReadingOrder(doc)

	return doc, nil
}

func outputFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	defer f.Close() //nolint:errcheck
	_, err = io.Copy(os.Stdout, f)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck

	_, err = io.Copy(out, in)
	return err
}

func writeJSONFile(doc *model.Document, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
