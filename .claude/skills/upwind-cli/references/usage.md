# Upwind CLI Usage Reference

Use this reference when the task is about operating the CLI. It contains the detailed setup and command examples that should stay out of `SKILL.md`.

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

Alternative auth:

- `UPWIND_ACCESS_TOKEN` or `--access-token` skips OAuth token acquisition and uses that bearer token directly.

Useful optional defaults:

- `UPWIND_OUTPUT=table` or `json`
- `UPWIND_TIMEOUT=30s`

The CLI loads `.env` automatically from the current working directory when present.

## Core Command Shape

Commands follow this pattern:

```bash
upwind <tag> <operation>
```

Examples:

- `upwind threats list-threat-detections`
- `upwind workflows list-all-workflows`
- `upwind inventory search-assets`
- `upwind threats list-stories`

Important global flags:

- `--organization-id`
- `--region us|eu|me`
- `--output table|json`
- `--client-id`
- `--client-secret`
- `--access-token`
- `--timeout`

## Discovery

Use live help first:

```bash
./upwind --help
./upwind inventory --help
./upwind threats --help
./upwind inventory search-assets --help
```

## Common Tasks

Fetch a single asset:

```bash
./upwind --output json inventory get-asset --id uwr-example
```

List detections in table output:

```bash
./upwind threats list-threat-detections --severity HIGH
```

Search assets with an inline JSON body:

```bash
./upwind inventory search-assets \
  --body '{"conditions":[{"field":"category","operator":"eq","value":["compute_platform"]}]}' \
  --limit 50
```

Search stories from a file and fetch all pages:

```bash
./upwind threats search-stories --body-file search.json --limit 100 --all
```

Update a threat detection:

```bash
./upwind threats update-threat-detection \
  --detection-id det_123 \
  --body '{"status":"ARCHIVED"}'
```

## Output And Request Bodies

Use table output for terminal reading:

```bash
./upwind --output table threats list-threat-detections
```

Use JSON output for scripting:

```bash
./upwind --output json inventory get-asset --id uwr-example
```

Operations with request bodies accept:

- `--body '{"key":"value"}'`
- `--body-file request.json`
- `--body-file -` to read JSON from stdin

Example from stdin:

```bash
cat query.json | ./upwind inventory search-assets --body-file - --limit 50
```

## Pagination

The CLI supports:

- page-based pagination
- token or link-header pagination
- cursor-based pagination

Use `--all` to automatically fetch every page when the operation supports it.

Examples:

```bash
./upwind events search-shift-left-events --all --per-page 100 --body-file query.json
./upwind threats get-events-list --all --per-page 100
./upwind inventory search-assets --all --limit 200 --body-file query.json
```

Without `--all`, the CLI makes one request and the caller controls page or cursor flags manually.

## Shell Completion

List completion targets:

```bash
./upwind completion --help
```

Generate zsh completion for the current shell:

```bash
source <(./upwind completion zsh)
```

Install zsh completion on macOS:

```bash
./upwind completion zsh > $(brew --prefix)/share/zsh/site-functions/_upwind
```

## Troubleshooting

- If auth fails, verify `UPWIND_CLIENT_ID` and `UPWIND_CLIENT_SECRET`, or provide `UPWIND_ACCESS_TOKEN`.
- If requests target the wrong environment, check `UPWIND_REGION` or override with `--region`.
- If output is hard to parse in scripts, switch to `--output json`.
- If a command or flag is unclear, use the specific help screen instead of guessing.
