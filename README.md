# Upwind CLI

`upwind` is a Go 1.26.1 Cobra CLI for the Upwind Management APIs. It builds a versionless command tree from generated Go catalog data derived from the upstream OpenAPI v1 and v2 specifications, prefers the v2 definition when the same tag and operation exist in both versions, supports table and JSON output, loads `.env` automatically when present, and handles the pagination patterns described by the supplied specs.

## Requirements

- Go 1.26.1

## Build

```bash
go build -o upwind .
```

To print the embedded build metadata:

```bash
./upwind version
./upwind --version
```

Tagged release builds inject the tag, commit, and build date automatically.

## Refresh Generated API Catalog

This repository does not vendor the upstream OpenAPI YAML files. Instead, it generates [internal/openapi/catalog_generated.go](/Users/sardo/Projects/upwind-cli/internal/openapi/catalog_generated.go) from the sibling spec repository at `../spec-upwind`.

Regenerate the catalog after upstream spec changes with:

```bash
go generate ./internal/openapi
```

That command expects `../spec-upwind` to contain the upstream YAML files.

## Configuration

The CLI reads configuration from flags and environment variables. If a `.env` file exists in the current working directory, it is loaded automatically before flags are evaluated.

Supported environment variables:

- `UPWIND_ORGANIZATION_ID`
- `UPWIND_REGION` with `us`, `eu`, or `me`
- `UPWIND_CLIENT_ID`
- `UPWIND_CLIENT_SECRET`
- `UPWIND_ACCESS_TOKEN`
- `UPWIND_BASE_URL`
- `UPWIND_AUTH_URL`
- `UPWIND_AUDIENCE`
- `UPWIND_OUTPUT` with `table` or `json`
- `UPWIND_TIMEOUT` like `30s`

Client credentials use the OAuth client-credentials flow against `https://auth.upwind.io/oauth/token`. If `UPWIND_ACCESS_TOKEN` or `--access-token` is set, the CLI skips token acquisition and uses that bearer token directly.

The included `.env.example` is the quickest starting point:

```bash
cp .env.example .env
```

## Discover Commands

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats --help
```

The generated shape is:

- `upwind <tag> <operation>`

Examples:

- `upwind threats list-threat-detections`
- `upwind workflows list-all-workflows`
- `upwind inventory search-assets`
- `upwind threats list-stories`

## Output Modes

Use `--output table` for human-readable terminal output or `--output json` for raw JSON.

```bash
./upwind --output table threats list-threat-detections
./upwind --output json inventory get-asset --id uwr-example
```

For list responses, table mode renders rows. For object responses, table mode renders flattened key/value output.

## Request Bodies

Operations with JSON request bodies accept either:

- `--body '{"key":"value"}'`
- `--body-file request.json`
- `--body-file -` to read JSON from stdin

Examples:

```bash
./upwind inventory search-assets \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  --limit 50
```

```bash
./upwind threats update-threat-detection \
  --detection-id det_123 \
  --body '{"status":"ARCHIVED"}'
