package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunPdfToHTML invokes pdftohtml to convert a PDF to XML.
// If xmlCache is non-empty, it is used as the output path and no temp file is created.
// Returns the path to the XML file and a cleanup function.
func RunPdfToHTML(pdfPath string, xmlCache string) (string, func(), error) {
	if xmlCache != "" {
		// Use the cached XML file directly; no cleanup needed.
		if _, err := os.Stat(xmlCache); err != nil {
			return "", nil, fmt.Errorf("xml-cache file not found: %w", err)
		}
		return xmlCache, func() {}, nil
	}

	tmpDir, err := os.MkdirTemp("", "pdf2md-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	xmlPath := filepath.Join(tmpDir, "output.xml")

	cmd := exec.Command("pdftohtml", "-xml", "-nodrm", "-i", "-noroundcoord", "-hidden", pdfPath, xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("pdftohtml failed: %w\noutput: %s", err, string(output))
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return xmlPath, cleanup, nil
}
