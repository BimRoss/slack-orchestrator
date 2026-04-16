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
- `GET /metrics` — Prometheus (Socket Mode state, acks; delegate metrics reserved for HTTP dispatch)

Structured JSON logs are **on by default** (`decision` per message). Set `LOG_JSON=false` to disable.

### Post-deploy sanity checklist

1. **Image** — Deployment image tag matches the Git / release you intended (Fleet or manual bump).  
2. **Secrets** — `./scripts/update-rancher-secrets.sh` (or your namespace equivalent) applied so `slack-orchestrator-runtime` matches `.env` / `.env.example`.  
3. **Probes** — `GET /health` and `GET /readyz` return 200.  
4. **Metrics** — `GET /metrics` exposes `slack_orchestrator_socket_mode_state` and `slack_orchestrator_events_api_acked_total`.  
5. **Logs** — At least one line with `socket_mode` / `state` / `connected` after startup (reconnect storms should still show alternating connecting → connected).  
6. **GitOps** — One line: Fleet repo revision applied; no need to poll pods here unless you are debugging.

### Observability (Grafana / Prometheus)

Use the same Prometheus scrape as other `employees` / admin workloads if the ServiceMonitor or annotations are already wired; otherwise scrape the orchestrator Service port manually.

Example **PromQL** (adjust job/namespace labels to your scrape config):

- **Socket Mode connected (1 = live):**  
  `slack_orchestrator_socket_mode_state == 2`
- **Events API ack rate (per second):**  
  `rate(slack_orchestrator_events_api_acked_total[5m])`
- **Delegate dispatch (when Phase 2 HTTP is enabled):**  
  `rate(slack_orchestrator_delegate_post_total[5m])`  
  `histogram_quantile(0.95, sum(rate(slack_orchestrator_delegate_http_request_seconds_bucket[5m])) by (le, result))`

When **`ORCHESTRATOR_DISPATCH_ENABLED=true`** and **`ORCHESTRATOR_WORKER_URL_TEMPLATE`** is set, `slack_orchestrator_delegate_*` metrics reflect outbound POSTs (retries on **503**; HMAC signing when **`ORCHESTRATOR_WORKER_HMAC_SECRET`** is set). Workers expose **`POST /internal/slack/event`** (see **employee-factory**).

## Env

See [`.env.example`](.env.example). Important:

- **Roster** — derived from keys in `MULTIAGENT_BOT_USER_IDS`, sorted, then **shuffled**; the shuffle seed is **derived from the map** (optional `MULTIAGENT_SHUFFLE_SECRET` override only). Optional `MULTIAGENT_ORDER` overrides for emergencies.
- `MULTIAGENT_BOT_USER_IDS` — `alex=Uxxx,tim=Uyyy` so `<@U>` mentions resolve to an employee and the squad list is known.
- `EVERYONE_AGENT_LIMIT` / `CHANNEL_AGENT_LIMIT` — default **5** and **3**.
- **Dispatch (optional)** — `ORCHESTRATOR_DISPATCH_ENABLED`, `ORCHESTRATOR_WORKER_URL_TEMPLATE` (must include `{employee}`), `ORCHESTRATOR_WORKER_HMAC_SECRET`, `ORCHESTRATOR_DISPATCH_TIMEOUT_SEC`.

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

## Phase 2 (dispatch)

- **Implemented:** `POST` JSON (**`internal/inbound/v1`**) to each worker URL built from **`ORCHESTRATOR_WORKER_URL_TEMPLATE`** (e.g. `http://employee-factory-{employee}.employee-factory.svc.cluster.local:8080`), path **`/internal/slack/event`**. Toggle with **`ORCHESTRATOR_DISPATCH_ENABLED`**. Optional **HMAC** via **`ORCHESTRATOR_WORKER_HMAC_SECRET`** (header `X-BimRoss-Orchestrator-Signature: v1=<hex>`).
- **Next:** wire worker handler to the existing Socket Mode pipeline (Redis dedupe, `SLACK_INGRESS=orchestrator`), then remove duplicate message subscriptions from agent apps.
- **Roster / tools (future):** Redis map (employee → Slack bot user id, skills catalog ids). Until then: **`MULTIAGENT_BOT_USER_IDS`** in env.
