# slack-orchestrator

Single **Socket Mode** ingress for BimRoss Slack: receives `message.*`, `app_mention`, and reactions (see [`slack-factory` manifests](../slack-factory/manifests/orchestrator/)). Computes **routing decisions** and, when dispatch is enabled, publishes **JetStream** events for **employee-factory** workers (`schema_version: 2`).

## Routing (Phase 1)

| Trigger | Behavior |
|--------|----------|
| `<!everyone>` / `@everyone` | First **N** in the **resolved roster** (default **5**) — `conversation` |
| `<!channel>` / `@channel` | First **N** in that roster (default **3**) — `conversation` |
| Squad `@mention` | First mentioned employee; `tool` vs `conversation` from keyword classifier |
| Plain **channel-root** message (no `thread_ts`) | **One** deterministic pseudo-random employee — `tool` vs `conversation` |
| Plain **thread** reply (`thread_ts` set) | **One** employee (same `pickPlainResponder` hash as channel-root) — `tool` vs `conversation`; **not** a full-roster fan-out |

Inbound NATS payload uses **`schema_version: 2`** (`internal/inbound/v1.go`). Each `routing.Decision` includes **`dispatch_mode`** (`single` \| `fanout`) and **`primary_employee`** for observability. `GET /debug/decisions` returns **`schema_version: 2`** with the same decision shape.

Ambiguous or non-tool text maps to **`conversation`** (no “missing tool” user errors at this layer).

## Run locally

```bash
cp .env.example .env
# set SLACK_BOT_TOKEN (xoxb-), SLACK_APP_TOKEN (xapp-)
go run ./cmd/slack-orchestrator
```

- `GET /health` — liveness  
- `GET /readyz` — readiness  
- `GET /metrics` — Prometheus (Socket Mode state, acks; JetStream delegate metrics when dispatch is enabled)
- `GET /debug/decisions?limit=100` — JSON decision log (last N **in-memory** entries on **this process only**). Bounded by **`ORCHESTRATOR_DECISION_LOG_MAX`** (default 500).

### Kubernetes: run **one replica** (until shared decision storage)

The decision log is **not** shared across pods. If you scale the Deployment to 2+, the Service round-robins `/debug/decisions` and **`/orchestrator` on makeacompany.ai will look like random events are missing** (each pod has its own buffer). **Keep `replicas: 1`** in GitOps unless you add Redis/SQL persistence for decisions or a single dedicated debug endpoint.

Slack **Socket Mode** for this app is also operated as **one active connection** in practice; do not scale out for “HA” without an explicit design (shared store, leader election, or Slack’s recommended topology).
  - **`ORCHESTRATOR_DEBUG_ALLOW_ANON=true`**: no `Authorization` header (convenience; use behind firewall or turn off later).
  - Otherwise **`ORCHESTRATOR_DEBUG_TOKEN`** must be set and requests must send `Authorization: Bearer <token>`. If the token is unset and anon is off, the endpoint returns **503**.

The **makeacompany.ai** page **`/orchestrator`** proxies via **`ORCHESTRATOR_DEBUG_BASE_URL`** on the frontend; set **`ORCHESTRATOR_DEBUG_ALLOW_ANON`** the same on both services, or use a shared **`ORCHESTRATOR_DEBUG_TOKEN`** in `makeacompany-ai-runtime-secrets` and orchestrator secrets.

Logging is **always JSON** (`log/slog` with a JSON handler) to **stdout** via **`internal/logging.Init()`** from `main`, so cluster UIs that only collect stdout still show logs.

Use **`log/slog`** everywhere (`slog.Info`, `slog.Warn`, …); do not configure ad hoc loggers or alternate formats outside this package.

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
- **Delegate dispatch (when JetStream dispatch is enabled):**  
  `rate(slack_orchestrator_delegate_publish_total[5m])`  
  `histogram_quantile(0.95, sum(rate(slack_orchestrator_delegate_publish_seconds_bucket[5m])) by (le, result))`

When **`ORCHESTRATOR_DISPATCH_ENABLED=true`** and **`ORCHESTRATOR_NATS_URL`** is set, the orchestrator publishes JSON (**`internal/inbound/v1`**) to **`slack.work.<employee>.events`** on stream **`ORCHESTRATOR_NATS_STREAM`** (default **`SLACK_WORK`**). Workers consume via **`NATS_URL`** (see **employee-factory**).

## Env

See [`.env.example`](.env.example). Important:

- **Roster** — derived from keys in `MULTIAGENT_BOT_USER_IDS`, sorted, then **shuffled**; the shuffle seed is **derived from the map** (optional `MULTIAGENT_SHUFFLE_SECRET` override only). Optional `MULTIAGENT_ORDER` overrides for emergencies.
- `MULTIAGENT_BOT_USER_IDS` — `alex=Uxxx,tim=Uyyy` so `<@U>` mentions resolve to an employee and the squad list is known.
- `EVERYONE_AGENT_LIMIT` / `CHANNEL_AGENT_LIMIT` — default **5** and **3**.
- **Dispatch (optional)** — `ORCHESTRATOR_DISPATCH_ENABLED`, `ORCHESTRATOR_NATS_URL`, `ORCHESTRATOR_NATS_STREAM` (default `SLACK_WORK`).

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

**employee-factory channel-knowledge CronJob** reads history with the same orchestrator bot token. After updating `slack-orchestrator-runtime`, run **[`employee-factory/scripts/sync-channel-knowledge-orchestrator-secret.sh`](https://github.com/BimRoss/employee-factory/blob/main/scripts/sync-channel-knowledge-orchestrator-secret.sh)** so `employee-factory-orchestrator-runtime` stays in sync (see **employee-factory** [`docs/channel-knowledge.md`](https://github.com/BimRoss/employee-factory/blob/main/docs/channel-knowledge.md)).

## Prod rollout

1. Merge Fleet manifests; ensure GitRepo watches `admin`.  
2. Run `./scripts/update-rancher-secrets.sh` after filling `.env`.  
3. Disable Socket Mode / message events on legacy employee Slack apps so only the orchestrator receives the firehose.  
4. Optional: dedicated dev workspace later.

## Dispatch (JetStream)

- **Implemented:** JSON (**`internal/inbound/v1`**) published to **`slack.work.<employee>.events`** per routing target. Toggle with **`ORCHESTRATOR_DISPATCH_ENABLED`**. Requires **`ORCHESTRATOR_NATS_URL`** (same NATS as worker **`NATS_URL`**). Stream/subjects are auto-created if missing.
- **Workers:** `SLACK_INGRESS=orchestrator`, **`NATS_URL`**, durable pull consumer per employee (see **employee-factory** `internal/natsbus`).
- **Roster / tools (future):** Redis map (employee → Slack bot user id, skills catalog ids). Until then: **`MULTIAGENT_BOT_USER_IDS`** in env.
