package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/blake3"
)

// HashFile computes the BLAKE3 hash of the file at the given path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for hashing: %w", err)
	}
	defer f.Close()

	h := blake3.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// PageFilename returns the cache filename for a given page number.
func PageFilename(page int) string {
	return fmt.Sprintf("page_%04d.md", page)
}

// Store manages a per-PDF cache directory.
type Store struct {
	dir     string
	noCache bool
}

// NewStore creates a new cache store rooted at cacheDir/hash.
func NewStore(cacheDir, hash string, noCache bool) (*Store, error) {
	dir := filepath.Join(cacheDir, hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	return &Store{dir: dir, noCache: noCache}, nil
}

// Read returns the cached content for a page, or empty string if not cached.
func (s *Store) Read(page int) (string, bool) {
	if s.noCache {
		return "", false
	}
	path := filepath.Join(s.dir, PageFilename(page))
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// Write stores the content for a page in the cache.
func (s *Store) Write(page int, content string) error {
	path := filepath.Join(s.dir, PageFilename(page))
	return os.WriteFile(path, []byte(content), 0o644)
}
