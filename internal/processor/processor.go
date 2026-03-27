package processor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/VincentChalnot/pdf2md/internal/cache"
	"github.com/VincentChalnot/pdf2md/internal/extractor"
	"github.com/VincentChalnot/pdf2md/internal/llm"
	"github.com/VincentChalnot/pdf2md/internal/prompt"
)

// Config holds the configuration for the page processor.
type Config struct {
	PDFPath      string
	OutputFile   string
	PagesDir     string
	CacheStore   *cache.Store
	LLMClient    *llm.Client
	SystemPrompt string
	Workers      int
	ContextLines int
	FirstPage    int
	LastPage     int
	DryRun       bool
}

// Stats tracks processing statistics.
type Stats struct {
	mu         sync.Mutex
	Pages      int
	CacheHits  int
	LLMCalls   int
	TokensIn   int
	TokensOut  int
}

func (s *Stats) addCacheHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CacheHits++
}

func (s *Stats) addLLMCall(tokensIn, tokensOut int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LLMCalls++
	s.TokensIn += tokensIn
	s.TokensOut += tokensOut
}

// Run processes all pages and returns the assembled markdown content and stats.
func Run(ctx context.Context, cfg Config) (*Stats, error) {
	stats := &Stats{Pages: cfg.LastPage - cfg.FirstPage + 1}

	// Phase 1: Extract all pages and check cache (can be concurrent)
	type pageResult struct {
		pageNum  int
		content  string
		err      error
	}

	results := make([]string, stats.Pages)
	rawTexts := make([]string, stats.Pages)
	needsLLM := make([]bool, stats.Pages)

	// Extract text for all pages first
	sem := semaphore.NewWeighted(int64(cfg.Workers))
	var wg sync.WaitGroup
	var extractErr error
	var extractErrMu sync.Mutex

	for i := range stats.Pages {
		pageNum := cfg.FirstPage + i
		wg.Add(1)
		go func(idx, pn int) {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				extractErrMu.Lock()
				extractErr = err
				extractErrMu.Unlock()
				return
			}
			defer sem.Release(1)

			text, err := extractor.ExtractPage(cfg.PDFPath, pn)
			if err != nil {
				extractErrMu.Lock()
				extractErr = fmt.Errorf("extracting page %d: %w", pn, err)
				extractErrMu.Unlock()
				return
			}
			rawTexts[idx] = text

			if cfg.DryRun {
				results[idx] = text
				return
			}

			// Check cache
			if cached, ok := cfg.CacheStore.Read(pn); ok {
				results[idx] = cached
				stats.addCacheHit()
				return
			}
			needsLLM[idx] = true
		}(i, pageNum)
	}
	wg.Wait()

	if extractErr != nil {
		return nil, extractErr
	}

	if cfg.DryRun {
		return stats, writeOutput(cfg, results)
	}

	// Phase 2: Process pages sequentially for context, but LLM calls concurrent
	// We need context from previous pages, so we process in order but can
	// do LLM calls concurrently for pages that don't need context from
	// pages that are also being LLM-processed.
	// Simpler approach: process sequentially to maintain context chain.
	for i := range stats.Pages {
		pageNum := cfg.FirstPage + i

		if !needsLLM[i] {
			continue
		}

		// Build context from previous page
		var contextStr string
		if i > 0 && cfg.ContextLines > 0 {
			contextStr = lastNLines(results[i-1], cfg.ContextLines)
		}

		userPrompt, err := prompt.BuildUserPrompt(prompt.UserPromptData{
			Context:  contextStr,
			PageNum:  pageNum,
			PageText: rawTexts[i],
		})
		if err != nil {
			return nil, fmt.Errorf("building prompt for page %d: %w", pageNum, err)
		}

		result, err := cfg.LLMClient.Call(ctx, cfg.SystemPrompt, userPrompt)
		if err != nil {
			// On failure, insert raw text with comment
			results[i] = fmt.Sprintf("<!-- LLM FAILED page %d -->\n%s", pageNum, rawTexts[i])
			continue
		}

		stats.addLLMCall(result.TokensIn, result.TokensOut)
		results[i] = result.Content

		// Write to cache
		if err := cfg.CacheStore.Write(pageNum, result.Content); err != nil {
			// Cache write failure is non-fatal, just continue
			fmt.Fprintf(os.Stderr, "Warning: failed to write cache for page %d: %v\n", pageNum, err)
		}
	}

	return stats, writeOutput(cfg, results)
}

