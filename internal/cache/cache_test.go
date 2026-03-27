package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash1, err := HashFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 == "" {
		t.Error("hash should not be empty")
	}

	// Same content should produce same hash
	hash2, err := HashFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("hash mismatch: %s != %s", hash1, hash2)
	}

	// Different content should produce different hash
	path2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(path2, []byte("different content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	hash3, err := HashFile(path2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 == hash3 {
		t.Error("different files should produce different hashes")
	}
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file.pdf")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPageFilename(t *testing.T) {
	tests := []struct {
		page     int
		expected string
	}{
		{1, "page_0001.md"},
		{10, "page_0010.md"},
		{100, "page_0100.md"},
		{9999, "page_9999.md"},
	}
	for _, tt := range tests {
		result := PageFilename(tt.page)
		if result != tt.expected {
			t.Errorf("PageFilename(%d) = %s, want %s", tt.page, result, tt.expected)
		}
	}
}

func TestStore_ReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir, "testhash", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reading non-existent page should return false
	_, ok := store.Read(1)
	if ok {
		t.Error("expected cache miss for non-existent page")
	}

	// Write and read back
	content := "# Test Markdown\n\nSome content here."
	if err := store.Write(1, content); err != nil {
		t.Fatalf("unexpected error writing cache: %v", err)
	}

	got, ok := store.Read(1)
	if !ok {
		t.Error("expected cache hit after write")
	}
	if got != content {
		t.Errorf("cache content mismatch: got %q, want %q", got, content)
	}
}

func TestStore_NoCache(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir, "testhash", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write content
	if err := store.Write(1, "content"); err != nil {
		t.Fatalf("unexpected error writing cache: %v", err)
	}

	// Reading with noCache should always miss
	_, ok := store.Read(1)
	if ok {
		t.Error("expected cache miss with noCache enabled")
	}
}

func TestStore_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir, "testhash", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write empty content
	if err := store.Write(1, ""); err != nil {
		t.Fatalf("unexpected error writing cache: %v", err)
	}

	// Empty file should be treated as cache miss
	_, ok := store.Read(1)
	if ok {
		t.Error("expected cache miss for empty content")
	}
}
