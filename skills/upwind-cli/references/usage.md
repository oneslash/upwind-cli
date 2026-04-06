# Upwind CLI Usage Reference

Use this reference when the task is about operating the CLI. It contains detailed setup, flags, command examples, and troubleshooting.

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

**Auth URL:** `https://auth.upwind.io` is the default for all regions.

## Authentication

### Option A — OAuth client credentials (default)

Set `UPWIND_CLIENT_ID` and `UPWIND_CLIENT_SECRET`. The CLI exchanges them for a bearer token via `POST {auth-url}/oauth/token` (grant type `client_credentials`). Tokens are cached in memory and reused until they have fewer than 30 seconds remaining.

### Option B — Bearer token (takes priority)

Set `UPWIND_ACCESS_TOKEN` or pass `--access-token`. No OAuth call is made. The value is sent as-is; `Bearer ` is prepended automatically if missing.

### Priority rules

1. If `--access-token` / `UPWIND_ACCESS_TOKEN` is set (even if client credentials are also set), OAuth is skipped entirely — the token is used as-is.
2. Otherwise, `--client-id` + `--client-secret` are required together. Missing either one produces: `missing credentials: set UPWIND_CLIENT_ID/UPWIND_CLIENT_SECRET or UPWIND_ACCESS_TOKEN`.
3. Tokens from OAuth are cached in memory for the lifetime of the process. A new token is fetched only when the cached one has fewer than 30 seconds until expiry. If the token response does not include `expires_in`, the CLI assumes a 5-minute lifetime.

## Core Command Shape

```bash
upwind <tag> <operation> [flags]
```

Examples:
- `upwind threats list-threat-detections`
- `upwind inventory search-assets`
- `upwind workflows list-all-workflows`
- `upwind events search-shift-left-events`

### HTTP method patterns

Operations follow naming conventions that hint at the underlying HTTP method and whether a body is needed:

| Operation name pattern | HTTP method | Accepts body? | Body required? |
|------------------------|-------------|---------------|----------------|
| `list-*`, `get-*` | GET | No | No |
| `search-*` | POST | Yes | Usually yes |
| `create-*`, `add-*` | POST | Yes | Yes |
| `update-*` | PUT or PATCH | Yes | Yes |
| `delete-*`, `remove-*` | DELETE | Usually no | No |

These are conventions — always verify with `--help`. If `--body` and `--body-file` appear in the flags, the operation accepts a body. If a body is required but not provided, the CLI immediately errors with: `this operation requires a JSON request body`.

## Discovery

Always start discovery with live help — command names and flags come from the embedded OpenAPI specs and only `--help` shows the real list:

```bash
./upwind --help                              # all tags
./upwind inventory --help                   # all operations under a tag
./upwind inventory search-assets --help     # flags and description for one operation
```

The `--help` output for each operation includes:
- The operation's summary and description from the OpenAPI spec
- All available flags with types, defaults, and allowed values
- Whether `--body`/`--body-file` flags are present (meaning the operation accepts a JSON body)
- Whether `--all` is present (meaning the operation supports pagination)

## Request Bodies

Operations that accept a JSON body expose three mutually exclusive input flags:

| Flag | Usage |
|------|-------|
| `--body '{"key":"value"}'` | Inline JSON string |
| `--body-file request.json` | Path to a JSON file |
| `--body-file -` | Read JSON from stdin |

Using both `--body` and `--body-file` in the same call is an error.

### Body validation

The CLI validates that the body is **valid JSON** before sending the request. It does **not** enforce the API schema — if fields are wrong or missing, the API returns a descriptive error (typically a 400 with a JSON body explaining the problem). This means:

- Syntax errors in the JSON (missing quotes, trailing commas) are caught locally.
- Schema errors (wrong field names, wrong types, missing required fields) come back as server errors.

### Discovering the body schema