// RunConcurrent processes pages with bounded concurrency, using context from
// the previous page's result. Pages that need LLM calls are dispatched
// concurrently, but context injection uses assembled previous-page output.
func RunConcurrent(ctx context.Context, cfg Config) (*Stats, error) {
	stats := &Stats{Pages: cfg.LastPage - cfg.FirstPage + 1}

	results := make([]string, stats.Pages)
	rawTexts := make([]string, stats.Pages)

	// Phase 1: Extract all page texts concurrently
	sem := semaphore.NewWeighted(int64(cfg.Workers))
	var wg sync.WaitGroup
	errs := make([]error, stats.Pages)

	for i := range stats.Pages {
		pageNum := cfg.FirstPage + i
		wg.Add(1)
		go func(idx, pn int) {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				errs[idx] = err
				return
			}
			defer sem.Release(1)

			text, err := extractor.ExtractPage(cfg.PDFPath, pn)
			if err != nil {
				errs[idx] = fmt.Errorf("extracting page %d: %w", pn, err)
				return
			}
			rawTexts[idx] = text
		}(i, pageNum)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	if cfg.DryRun {
		copy(results, rawTexts)
		stats.Pages = len(results)
		return stats, writeOutput(cfg, results)
	}

	// Phase 2: Process pages sequentially (context depends on previous output)
	for i := range stats.Pages {
		pageNum := cfg.FirstPage + i

		// Check cache
		if cached, ok := cfg.CacheStore.Read(pageNum); ok {
			results[i] = cached
			stats.addCacheHit()
			continue
		}

		// Build context from previous page
		var contextStr string
		if i > 0 && cfg.ContextLines > 0 {
			contextStr = lastNLines(results[i-1], cfg.ContextLines)
		}

		userPrompt, err := prompt.BuildUserPrompt(prompt.UserPromptData{
			Context:  contextStr,
			PageNum:  pageNum,
			PageText: rawTexts[i],
		})
		if err != nil {
			return nil, fmt.Errorf("building prompt for page %d: %w", pageNum, err)
		}

		result, err := cfg.LLMClient.Call(ctx, cfg.SystemPrompt, userPrompt)
		if err != nil {
			results[i] = fmt.Sprintf("<!-- LLM FAILED page %d -->\n%s", pageNum, rawTexts[i])
			continue
		}

		stats.addLLMCall(result.TokensIn, result.TokensOut)
		results[i] = result.Content

		if err := cfg.CacheStore.Write(pageNum, result.Content); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write cache for page %d: %v\n", pageNum, err)
		}
	}

	return stats, writeOutput(cfg, results)
}

func writeOutput(cfg Config, results []string) error {
	// Write combined output
	combined := strings.Join(results, "\n")
	if err := os.MkdirAll(filepath.Dir(cfg.OutputFile), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(cfg.OutputFile, []byte(combined), 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	// Write per-page files if requested
	if cfg.PagesDir != "" {
		if err := os.MkdirAll(cfg.PagesDir, 0o755); err != nil {
			return fmt.Errorf("creating pages directory: %w", err)
		}
		for i, content := range results {
			pageNum := cfg.FirstPage + i
			filename := cache.PageFilename(pageNum)
			path := filepath.Join(cfg.PagesDir, filename)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing page file %s: %w", path, err)
			}
		}
	}

	return nil
}

// lastNLines returns the last n lines of the given text.
func lastNLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
