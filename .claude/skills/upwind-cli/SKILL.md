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

## Key Facts (quick reference)

### Command shape
```
upwind <tag> <operation> [flags]
```
Examples: `upwind threats list-threat-detections`, `upwind inventory search-assets`

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

## Gotchas

- `--organization-id` is **always required** and is injected automatically into API paths; omitting it gives: `missing organization ID: set --organization-id or UPWIND_ORGANIZATION_ID`
- Only `us`, `eu`, `me` are valid regions; anything else errors at runtime, not at flag parse time
- `--all` and `--body` can be combined; the same body is re-sent on every page request
- `cloud-account-tags` is a **CSV-encoded** array parameter (`?cloud-account-tags=a,b,c`), unlike all other array parameters which repeat the key
- `--access-token` takes priority over `--client-id`/`--client-secret`; if set, no OAuth call is made
- Tokens are cached **in memory only** — each CLI invocation fetches a fresh token unless the cached one has >30 s remaining (only matters within a single process)
- The `.env` file in the current working directory is loaded automatically and silently; env vars and flags override it

## Validation

After updating examples or usage guidance, verify key help screens still match:

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats list-threat-detections --help
```

A passing state means each help screen shows the expected flags, descriptions, and command names without errors.
