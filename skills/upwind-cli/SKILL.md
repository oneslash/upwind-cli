---
name: upwind-cli
description: Use when helping someone configure or operate the Upwind CLI application. Covers setup, authentication, environment variables, command discovery, request bodies, output modes, pagination, and shell completion.
---

# Upwind CLI

## What is Upwind

Upwind is a **cloud security platform** providing APIs for asset inventory, threat detection, vulnerability management, API security, compliance, and shift-left event tracking across AWS, Azure, and GCP. This CLI is a Go tool that exposes those APIs as a command tree auto-generated from embedded OpenAPI specs.

## When To Use This Skill

Use this skill when the user wants help **operating** the `upwind` binary:

- first-time setup and `.env` configuration
- authentication (OAuth client credentials or bearer token)
- region or organization selection
- finding the right command or flags via `--help`
- listing, searching, fetching, or updating resources
- passing JSON request bodies (`--body`, `--body-file`)
- fetching all pages with `--all`
- choosing `table` vs `json` output
- enabling shell completion
- interpreting errors or troubleshooting failed requests

To modify Go source code, explain internals, or change generated behavior → read [references/architecture.md](references/architecture.md).

## How To Help

Read [references/usage.md](references/usage.md) for the full reference. Use this workflow:

1. **Discover commands live** — prefer `./upwind --help` and `./upwind <tag> <operation> --help` over guessing. All command names, flags, and defaults come from the help output.
2. **Give runnable commands**, not abstract descriptions.
3. **Always surface auth requirements** — `--organization-id` is required for every call; `--client-id`/`--client-secret` or `--access-token` are required for auth.
4. **Match output to context** — `--output table` for human reading, `--output json` for scripting or jq pipelines.
5. **Mention `--all` when results may be paginated** — the CLI supports page, token (Link-header), and cursor pagination; `--all` handles all three automatically.
6. **Discover request body shape** — run `./upwind <tag> <operation> --help` and read the OpenAPI description, or point the user to the Upwind API docs. The CLI accepts `--body '{...}'`, `--body-file request.json`, or `--body-file -` (stdin).
7. **Know which operations need a body** — GET operations (list, get) never need one. POST operations (search, create) typically require one; some mark it as required, meaning the CLI errors without it. PUT/PATCH operations (update) need one for the fields being changed. Run `--help` to confirm: if `--body` and `--body-file` flags appear, the operation accepts a body.
8. **Show error handling** — when suggesting commands, tell the user what common errors look like and how to fix them. See the Gotchas and Error Reference sections below.

## Key Facts (quick reference)

### Command shape
```
upwind <tag> <operation> [flags]
```
Examples: `upwind threats list-threat-detections`, `upwind inventory search-assets`

### HTTP method patterns

| Pattern | Method | Body? | Example operations |
|---------|--------|-------|--------------------|
| `list-*`, `get-*` | GET | No | `list-threat-detections`, `get-asset` |
| `search-*` | POST | Yes (usually required) | `search-assets`, `search-stories` |
| `create-*`, `add-*` | POST | Yes (required) | `create-workflow` |
| `update-*` | PUT/PATCH | Yes (required) | `update-threat-detection` |
| `delete-*`, `remove-*` | DELETE | No (usually) | `delete-workflow` |

These are conventions from the upstream API — always verify with `--help` since individual operations may differ.

### Required config (every invocation)
| What | Flag | Env var |
|------|------|---------|
| Organization ID | `--organization-id` | `UPWIND_ORGANIZATION_ID` |
| Auth (option A) | `--client-id` + `--client-secret` | `UPWIND_CLIENT_ID` + `UPWIND_CLIENT_SECRET` |
| Auth (option B) | `--access-token` | `UPWIND_ACCESS_TOKEN` |

### Common optional flags (all have env var equivalents)
| Flag | Env var | Default |
|------|---------|---------|
| `--region` | `UPWIND_REGION` | `us` (also: `eu`, `me`) |
| `--output` | `UPWIND_OUTPUT` | `table` (also: `json`) |
| `--timeout` | `UPWIND_TIMEOUT` | `30s` |
| `--base-url` | `UPWIND_BASE_URL` | region-dependent |

### Pagination flags (operation-specific)
- `--all` — fetch every page automatically
- `--per-page` / `--limit` — page size
- `--page`, `--page-token`, `--cursor` — manual page control (only one applies per operation depending on API version)

