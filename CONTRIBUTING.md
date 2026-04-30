# Contributing

## Local setup

1. Copy `.env.example` to `.env.dev` and fill local/test values only.
2. Start dependencies and app:
   - Docker: `docker compose --profile local up --build`
   - Go: `./scripts/use-env.sh dev && go run ./cmd/slack-orchestrator`
3. Run tests before opening a PR:
   - `go test ./...`

## Secrets and environment files

- Never commit real credentials or tokens.
- Keep `.env.dev` and `.env.prod` local only (already gitignored).
- Commit template-only changes in `.env.example` when docs/defaults change.

## Pull request expectations

- Keep changes scoped and explain the why in the PR description.
- Update docs when behavior, config, or workflows change.
- Ensure CI is green and no new secret-like values are introduced in tracked files.
