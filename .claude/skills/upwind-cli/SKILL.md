---
name: upwind-cli
description: Use when helping someone configure or operate the Upwind CLI application. Covers setup, authentication, environment variables, command discovery, request bodies, output modes, pagination, and shell completion.
---

# Upwind CLI

## When To Use This Skill

Use this skill when the user wants help using the `upwind` CLI itself.

Typical cases:

- first-time setup
- `.env` and auth configuration
- region or organization selection
- finding the right command or flags
- listing, searching, fetching, or updating resources
- choosing table versus JSON output
- sending JSON request bodies
- using `--all` and pagination flags
- enabling shell completion
- troubleshooting CLI usage

If the user wants to modify the Go code, explain how commands are generated, or change runtime behavior, inspect the repository and read [references/architecture.md](references/architecture.md) instead.

## How To Help

Start with [references/usage.md](references/usage.md).

Follow this workflow:

1. Prefer live `./upwind --help` output over guessing command names or flags.
2. Give runnable commands, not abstract descriptions.
3. Be explicit about required auth and config inputs.
4. Mention `--output json` for scripting and `--output table` for terminal reading.
5. If an operation can return many results, mention pagination and whether `--all` is appropriate.
6. Use [references/architecture.md](references/architecture.md) only when the task shifts from using the app to explaining or changing its internals.

Start discovery with:

```bash
./upwind --help
./upwind <tag> --help
./upwind <tag> <operation> --help
```

## Validation

When updating examples or usage guidance, verify the key help screens still match:

```bash
./upwind --help
./upwind inventory --help
./upwind inventory search-assets --help
./upwind threats --help
```
