# slack-orchestrator

Single **Socket Mode** ingress for BimRoss Slack: receives `message.*`, `app_mention`, and reactions (see [`slack-factory` manifests](../slack-factory/manifests/orchestrator/)). Emits **routing decisions** (structured logs) — delegate to thin agent workers in a later phase.

## Routing (Phase 1)

| Trigger | Behavior |
|--------|----------|
| `<!everyone>` / `@everyone` | First **N** in the **resolved roster** (default **5**) — `conversation` |
| `<!channel>` / `@channel` | First **N** in that roster (default **3**) — `conversation` |
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

- **Roster** — derived from keys in `MULTIAGENT_BOT_USER_IDS`, sorted, then **shuffled**; the shuffle seed is **derived from the map** (optional `MULTIAGENT_SHUFFLE_SECRET` override only). Optional `MULTIAGENT_ORDER` overrides for emergencies.
- `MULTIAGENT_BOT_USER_IDS` — `alex=Uxxx,tim=Uyyy` so `<@U>` mentions resolve to an employee and the squad list is known.
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

## Admin cluster (GitOps + secrets)

Fleet manifests live in **[`rancher-admin`](https://github.com/BimRoss/rancher-admin)** under `admin/apps/slack-orchestrator/` (Deployment references Secret `slack-orchestrator-runtime` and `imagePullSecrets: dockerhub-pull`).

Push **cluster-only** secrets from this repo (keeps `.env` → cluster mapping next to the code):

```bash
# From repo root; uses .env by default
./scripts/update-rancher-secrets.sh
# optional: pick up new Secret without waiting for rollout
ROLLOUT_RESTART=true ./scripts/update-rancher-secrets.sh
```

| Script | Purpose |
|--------|---------|
| [`scripts/update-rancher-secrets.sh`](scripts/update-rancher-secrets.sh) | Runs pull-secret sync + runtime Secret (canonical entrypoint). |
| [`scripts/sync-dockerhub-pull-secret.sh`](scripts/sync-dockerhub-pull-secret.sh) | Copies `dockerhub-pull` into namespace `slack-orchestrator` (same pattern as other `geeemoney/*` workloads). |
| [`scripts/update-runtime-secret.sh`](scripts/update-runtime-secret.sh) | Creates/replaces `slack-orchestrator-runtime` from `.env` (keys listed in the script header). |

## Prod rollout

1. Merge Fleet manifests; ensure GitRepo watches `admin`.  
2. Run `./scripts/update-rancher-secrets.sh` after filling `.env`.  
3. Disable Socket Mode / message events on legacy employee Slack apps so only the orchestrator receives the firehose.  
4. Optional: dedicated dev workspace later.

## Phase 2

- **Dispatch:** HTTP (or queue) from orchestrator to per-employee runtimes; routing policy stays here.
- **Roster / tools (future source of truth):** a **Redis** map (employee → Slack **bot user id**, tool / capability ids aligned with the skills catalog). Same IDs you already use in **employee-factory**; orchestrator stays thin. **Admin UI** can own writes later; **Slack / ops** is fine until then.
- **Now:** `MULTIAGENT_BOT_USER_IDS` in env (bootstrap only, not the long-term config surface).
