package extractor

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CheckPdftotext verifies that pdftotext is available on PATH.
func CheckPdftotext() error {
	_, err := exec.LookPath("pdftotext")
	if err != nil {
		return fmt.Errorf("pdftotext not found on PATH; install poppler-utils:\n" +
			"  Debian/Ubuntu: sudo apt-get install poppler-utils\n" +
			"  macOS:         brew install poppler\n" +
			"  Fedora:        sudo dnf install poppler-utils")
	}
	return nil
}

var pageCountRe = regexp.MustCompile(`Pages:\s+(\d+)`)

// PageCount returns the total number of pages in the PDF.
func PageCount(pdfPath string) (int, error) {
	out, err := exec.Command("pdfinfo", pdfPath).Output()
	if err != nil {
		// Fallback: try to extract pages by running pdftotext on the whole document
		// and counting form-feeds (less reliable).
		return pageCountFallback(pdfPath)
	}
	matches := pageCountRe.FindSubmatch(out)
	if matches == nil {
		return 0, fmt.Errorf("could not determine page count from pdfinfo output")
	}
	n, err := strconv.Atoi(string(matches[1]))
	if err != nil {
		return 0, fmt.Errorf("invalid page count: %w", err)
	}
	return n, nil
}

// pageCountFallback counts pages by extracting all text and counting form-feeds.
func pageCountFallback(pdfPath string) (int, error) {
	out, err := exec.Command("pdftotext", "-layout", pdfPath, "-").Output()
	if err != nil {
		return 0, fmt.Errorf("pdftotext failed: %w", err)
	}
	// Each page is separated by a form-feed character (\f).
	// Number of pages = number of form-feeds + 1 (if there's content).
	text := string(out)
	if len(strings.TrimSpace(text)) == 0 {
		return 0, nil
	}
	return strings.Count(text, "\f") + 1, nil
}

// ExtractPage extracts text from a single page of the PDF using pdftotext.
func ExtractPage(pdfPath string, page int) (string, error) {
	pageStr := strconv.Itoa(page)
	cmd := exec.Command("pdftotext", "-layout", "-f", pageStr, "-l", pageStr, pdfPath, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext page %d: %w", page, err)
	}
	return string(out), nil
}
