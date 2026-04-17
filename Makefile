.PHONY: env-dev env-prod run run-dev run-prod test docker-build docker-up docker-logs docker-local-up docker-local-logs

# Point .env at .env.dev or .env.prod (gitignored symlinks).
env-dev:
	@./scripts/use-env.sh dev

env-prod:
	@./scripts/use-env.sh prod

# Requires ./scripts/use-env.sh dev|prod first (or an existing .env).
run:
	go run ./cmd/slack-orchestrator

run-dev: env-dev
	go run ./cmd/slack-orchestrator

run-prod: env-prod
	go run ./cmd/slack-orchestrator

test:
	go test ./...

docker-build:
	docker build -t slack-orchestrator:local .

# Reads .env.dev by default (see docker-compose.yml). Override: SLACK_ORCHESTRATOR_ENV_FILE=.env.prod
# Prefer this target: orchestrator is on profile "local" (NATS on host via host.docker.internal:4222).
docker-up:
	docker compose --profile local up --build

docker-logs:
	docker compose --profile local logs -f slack-orchestrator

docker-local-up: docker-up

docker-local-logs: docker-logs