Run the operation's `--help` to read the OpenAPI summary/description. For the full request schema, consult the Upwind API docs. When building bodies from scratch, the `search-*` operations typically accept a `conditions` array with objects containing `field`, `operator`, and `value` keys.

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

Array responses are rendered as row sets. Envelope responses with an `items` or `resourceFindings` key render that array as rows. Single-object responses are rendered as a two-column field/value table with flattened keys sorted alphabetically.

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

When `--all` is combined with `--body`, the same body is re-sent on every page request.

## Common Workflows

### Investigate threats end-to-end

```bash
# 1. List high-severity detections in table form
./upwind threats list-threat-detections --severity HIGH

# 2. Get full JSON details on one detection
./upwind --output json threats get-threat-detection --detection-id det_abc123

# 3. Archive it after investigation
./upwind threats update-threat-detection \
  --detection-id det_abc123 \
  --body '{"status":"ARCHIVED"}'
```

### Search and export assets

```bash
# 1. Search for compute assets
./upwind inventory search-assets \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  --limit 50

# 2. Get full details on a specific asset
./upwind --output json inventory get-asset --id uwr-b7d8d158c28ab7ca281fd424311e9d19

# 3. Fetch ALL matching assets for a report
./upwind --output json inventory search-assets --all --limit 200 \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  > compute_assets.json
```

### Scripting with jq

```bash
# Extract all asset IDs
./upwind --output json inventory search-assets --all --limit 200 \
  --body '{"conditions":[]}' | jq -r '.[].id'

# Count threats by severity
./upwind --output json threats list-threat-detections --all --per-page 100 \
  | jq 'group_by(.severity) | map({severity: .[0].severity, count: length})'

# Pipe a body from another command
echo '{"conditions":[]}' | ./upwind inventory search-assets --body-file - --limit 10
```

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

## Error Reference

### Local errors (before any HTTP request)

These errors are raised by the CLI itself before contacting the API:

| Error | Cause |
|-------|-------|
| `missing organization ID: set --organization-id or UPWIND_ORGANIZATION_ID` | No org ID configured |
| `unsupported region "xx" (expected us, eu, or me)` | Invalid `--region` value |
| `unsupported output format "xx" (expected table or json)` | Invalid `--output` value |
| `missing credentials: set UPWIND_CLIENT_ID/UPWIND_CLIENT_SECRET or UPWIND_ACCESS_TOKEN` | No auth configured |
| `use either --body or --body-file, not both` | Conflicting body flags |
| `invalid JSON request body: <parse error>` | Malformed JSON syntax |
| `this operation requires a JSON request body` | Required body not provided |
| `missing required path parameter --<flag>` | Required path param missing |

### OAuth errors

| Error | Cause |
|-------|-------|
| `oauth token request failed: <error> (<description>)` | Server returned an OAuth error (bad credentials, no access) |
| `oauth token request failed with status <status>: <body>` | Non-200 response without standard OAuth error fields |
| `oauth token response did not include access_token` | Token endpoint returned 200 but no token in body |

### HTTP errors (API responses)

HTTP errors (status >= 400) are formatted as:

```
request failed: <status>
<pretty-printed JSON body>
```

For example:

```
request failed: 400 Bad Request
{
  "message": "invalid filter field: unknown_field",
  "code": "INVALID_REQUEST"
}
```

If the response body is not valid JSON, it is printed as-is. If the body is empty, only the status line appears.

## Troubleshooting Quick Reference

| Symptom | Fix |
|---------|-----|
| Wrong environment targeted | Check `UPWIND_REGION` or pass `--region` |
| Output hard to parse in scripts | Switch to `--output json` and pipe through `jq` |
| Unknown command or flag | Run `./upwind <tag> --help` — names are generated from OpenAPI and may differ from what you expect |
| Request returns empty result | Try `--output json` to see the full response; the table renderer skips nil values |
| Pagination seems incomplete | Add `--all` to fetch every page; without it only one page is returned |
| Body schema unclear | Run `--help` on the operation for the description, then consult Upwind API docs for the full schema |
