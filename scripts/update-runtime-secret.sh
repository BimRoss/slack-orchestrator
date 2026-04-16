#!/usr/bin/env bash
# Create or replace the Kubernetes Secret consumed by slack-orchestrator pods from this repo's .env.
# Direction: local .env → cluster Secret only. Does not read the cluster or modify .env.
#
# Expected by GitOps (envFrom):
#   rancher-admin/admin/apps/slack-orchestrator/deployment.yaml
#   secretRef: name: slack-orchestrator-runtime
#
# Keys (subset of .env.example / internal/config): only non-empty values are sent.
#
# Usage:
#   ./scripts/update-runtime-secret.sh
#   ENV_FILE=/path/.env NAMESPACE=slack-orchestrator ./scripts/update-runtime-secret.sh
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [[ -z "${KUBECONFIG:-}" ]]; then
  _bimross_kube="${HOME}/.kube/config/admin.yaml"
  if [[ -f "${_bimross_kube}" ]]; then
    export KUBECONFIG="${_bimross_kube}"
  fi
fi

ENV_FILE="${ENV_FILE:-${ROOT}/.env}"
NAMESPACE="${NAMESPACE:-slack-orchestrator}"
SECRET_NAME="${SECRET_NAME:-slack-orchestrator-runtime}"

kubectl_cmd() {
  if [[ -n "${KUBECONFIG:-}" ]]; then
    kubectl --kubeconfig="$KUBECONFIG" "$@"
  else
    kubectl "$@"
  fi
}

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE (copy .env.example to .env and fill tokens)." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

# Must match keys read by internal/config.FromEnv and deployment defaults.
RUNTIME_KEYS=(
  SLACK_BOT_TOKEN
  SLACK_APP_TOKEN
  MULTIAGENT_ORDER
  MULTIAGENT_BOT_USER_IDS
  MULTIAGENT_SHUFFLE_SECRET
  EVERYONE_AGENT_LIMIT
  CHANNEL_AGENT_LIMIT
  HTTP_ADDR
  LOG_JSON
)

secret_args=()
for key in "${RUNTIME_KEYS[@]}"; do
  val=""
  eval "val=\"\${${key}:-}\""
  if [[ -n "$val" ]]; then
    secret_args+=(--from-literal="$key=$val")
  fi
done

if [[ "${#secret_args[@]}" -eq 0 ]]; then
  echo "No runtime keys with values found in '$ENV_FILE'. Expected at least SLACK_BOT_TOKEN / SLACK_APP_TOKEN." >&2
  exit 1
fi

kubectl_cmd get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl_cmd create namespace "${NAMESPACE}"

kubectl_cmd -n "${NAMESPACE}" delete secret "${SECRET_NAME}" --ignore-not-found=true >/dev/null
kubectl_cmd -n "${NAMESPACE}" create secret generic "${SECRET_NAME}" "${secret_args[@]}"
echo "Updated secret '${SECRET_NAME}' in namespace '${NAMESPACE}' (${#secret_args[@]} keys)."
