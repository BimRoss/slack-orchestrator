#!/usr/bin/env bash
# Canonical entrypoint for pushing cluster secrets needed by the rancher-admin GitOps Deployment
# for slack-orchestrator. Lives in this repo so .env → cluster wiring stays next to the app.
#
# Steps:
#   1. sync-dockerhub-pull-secret.sh — copy dockerhub-pull into namespace slack-orchestrator
#   2. update-runtime-secret.sh — create/replace slack-orchestrator-runtime from .env
#
# GitOps reference:
#   https://github.com/BimRoss/rancher-admin/tree/master/admin/apps/slack-orchestrator
#
# Usage:
#   ./scripts/update-rancher-secrets.sh
#   ROLLOUT_RESTART=true ./scripts/update-rancher-secrets.sh
#   ENV_FILE=/path/.env KUBECONFIG=~/.kube/config/admin.yaml ./scripts/update-rancher-secrets.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

"${SCRIPT_DIR}/sync-dockerhub-pull-secret.sh"
"${SCRIPT_DIR}/update-runtime-secret.sh"

if [[ "${ROLLOUT_RESTART:-}" == "true" ]]; then
  NAMESPACE="${NAMESPACE:-slack-orchestrator}"
  if [[ -z "${KUBECONFIG:-}" ]]; then
    _bimross_kube="${HOME}/.kube/config/admin.yaml"
    if [[ -f "${_bimross_kube}" ]]; then
      export KUBECONFIG="${_bimross_kube}"
    fi
  fi
  kubectl_cmd() {
    if [[ -n "${KUBECONFIG:-}" ]]; then
      kubectl --kubeconfig="$KUBECONFIG" "$@"
    else
      kubectl "$@"
    fi
  }
  kubectl_cmd -n "${NAMESPACE}" rollout restart deploy/slack-orchestrator
  echo "Rollout restart requested for deploy/slack-orchestrator."
fi

echo "Done. Fleet manifests unchanged; only cluster Secrets were updated."