### Body flags
- `--body '{"key":"value"}'` — inline JSON
- `--body-file request.json` — from file
- `--body-file -` — from stdin
- Cannot use `--body` and `--body-file` together

### Output
- `table` — human-readable, nested fields flattened with dot notation (e.g. `metadata.status`)
- `json` — pretty-printed, preserves full API response structure

## Common Workflows

These multi-step examples show how commands chain together in real usage.

### Investigate a threat detection

```bash
# List high-severity detections
./upwind threats list-threat-detections --severity HIGH

# Get details on a specific detection (use an ID from the list output)
./upwind --output json threats get-threat-detection --detection-id det_abc123

# Archive it
./upwind threats update-threat-detection \
  --detection-id det_abc123 \
  --body '{"status":"ARCHIVED"}'
```

### Search assets and drill down

```bash
# Search for compute assets
./upwind inventory search-assets \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  --limit 50

# Get full details on a specific asset as JSON
./upwind --output json inventory get-asset --id uwr-b7d8d158c28ab7ca281fd424311e9d19

# Fetch all matching assets across pages
./upwind inventory search-assets --all --limit 200 \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}'
```

### Export data for scripting

```bash
# Get all shift-left events as JSON and extract IDs with jq
./upwind --output json events search-shift-left-events --all --per-page 100 \
  --body-file query.json | jq '.[].id'

# Pipe a body from another command
echo '{"conditions":[]}' | ./upwind inventory search-assets --body-file - --limit 10
```

## Error Reference

These are the exact error strings the CLI produces and what causes them.

| Error message | Cause | Fix |
|---------------|-------|-----|
| `missing organization ID: set --organization-id or UPWIND_ORGANIZATION_ID` | No org ID provided | Set the flag or env var |
| `unsupported region "xx" (expected us, eu, or me)` | Invalid region value | Use `us`, `eu`, or `me` |
| `missing credentials: set UPWIND_CLIENT_ID/UPWIND_CLIENT_SECRET or UPWIND_ACCESS_TOKEN` | No auth configured | Provide client credentials or a token |
| `oauth token request failed: <error> (<description>)` | Bad client ID/secret or no API access | Verify credentials with the Upwind console |
| `use either --body or --body-file, not both` | Both body flags set | Remove one |
| `invalid JSON request body: <parse error>` | Malformed JSON in `--body` or `--body-file` | Validate the JSON before passing it |
| `this operation requires a JSON request body` | POST/PUT with `BodyRequired` but no body given | Add `--body` or `--body-file` |
| `missing required path parameter --<flag>` | A required path parameter was not provided | Add the missing flag (e.g. `--detection-id`) |
| `request failed: 400 Bad Request\n{...}` | Server rejected the request | Read the JSON error body — the API returns a descriptive message |
| `request failed: 401 Unauthorized` | Token expired or invalid | Refresh credentials; if using `--access-token`, get a new token |
| `request failed: 403 Forbidden` | No permission for this operation/org | Check org ID and API role |
| `unsupported output format "x" (expected table or json)` | Bad `--output` value | Use `table` or `json` |

HTTP errors (status >= 400) include the response body when the API returns one. The CLI pretty-prints it as indented JSON, so the user can read the `message` or `error` field directly.

## Gotchas

- `--organization-id` is **always required** and is injected automatically into API paths; omitting it gives: `missing organization ID: set --organization-id or UPWIND_ORGANIZATION_ID`
- Only `us`, `eu`, `me` are valid regions; anything else errors at runtime, not at flag parse time
- `--all` and `--body` can be combined; the same body is re-sent on every page request
- `cloud-account-tags` is a **CSV-encoded** array parameter (`?cloud-account-tags=a,b,c`), unlike all other array parameters which repeat the key
- `--access-token` takes priority over `--client-id`/`--client-secret`; if set, no OAuth call is made
- Tokens are cached **in memory only** — each CLI invocation fetches a fresh token unless the cached one has >30 s remaining (only matters within a single process)
- The `.env` file in the current working directory is loaded automatically and silently; env vars and flags override it
- Operations with `BodyRequired: true` will fail immediately if no `--body` or `--body-file` is provided — the CLI checks this *before* making the HTTP request
- The CLI validates that bodies are valid JSON but does **not** enforce the API schema — invalid fields produce a 400 from the server, not a local error

## Validation

After updating examples or usage guidance, verify key help screens still match:

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats list-threat-detections --help
```

A passing state means each help screen shows the expected flags, descriptions, and command names without errors.
