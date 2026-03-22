# Upwind CLI Architecture

## Purpose

This repository provides a Go CLI for the Upwind Management APIs. The CLI is generated at runtime from two embedded OpenAPI documents rather than from hand-written command definitions, then merged into one versionless command tree.

## Code Map

- `main.go`
  Entrypoint. Calls `cmd.Execute()`.
- `cmd/root.go`
  Thin wrapper that builds the root Cobra command from `internal/app`.
- `internal/app/app.go`
  The central runtime:
  - loads `.env`
  - loads the OpenAPI catalog
  - creates the Cobra tree
  - collects flags and request bodies
  - executes HTTP requests
  - follows pagination
  - sends decoded values to the renderer
- `internal/openapi/spec.go`
  Parses the embedded YAML specs and derives:
  - versions and tags
  - operation names
  - parameters and flag metadata
  - JSON body support
  - pagination style
- `internal/auth/provider.go`
  Produces the `Authorization` header from either:
  - `UPWIND_ACCESS_TOKEN`, or
  - an OAuth client-credentials token request
- `internal/config/config.go`
  Loads `.env`, resolves region defaults, validates output mode, and finalizes runtime config.
- `internal/render/render.go`
  Writes either pretty JSON or flattened tables.

## Startup Flow

1. `main.go` calls `cmd.Execute()`.
2. `cmd.Execute()` calls `app.NewRootCmd()`.
3. `NewRootCmd()` loads `.env`, parses both embedded specs, builds shared config options from environment variables, and creates the root Cobra command.
4. `internal/openapi` merges both versions into one tag tree and prefers v2 when a tag and operation exist in both specs.
5. `internal/app` creates the root tag commands and their operations from that merged view.

## Request Flow

For each generated operation command:

1. Resolve runtime config from flags and environment variables.
2. Require `organization-id` before making the request.
3. Build an HTTP client and auth provider.
4. Collect path parameters from flags and inject the global organization ID.
5. Collect query parameters from flags.
6. Load a JSON request body from `--body`, `--body-file`, or stdin.
7. Build the final URL from the base URL, path template, path values, and query values.
8. Acquire an auth header.
9. Execute the request and decode JSON into generic Go values.
10. If `--all` is enabled, keep following pagination until there is no next page.
11. Render the final value as table or JSON.

## Pagination Rules

Pagination is inferred from the operation's query parameters:

- `page` means v1 page-based pagination
- `page-token` means v1 token pagination using the `Link` response header
- `cursor` means v2 cursor pagination using `metadata.next_cursor`

When `--all` is not set, the CLI performs exactly one request.

## Rendering Rules

- `json` output writes indented JSON
- `table` output flattens objects using dotted keys
- arrays become row sets
- envelope objects with an `items` array render that array as rows

## Important Repo Assumptions

- The embedded specs are the command source of truth.
- `organization-id` is always treated as a top-level required input.
- Only JSON request bodies are supported.
- `cloud-account-tags` is a special-case CSV-encoded array query parameter.

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
