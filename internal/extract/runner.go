package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunPdfToText invokes pdftotext to convert a PDF to bbox-layout HTML.
// If bboxCache is non-empty, it is used as the output path and no temp file is created.
// Returns the path to the HTML file and a cleanup function.
func RunPdfToText(pdfPath string, bboxCache string) (string, func(), error) {
	if bboxCache != "" {
		// Use the cached bbox file directly; no cleanup needed.
		if _, err := os.Stat(bboxCache); err != nil {
			return "", nil, fmt.Errorf("bbox-cache file not found: %w", err)
		}
		return bboxCache, func() {}, nil
	}

	tmpDir, err := os.MkdirTemp("", "pdf2md-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	htmlPath := filepath.Join(tmpDir, "output.html")

	cmd := exec.Command("pdftotext", "-bbox-layout", pdfPath, htmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("pdftotext failed: %w\noutput: %s", err, string(output))
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return htmlPath, cleanup, nil
}
