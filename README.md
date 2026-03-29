# pdf2md

Convert PDF files into clean, structured output for downstream Markdown generation and RAG indexing.

## Installation

```bash
go build -o pdf2md ./cmd/pdf2md
```

Or with Docker:

```bash
docker build -t pdf2md .
```

## Requirements

- [pdftohtml](https://poppler.freedesktop.org/) (from poppler-utils) must be installed and on PATH.

## Usage

```bash
pdf2md [flags] <filepath>...
```

Input format is detected from file extension: `.pdf`, `.xml`, `.json`.

The pipeline is: PDF → XML → JSON → HTML → Markdown. The `--format` flag stops the pipeline
at the requested step and outputs that intermediate format.

### Examples

```bash
# Convert XML to JSON (pretty-printed)
pdf2md --format json --pretty input.xml

# Convert XML to HTML (SVG-based visualization)
pdf2md --format html input.xml > output.html

# Convert PDF to JSON with font exclusions
pdf2md --format json --exclude-fonts "1,5" input.pdf

# Convert PDF to JSON, caching intermediates
pdf2md --format json --cache-dir ./cache input.pdf

# Process multiple files
pdf2md --format html file1.xml file2.xml
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--format` | Output format: `xml`, `json`, `html`, `markdown` | `markdown` |
| `--cache-dir` | Directory for intermediate files (kept after run) | |
| `--exclude-fonts` | Comma-separated font IDs to exclude | |
| `--toc-source` | TOC source: `auto`, `outline`, or `headings` | `auto` |
| `--pretty` | Pretty-print JSON output | `false` |

### Format compatibility

- Input `.pdf` → can output `xml`, `json`, `html`, `markdown`
- Input `.xml` → can output `json`, `html`, `markdown`
- Input `.json` → can output `html`, `markdown`

## License

See [LICENSE](LICENSE).
