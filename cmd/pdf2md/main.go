package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/pdf2md/internal/extract"
	"github.com/user/pdf2md/internal/inspect"
	"github.com/user/pdf2md/internal/model"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "extract":
		if err := runExtract(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "inspect":
		if err := runInspect(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: pdf2md <command> [flags] <input>

Commands:
  extract    Convert a PDF to structured JSON
  inspect    Launch a web UI to inspect a PDF or JSON file

Run 'pdf2md <command> -h' for command-specific help.
`)
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	excludeFonts := fs.String("exclude-fonts", "", "Comma-separated font IDs to exclude")
	tocSource := fs.String("toc-source", "auto", `TOC source: "auto", "outline", or "headings"`)
	xmlCache := fs.String("xml-cache", "", "Path to existing pdftohtml XML output (skip extraction)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: pdf2md extract [flags] <input.pdf> [output.json]")
	}

	inputPath := fs.Arg(0)
	outputPath := fs.Arg(1) // may be empty

	if strings.HasSuffix(strings.ToLower(inputPath), ".json") {
		return fmt.Errorf("input file has .json extension — did you mean to use 'inspect'?")
	}

	doc, err := extractDocument(inputPath, *excludeFonts, *tocSource, *xmlCache)
	if err != nil {
		return err
	}

	var out *os.File
	if outputPath != "" {
		out, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	enc := json.NewEncoder(out)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(doc)
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP port")
	excludeFonts := fs.String("exclude-fonts", "", "Comma-separated font IDs to exclude (PDF input only)")
	tocSource := fs.String("toc-source", "auto", `TOC source: "auto", "outline", or "headings" (PDF input only)`)
	xmlCache := fs.String("xml-cache", "", "Path to existing pdftohtml XML output (PDF input only)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: pdf2md inspect [flags] <input.pdf|input.json>")
	}

	inputPath := fs.Arg(0)

	var doc *model.Document

	if strings.HasSuffix(strings.ToLower(inputPath), ".json") {
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
		doc, err = extractDocument(inputPath, *excludeFonts, *tocSource, *xmlCache)
		if err != nil {
			return err
		}
	}

	srv, err := inspect.NewServer(doc, *port)
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

func extractDocument(pdfPath, excludeFontsStr, tocSource, xmlCache string) (*model.Document, error) {
	// Parse exclude fonts.
	excludeFonts := make(map[string]bool)
	if excludeFontsStr != "" {
		for _, id := range strings.Split(excludeFontsStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				excludeFonts[id] = true
			}
		}
	}

	// Run pdftohtml or use cached XML.
	xmlPath, cleanup, err := extract.RunPdfToHTML(pdfPath, xmlCache)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Parse XML.
	doc, err := extract.ParseXML(xmlPath)
	if err != nil {
		return nil, err
	}

	doc.Source = filepath.Base(pdfPath)

	// Clean pipeline.
	extract.Clean(doc, excludeFonts)

	// Assign font roles.
	extract.AssignFontRoles(doc, excludeFonts)

	// Apply roles to elements.
	extract.ApplyRolesToElements(doc)

	// Resolve TOC.
	extract.ResolveTOC(doc, tocSource)

	return doc, nil
}
