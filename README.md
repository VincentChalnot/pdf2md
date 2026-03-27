# pdf2md

Convert PDF files into clean, structured JSON for downstream Markdown generation and RAG indexing.

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

### Extract

Convert a PDF to structured JSON:

```bash
# Output to stdout
pdf2md extract input.pdf

# Output to file, pretty-printed
pdf2md extract --pretty input.pdf output.json

# Exclude specific fonts and use cached XML
pdf2md extract --exclude-fonts "1,5" --xml-cache cached.xml input.pdf output.json
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--exclude-fonts` | Comma-separated font IDs to exclude | |
| `--toc-source` | TOC source: `auto`, `outline`, or `headings` | `auto` |
| `--xml-cache` | Path to an existing pdftohtml XML output | |
| `--pretty` | Pretty-print JSON output | `false` |

### Inspect

Launch a web UI to visually inspect the extraction results:

```bash
# From a PDF
pdf2md inspect input.pdf

# From a previously extracted JSON
pdf2md inspect output.json

# Custom port
pdf2md inspect --port 3000 input.pdf
```

**Routes:**

| Route | Description |
|-------|-------------|
| `/page/{n}` | HTML debug view of page n |
| `/page/{n}/raw` | JSON of page n elements |
| `/fonts` | Font table with roles and character counts |
| `/outline` | Table of contents view |

## License

See [LICENSE](LICENSE).
