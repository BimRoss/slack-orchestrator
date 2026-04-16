# slack-orchestrator

Single **Socket Mode** ingress for BimRoss Slack: receives `message.*`, `app_mention`, and reactions (see [`slack-factory` manifests](../slack-factory/manifests/orchestrator/)). Emits **routing decisions** (structured logs) — delegate to thin agent workers in a later phase.

## Routing (Phase 1)

| Trigger | Behavior |
|--------|----------|
| `<!everyone>` / `@everyone` | First **N** employees in `MULTIAGENT_ORDER` (default **5**) — `conversation` |
| `<!channel>` / `@channel` | First **N** (default **3**) — `conversation` |
| Squad `@mention` | First mentioned employee; `tool` vs `conversation` from keyword classifier |
| Plain channel message | **One** deterministic pseudo-random employee — `tool` vs `conversation` |

Ambiguous or non-tool text maps to **`conversation`** (no “missing tool” user errors at this layer).

## Run locally

```bash
cp .env.example .env
# set SLACK_BOT_TOKEN (xoxb-), SLACK_APP_TOKEN (xapp-)
go run ./cmd/slack-orchestrator
```

- `GET /health` — liveness  
- `GET /readyz` — readiness  

Set `LOG_JSON=true` for one JSON log line per routed message (includes `decision`).

## Env

See [`.env.example`](.env.example). Important:

- `MULTIAGENT_ORDER` — comma-separated keys (e.g. `alex,tim,ross,garth,joanne`).
- `MULTIAGENT_BOT_USER_IDS` — `alex=Uxxx,tim=Uyyy` so `<@U>` mentions resolve to an employee.
- `EVERYONE_AGENT_LIMIT` / `CHANNEL_AGENT_LIMIT` — default **5** and **3**.

## Docker

```bash
docker build -t slack-orchestrator:local .
```

Image CI: `geeemoney/slack-orchestrator` (tag on `v*`).

## Slack app manifests

Authoritative JSON lives in **`slack-factory`**:

- **Orchestrator** — [`manifests/orchestrator/app-manifest.json`](../slack-factory/manifests/orchestrator/app-manifest.json) (Socket Mode + message events).
- **Agents** — `manifests/<employee>/` — **no** `message.channels` / `message.im` subscriptions; minimal **write** scopes (`chat:write`, reactions, etc.). Re-OAuth after changes.

## Prod rollout

1. Deploy orchestrator with secrets (`SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, plus env above).  
2. Disable Socket Mode / message events on legacy employee Slack apps so only the orchestrator receives the firehose.  
3. Optional: dedicated dev workspace later.

## Phase 2

HTTP (or queue) dispatch from orchestrator to per-employee runtimes; keep routing policy centralized here.
