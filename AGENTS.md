# Agent Instructions: Building the App with Mise

This project uses [`mise`](https://mise.jdx.dev/) (previously known as `rtx`) to manage toolchains (like Go, Buf, and golangci-lint) and run project tasks.

## Build Steps

To build the application, follow these steps:

1. **Trust the configuration file:**
   Before running any tasks, you may need to explicitly trust the `mise.toml` configuration file in the project repository.
   ```bash
   mise trust
   ```

2. **Run the build task:**
   Execute the build task defined in `mise.toml`. This will automatically install the correct versions of the required tools (like Go 1.25.0) if they are missing, and then compile the project.
   ```bash
   mise run build
   ```

### What happens during the build?

Running `mise run build` triggers the `build` task, which executes the following command under the hood:

```bash
go build -o bin/pdf2md ./cmd/pdf2md/
```

The resulting binary will be placed at `bin/pdf2md`.

## Other Available Tasks

You can also run other tasks defined in the project:
- **Lint the code:** `mise run lint` (runs `golangci-lint`)
- **Run tests:** `mise run test` (runs `CGO_ENABLED=1 go test ./...`)
- **Build Docker image:** `mise run docker-build` (builds the `pdf2md` image)
