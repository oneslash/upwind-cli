---
name: upwind-cli
description: Use when helping someone use the Upwind CLI. Covers setup, authentication, configuration, command discovery, output formats, request bodies, pagination, and shell completion.
---

# Upwind CLI Usage

## When To Use This Skill

Use this skill when the task is about operating the `upwind` app rather than changing its implementation.

Typical cases:

- getting the CLI built and ready to run
- configuring auth, region, organization, and output defaults
- discovering commands and flags
- listing, searching, fetching, or updating Upwind resources
- choosing between table and JSON output
- sending JSON request bodies
- handling paginated endpoints
- enabling shell completion
- troubleshooting common usage mistakes

If the user wants to modify the Go code, update specs, or explain internal architecture, inspect the repository directly instead of relying on this skill.

## How To Help

1. Prefer live help output over guessing command names or flags.
2. Give users runnable commands, not abstract descriptions.
3. Use `--output json` in examples that are likely to be scripted or piped.
4. Keep examples explicit about whether they require `--body`, `--body-file`, `--all`, `--limit`, or an object ID.
5. If a command might return many records, mention pagination behavior.

Start discovery with:

```bash
./upwind --help
./upwind <tag> --help
./upwind <tag> <operation> --help
```

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

## Core Usage Model

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

## Common Tasks

Discover available commands:

```bash
./upwind --help
./upwind inventory --help
./upwind threats --help
```

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

The CLI supports the pagination styles exposed by the Upwind APIs:

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

Without `--all`, the CLI makes a single request and you can control page or cursor flags manually.

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
- If a command or flag is unclear, use the specific help screen instead of guessing:

```bash
./upwind <tag> <operation> --help
```

## Validation

When answering usage questions, these are the quickest sanity checks:

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats --help
```