```

## Pagination

The CLI supports the pagination patterns present in the supplied specs:

- v1 page-based pagination with `page` and `per-page`
- v1 token-based pagination using `page-token` and `Link` response headers
- v2 cursor-based pagination using `cursor`, `limit`, and `metadata.next_cursor`

Use `--all` on paginated operations to fetch every page automatically.

Examples:

```bash
./upwind events search-shift-left-events --all --per-page 100 --body-file query.json
./upwind threats get-events-list --all --per-page 100
./upwind inventory search-assets --all --limit 200 --body-file query.json
```

Without `--all`, the CLI performs a single request and lets you control the page or cursor flags manually.

## How It Works

The CLI is intentionally thin. Almost all command structure comes from the generated catalog in [internal/openapi/catalog_generated.go](/Users/sardo/Projects/upwind-cli/internal/openapi/catalog_generated.go), which is produced from the upstream specs in `../spec-upwind`.

Startup flow:

1. `main.go` calls `cmd.Execute()`.
2. `cmd/root.go` delegates to `internal/app.NewRootCmd()`.
3. `internal/app` loads `.env`, loads the generated catalog, merges it into one tag-based command tree, and prefers the v2 definition when a tag and operation exist in both versions.

Runtime flow for an operation:

1. `config.Resolve()` merges flags and environment variables into a runtime config.
2. `auth.Provider` either reuses `UPWIND_ACCESS_TOKEN` or fetches an OAuth client-credentials token.
3. `internal/app` collects path and query values from the generated flags.
4. The request executor builds the final URL, sends the HTTP request, and decodes JSON responses.
5. If `--all` is set, the executor follows the pagination style inferred from the spec.
6. `internal/render` writes table output or pretty JSON.

Key packages:

- `internal/openapi`: parses the embedded OpenAPI files and derives tags, operations, parameters, body support, and pagination style
- `internal/app`: generates Cobra commands and executes HTTP requests
- `internal/auth`: implements bearer-token resolution and OAuth client-credentials exchange
- `internal/config`: handles environment loading, region defaults, and runtime config validation
- `internal/render`: renders JSON or flattened table output

## Testing

Run the normal validation set before changing behavior:

```bash
go test ./...
go vet ./...
go build -o upwind .
```

To validate the release configuration without publishing a GitHub Release:

```bash
go run github.com/goreleaser/goreleaser/v2@v2.14.3 check
go run github.com/goreleaser/goreleaser/v2@v2.14.3 release --snapshot --skip=publish --clean
```

## Releases

This repository uses GoReleaser for tagged releases. The workflow lives at [.github/workflows/release.yml](/Users/sardo/Projects/upwind-cli/.github/workflows/release.yml) and triggers when a Git tag matching `v*` is pushed.

On each matching tag, GitHub Actions:

- checks out the full git history
- installs the Go toolchain from `go.mod`
- runs `go test ./...`
- runs GoReleaser to build `upwind` for Linux, macOS, and Windows on `amd64` and `arm64`
- uploads the archives and `checksums.txt` asset to the GitHub Release for that tag

Create and publish a release tag with:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

The GoReleaser configuration is stored in [.goreleaser.yaml](/Users/sardo/Projects/upwind-cli/.goreleaser.yaml).

Useful smoke checks:

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats --help
```

## AI Skill

This repository includes an [Agent Skill](https://skills.sh) at `skills/upwind-cli/`. It teaches AI agents how to operate the CLI — authentication, command discovery, request bodies, pagination, output modes, and troubleshooting.

### Install with skills.sh (recommended)

The easiest way to install the skill into any supported agent (Claude Code, Cursor, Codex, OpenCode, Gemini CLI, and [40+ more](https://github.com/vercel-labs/skills#supported-agents)):

```bash
npx skills add oneslash/upwind-cli
```

This auto-detects which agents you have installed and symlinks the skill into each one.

To install into specific agents:

```bash
npx skills add oneslash/upwind-cli -a claude-code -a cursor -a codex
```

To install globally (available across all projects):

```bash
npx skills add oneslash/upwind-cli -g
```

### Project-level (automatic)

When you open this repository in an agent that supports project-level skills, the skill is discovered automatically:

- **Claude Code** — discovers `.claude/skills/upwind-cli/` (symlinked to `skills/upwind-cli/`)
- **Cursor** — discovers via `skills/` or `.agents/skills/`
- **Codex, OpenCode, Gemini CLI** — discovers via `skills/` directory

No installation needed — just open the project.

### Manual global install

If you prefer manual setup without `npx skills`:

```bash
# Claude Code
mkdir -p ~/.claude/skills
ln -snf "$(pwd)/skills/upwind-cli" ~/.claude/skills/upwind-cli

# Codex
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
ln -snf "$(pwd)/skills/upwind-cli" "${CODEX_HOME:-$HOME/.codex}/skills/upwind-cli"

# OpenCode
mkdir -p ~/.config/opencode/skills
ln -snf "$(pwd)/skills/upwind-cli" ~/.config/opencode/skills/upwind-cli

# Cursor
mkdir -p ~/.cursor/skills
ln -snf "$(pwd)/skills/upwind-cli" ~/.cursor/skills/upwind-cli
```

## Examples

List detections in table form:

```bash
./upwind threats list-threat-detections --severity HIGH
```

Fetch a single asset as JSON:

```bash
./upwind --output json inventory get-asset --id uwr-b7d8d158c28ab7ca281fd424311e9d19
```

Search stories with a request body from a file:

```bash
./upwind threats search-stories --body-file search.json --limit 100 --all
```
