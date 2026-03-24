# Upwind CLI Usage Reference

Use this reference when the task is about operating the CLI. It contains detailed setup, flags, and command examples.

## Quick Start

Build the CLI:

```bash
go build -o upwind .
```

Create a local env file:

```bash
cp .env.example .env
```

Minimum config:

```bash
UPWIND_REGION=us
UPWIND_ORGANIZATION_ID=your-org-id
UPWIND_CLIENT_ID=your-client-id
UPWIND_CLIENT_SECRET=your-client-secret
```

The CLI loads `.env` automatically from the current working directory when present. Environment variables and flags override `.env` values.

## All Global Flags

All flags are persistent (available on every command). Each has an env var equivalent.

| Flag | Short | Env Var | Default | Notes |
|------|-------|---------|---------|-------|
| `--organization-id` | `-o` | `UPWIND_ORGANIZATION_ID` | — | **Required.** Injected into all API paths automatically. |
| `--region` | | `UPWIND_REGION` | `us` | `us`, `eu`, or `me` only. |
| `--output` | | `UPWIND_OUTPUT` | `table` | `table` or `json`. |
| `--timeout` | | `UPWIND_TIMEOUT` | `30s` | Any valid Go duration, e.g. `1m`. |
| `--client-id` | | `UPWIND_CLIENT_ID` | — | OAuth client credentials. |
| `--client-secret` | | `UPWIND_CLIENT_SECRET` | — | OAuth client credentials. |
| `--access-token` | | `UPWIND_ACCESS_TOKEN` | — | Bearer token. Skips OAuth if set. |
| `--base-url` | | `UPWIND_BASE_URL` | region-dependent | Override API base URL. |
| `--auth-url` | | `UPWIND_AUTH_URL` | `https://auth.upwind.io` | Override OAuth token endpoint host. |
| `--audience` | | `UPWIND_AUDIENCE` | same as base URL | OAuth audience claim. |

**Region base URLs:**
- `us` → `https://api.upwind.io`
- `eu` → `https://api.eu.upwind.io`
- `me` → `https://api.me.upwind.io`

## Authentication

### Option A — OAuth client credentials (default)

Set `UPWIND_CLIENT_ID` and `UPWIND_CLIENT_SECRET`. The CLI exchanges them for a bearer token via `POST {auth-url}/oauth/token` (grant type `client_credentials`). Tokens are cached in memory and reused until they have fewer than 30 seconds remaining.

### Option B — Bearer token (takes priority)

Set `UPWIND_ACCESS_TOKEN` or pass `--access-token`. No OAuth call is made. The value is sent as-is; `Bearer ` is prepended automatically if missing.

## Core Command Shape

```bash
upwind <tag> <operation> [flags]
```

Examples:
- `upwind threats list-threat-detections`
- `upwind inventory search-assets`
- `upwind workflows list-all-workflows`
- `upwind events search-shift-left-events`

## Discovery

Always start discovery with live help — command names and flags come from the embedded OpenAPI specs and only `--help` shows the real list:

```bash
./upwind --help                              # all tags
./upwind inventory --help                   # all operations under a tag
./upwind inventory search-assets --help     # flags and description for one operation
```

## Request Bodies

Operations that accept a JSON body expose three mutually exclusive input flags:

| Flag | Usage |
|------|-------|
| `--body '{"key":"value"}'` | Inline JSON string |
| `--body-file request.json` | Path to a JSON file |
| `--body-file -` | Read JSON from stdin |

Using both `--body` and `--body-file` in the same call is an error.

### Discovering the body schema

Run the operation's `--help` to read the OpenAPI summary/description, then consult the Upwind API docs for the full request schema. The CLI validates that the body is valid JSON but does not enforce the schema — the API returns a descriptive error if fields are wrong.

### Examples

Inline body:

```bash
./upwind inventory search-assets \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  --limit 50
```

From file:

```bash
./upwind threats search-stories --body-file search.json --limit 100 --all
```

From stdin:

```bash
cat query.json | ./upwind inventory search-assets --body-file - --limit 50
```

Update with inline body:

```bash
./upwind threats update-threat-detection \
  --detection-id det_123 \
  --body '{"status":"ARCHIVED"}'
```

## Output

### Table (default)

Human-readable. Nested fields are flattened with dot notation (e.g. `metadata.status`). Common fields (`id`, `name`, `severity`, `status`, …) are shown first; remaining fields are alphabetical.

```bash
./upwind threats list-threat-detections --severity HIGH
```

### JSON

Pretty-printed with 2-space indentation. Full API response structure is preserved. Best for scripting and `jq` pipelines.

```bash
./upwind --output json inventory get-asset --id uwr-example
```

```bash
./upwind --output json inventory search-assets \
  --body '{"conditions":[]}' | jq '.[].id'
```

## Pagination

The CLI auto-detects pagination style from the operation's flags. Three modes exist:

| Mode | Triggered by | `--all` behaviour |
|------|-------------|-------------------|
| Page-based | `--page` flag on operation | Increments `page` until response has fewer items than page size |
| Token / Link-header | `--page-token` flag | Follows `Link: <url>; rel="next"` response headers |
| Cursor-based | `--cursor` flag | Reads `metadata.next_cursor` from response |

Use `--all` to fetch every page automatically. Without it, a single request is made.

```bash
./upwind events search-shift-left-events --all --per-page 100 --body-file query.json
./upwind threats get-events-list --all --per-page 100
./upwind inventory search-assets --all --limit 200 --body-file query.json
```

**Pagination flags:**
- `--all` — enable automatic multi-page fetch
- `--per-page N` or `--limit N` — page size (which flag applies depends on the operation)
- `--page N`, `--page-token TOKEN`, `--cursor TOKEN` — manual page control (one applies per operation)

Paginated results are merged before rendering: array responses are appended; envelope responses with an `items` or `resourceFindings` key merge those arrays while preserving metadata from the last page.

## Shell Completion

```bash
./upwind completion bash       # bash
./upwind completion zsh        # zsh
./upwind completion fish       # fish
./upwind completion powershell # powershell
```

Load immediately in the current shell:

```bash
source <(./upwind completion zsh)
```

Install persistently on macOS (zsh + Homebrew):

```bash
./upwind completion zsh > $(brew --prefix)/share/zsh/site-functions/_upwind
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `missing organization ID` | Set `--organization-id` or `UPWIND_ORGANIZATION_ID` |
| `unsupported region 'xx'` | Use `us`, `eu`, or `me` |
| `missing credentials` | Set `UPWIND_CLIENT_ID`+`UPWIND_CLIENT_SECRET` or `UPWIND_ACCESS_TOKEN` |
| `oauth token request failed` | Check client ID/secret are correct and the org has API access |
| `use either --body or --body-file, not both` | Remove one of the two body flags |
| `invalid JSON request body` | Validate JSON syntax before passing it |
| `request failed: 4xx …` | Read the error body — the API includes a descriptive message |
| Output hard to parse in scripts | Switch to `--output json` and pipe through `jq` |
| Wrong environment targeted | Check `UPWIND_REGION` or pass `--region` |
| Unknown command or flag | Run `./upwind <tag> --help` — names are generated from OpenAPI and may differ from what you expect |
