# Upwind CLI Architecture

## Purpose

This repository provides a Go CLI for the Upwind Management APIs. The command tree is built at runtime from a **pre-generated Go catalog** (`internal/openapi/catalog_generated.go`) rather than from hand-written command definitions. The catalog is produced ahead of time from two upstream OpenAPI documents (v1 and v2) by the code generator in `tools/openapi-gen`. At runtime, no YAML parsing occurs — the CLI simply loads the generated catalog and merges it into one versionless command tree.

## Code Map

- `main.go`
  Entrypoint. Calls `cmd.Execute()`.
- `cmd/root.go`
  Thin wrapper that builds the root Cobra command from `internal/app`.
- `internal/app/app.go`
  The central runtime:
  - loads `.env`
  - loads the generated OpenAPI catalog (no YAML parsing at runtime)
  - creates the Cobra tree
  - collects flags and request bodies
  - validates JSON bodies locally (syntax only, not schema)
  - executes HTTP requests
  - follows pagination
  - formats HTTP errors with pretty-printed JSON bodies
  - sends decoded values to the renderer
- `internal/openapi/spec.go`
  Types and parsing logic used at **code-generation time** to derive:
  - versions and tags
  - operation names (kebab-case from `operationId`)
  - parameters and flag metadata
  - JSON body support (`HasJSONBody`, `BodyRequired`, `BodyDescription`)
  - pagination style
  - array encoding format (repeat vs CSV)
  At runtime, only `LoadCatalog()` is called, which returns the generated catalog directly.
- `internal/openapi/catalog_generated.go`
  The pre-generated catalog. **Do not edit by hand.** Regenerate with `go generate ./internal/openapi` after upstream spec changes.
- `internal/auth/provider.go`
  Produces the `Authorization` header from either:
  - `UPWIND_ACCESS_TOKEN` (bearer token used as-is; `Bearer ` prepended if missing), or
  - an OAuth client-credentials token request (`POST {auth-url}/oauth/token`)
  Tokens are cached in memory with a 30-second expiry buffer. If the token response lacks `expires_in`, a 5-minute lifetime is assumed.
- `internal/config/config.go`
  Loads `.env`, resolves region defaults, validates output mode and region, and finalizes runtime config. The `Resolve()` function normalizes all options and returns a `Runtime` struct.
- `internal/render/render.go`
  Writes either pretty JSON or flattened tables:
  - **JSON:** `json.MarshalIndent` with 2-space indentation.
  - **Table:** Arrays become row sets. Envelope objects with `items` or `resourceFindings` arrays render those as rows. Other objects render as field/value pairs. Column order uses a priority list (`id`, `name`, `title`, `display_name`, `status`, `severity`, `type`, `category`, `version`, timestamps) followed by remaining keys sorted alphabetically.
- `tools/openapi-gen/main.go`
  Code generator that reads YAML from `../spec-upwind` and writes `catalog_generated.go`. Only runs during `go generate`, never at normal runtime.

## Startup Flow

1. `main.go` calls `cmd.Execute()`.
2. `cmd.Execute()` calls `app.NewRootCmd()`.
3. `NewRootCmd()` loads `.env`, loads the generated catalog via `openapi.LoadCatalog()`, builds shared config options from environment variables, and creates the root Cobra command.
4. The catalog already contains both v1 and v2 operations. `PreferredTags()` merges them into one tag tree and prefers v2 when a tag and operation command name exist in both specs.
5. `internal/app` creates the root tag commands and their operations from that merged view. Operations whose `OperationID` differs from their `CommandName` get the ID added as a Cobra alias.

## Request Flow

For each generated operation command:

1. Resolve runtime config from flags and environment variables (`config.Resolve()`).
2. Require `organization-id` before proceeding.
3. Build an HTTP client and auth provider.
4. Collect path parameters from flags and inject the global organization ID.
5. Collect query parameters from flags (only flags explicitly changed by the user are sent).
6. Load a JSON request body from `--body`, `--body-file`, or stdin. Validate it is valid JSON. If the operation has `BodyRequired: true` and no body is provided, error immediately.
7. Build the final URL from the base URL, path template, path values, and query values.
8. Acquire an auth header (bearer token or OAuth).
9. Execute the request with `Accept: application/json`, `User-Agent: upwind-cli/<version>`, and `Content-Type: application/json` if a body is present.
10. If status >= 400, format the error with status line and pretty-printed JSON body (if parseable).
11. If `--all` is enabled, keep following pagination until there is no next page.
12. Render the final value as table or JSON.

## Pagination Rules

Pagination is inferred from the operation's query parameters:

- `cursor` → v2 cursor pagination using `metadata.next_cursor`
- `page-token` → v1 token pagination using the `Link` response header
- `page` → v1 page-based pagination

Priority order for detection: cursor > page-token > page.

When `--all` is set:
- **v1-page:** Increments `page` in the query string. Stops when the response has zero items, or fewer items than the page size.
- **v1-token:** Follows `Link: <url>; rel="next"` headers. Stops when no next link is present.
- **v2-cursor:** Reads `metadata.next_cursor` from the JSON response and sets `cursor` in the query string. Stops when the cursor is empty.

When `--all` is not set, the CLI performs exactly one request.

Paginated results are merged: array responses are concatenated; envelope objects with `items` or `resourceFindings` arrays merge those arrays while top-level metadata comes from the last page.

## Rendering Rules

- `json` output writes indented JSON (2-space indent, trailing newline)
- `table` output flattens objects using dotted keys
- Arrays become row sets
- Envelope objects with an `items` or `resourceFindings` array render that array as rows
- Single objects render as two-column (field / value) tables
- Column priority: `id`, `name`, `title`, `display_name`, `status`, `severity`, `type`, `category`, `version`, `create_time`, `update_time`, `created_at`, `updated_at`, `first_seen`, `last_seen`, then remaining keys alphabetically
- `nil` responses produce no output (no error)

## Error Handling

The CLI has three layers of error handling:

1. **Local validation errors** — produced before any HTTP request (missing org ID, bad region, invalid JSON body syntax, missing required body, conflicting `--body`/`--body-file`).
2. **OAuth errors** — produced when token acquisition fails. The CLI extracts `error` and `error_description` from the OAuth response body when available.
3. **HTTP errors** — any response with status >= 400. Formatted as `request failed: <status>\n<pretty JSON body>`. If the body is not valid JSON, it is printed as-is. If empty, only the status line appears.

## Important Repo Assumptions

- The generated catalog is the command source of truth. Do not add Cobra commands by hand (except `version`).
- `organization-id` is always treated as a top-level required input — it is injected into all API paths and never appears as a per-operation flag.
- Only JSON request bodies are supported.
- `cloud-account-tags` is a special-case CSV-encoded array query parameter; all others use repeated keys.
- Query parameters are only included in the request when the user explicitly sets the corresponding flag (`cmd.Flags().Changed()`).

## Recommended Validation

Use these commands after changes:

```bash
go test ./...
go vet ./...
go build -o upwind .
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
```
