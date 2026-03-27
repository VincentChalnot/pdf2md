package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VincentChalnot/pdf2md/internal/cache"
	"github.com/VincentChalnot/pdf2md/internal/extractor"
	"github.com/VincentChalnot/pdf2md/internal/llm"
	"github.com/VincentChalnot/pdf2md/internal/processor"
	"github.com/VincentChalnot/pdf2md/internal/prompt"
)

var (
	output       string
	apiKey       string
	model        string
	baseURL      string
	workers      int
	contextLines int
	firstPage    int
	lastPage     int
	cacheDir     string
	noCache      bool
	pagesDir     string
	dryRun       bool
	systemPrompt string
)

var rootCmd = &cobra.Command{
	Use:   "pdf2md [flags] input.pdf",
	Short: "Convert PDF to Markdown via pdftotext + LLM",
	Long: `pdf2md converts any text-based PDF into clean Markdown by combining
pdftotext -layout for extraction and an OpenAI-compatible LLM for
structural reconstruction.`,
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	rootCmd.Flags().StringVar(&output, "output", "", "Output file (default: input basename + .md)")
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "API key (or env OPENROUTER_API_KEY)")
	rootCmd.Flags().StringVar(&model, "model", "google/gemini-2.5-flash-lite", "LLM model")
	rootCmd.Flags().StringVar(&baseURL, "base-url", "https://openrouter.ai/api/v1", "API base URL")
	rootCmd.Flags().IntVar(&workers, "workers", 4, "Max concurrent pages")
	rootCmd.Flags().IntVar(&contextLines, "context-lines", 5, "Lines of context from previous page")
	rootCmd.Flags().IntVar(&firstPage, "first-page", 1, "First page to process")
	rootCmd.Flags().IntVar(&lastPage, "last-page", 0, "Last page to process (default: all)")
	rootCmd.Flags().StringVar(&cacheDir, "cache-dir", "/tmp/pdf2md", "Cache root directory")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable cache, always call LLM")
	rootCmd.Flags().StringVar(&pagesDir, "pages-dir", "", "Write each page as a separate .md file here")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Extract text without calling the LLM")
	rootCmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Override the default system prompt (path to file or inline string)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runConvert(cmd *cobra.Command, args []string) error {
	pdfPath := args[0]

	// Verify pdftotext is available
	if err := extractor.CheckPdftotext(); err != nil {
		return err
	}

	// Verify PDF file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", pdfPath)
	}

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("OPENROUTER_API_KEY")
	}
	if key == "" && !dryRun {
		return fmt.Errorf("API key required: use --api-key or set OPENROUTER_API_KEY environment variable")
	}

	// Resolve output file
	outFile := output
	if outFile == "" {
		base := filepath.Base(pdfPath)
		ext := filepath.Ext(base)
		outFile = strings.TrimSuffix(base, ext) + ".md"
	}

	// Get page count
	totalPages, err := extractor.PageCount(pdfPath)
	if err != nil {
		return fmt.Errorf("determining page count: %w", err)
	}

	// Resolve page range
	fp := firstPage
	if fp < 1 {
		fp = 1
	}
	lp := lastPage
	if lp <= 0 || lp > totalPages {
		lp = totalPages
	}
	if fp > lp {
		return fmt.Errorf("first-page (%d) is greater than last-page (%d)", fp, lp)
	}

	// Compute hash and set up cache
	hash, err := cache.HashFile(pdfPath)
	if err != nil {
		return fmt.Errorf("hashing input file: %w", err)
	}
	cacheStore, err := cache.NewStore(cacheDir, hash, noCache)
	if err != nil {
		return fmt.Errorf("initializing cache: %w", err)
	}

	// Resolve system prompt
	sysPrompt := prompt.DefaultSystemPrompt
	if systemPrompt != "" {
		// Check if it's a file path
		if data, err := os.ReadFile(systemPrompt); err == nil {
			sysPrompt = string(data)
		} else {
			// Treat as inline string
			sysPrompt = systemPrompt
		}
	}

	// Create LLM client (only needed if not dry-run)
	var client *llm.Client
	if !dryRun {
		client = llm.NewClient(key, baseURL, model)
	}

	cfg := processor.Config{
		PDFPath:      pdfPath,
		OutputFile:   outFile,
		PagesDir:     pagesDir,
		CacheStore:   cacheStore,
		LLMClient:    client,
		SystemPrompt: sysPrompt,
		Workers:      workers,
		ContextLines: contextLines,
		FirstPage:    fp,
		LastPage:     lp,
		DryRun:       dryRun,
	}

	stats, err := processor.RunConcurrent(context.Background(), cfg)
	if err != nil {
		return err
	}

	// Print cost report
	fmt.Printf("Pages processed : %d\n", stats.Pages)
	fmt.Printf("Cache hits      : %d\n", stats.CacheHits)
	fmt.Printf("LLM calls       : %d\n", stats.LLMCalls)
	if stats.TokensIn > 0 || stats.TokensOut > 0 {
		fmt.Printf("Tokens in       : %s\n", formatNumber(stats.TokensIn))
		fmt.Printf("Tokens out      : %s\n", formatNumber(stats.TokensOut))
	}

	return nil
}

// formatNumber formats an integer with comma separators.
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
