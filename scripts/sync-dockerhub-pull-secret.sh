#!/usr/bin/env bash
# Copy dockerhub-pull into the slack-orchestrator namespace so the cluster can pull
# geeemoney/slack-orchestrator (private Docker Hub).
#
# GitOps manifests that reference this secret:
#   rancher-admin/admin/apps/slack-orchestrator/deployment.yaml  (imagePullSecrets: dockerhub-pull)
#
# Usage (from slack-orchestrator repo root):
#   ./scripts/sync-dockerhub-pull-secret.sh
#   KUBECONFIG=~/.kube/config/admin.yaml ./scripts/sync-dockerhub-pull-secret.sh
#
set -euo pipefail

NAMESPACE="${NAMESPACE:-slack-orchestrator}"
PULL_SECRET_NAME="${PULL_SECRET_NAME:-dockerhub-pull}"
PULL_SECRET_SOURCE_NAMESPACE="${PULL_SECRET_SOURCE_NAMESPACE:-bimross-web}"
PULL_SECRET_FALLBACK_NAMESPACE="${PULL_SECRET_FALLBACK_NAMESPACE:-employee-factory}"

if [[ -z "${KUBECONFIG:-}" ]]; then
  if [[ -f "${HOME}/.kube/config/admin.yaml" ]]; then
    export KUBECONFIG="${HOME}/.kube/config/admin.yaml"
  elif [[ -f "${HOME}/.kube/config/grant-admin.yaml" ]]; then
    export KUBECONFIG="${HOME}/.kube/config/grant-admin.yaml"
  elif [[ -f "${HOME}/.kube/config/grant-admin.yml" ]]; then
    export KUBECONFIG="${HOME}/.kube/config/grant-admin.yml"
  fi
fi

kubectl_cmd() {
  if [[ -n "${KUBECONFIG:-}" ]]; then
    kubectl --kubeconfig="$KUBECONFIG" "$@"
  else
    kubectl "$@"
  fi
}

kubectl_cmd get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl_cmd create namespace "${NAMESPACE}"

source_ns="${PULL_SECRET_SOURCE_NAMESPACE}"
if ! kubectl_cmd get secret "${PULL_SECRET_NAME}" -n "${source_ns}" >/dev/null 2>&1; then
  echo "Pull secret '${PULL_SECRET_NAME}' not found in '${source_ns}', trying '${PULL_SECRET_FALLBACK_NAMESPACE}'..." >&2
  source_ns="${PULL_SECRET_FALLBACK_NAMESPACE}"
  kubectl_cmd get secret "${PULL_SECRET_NAME}" -n "${source_ns}" >/dev/null 2>&1 || {
    echo "Unable to find '${PULL_SECRET_NAME}' in '${PULL_SECRET_SOURCE_NAMESPACE}' or '${PULL_SECRET_FALLBACK_NAMESPACE}'." >&2
    exit 1
  }
fi

kubectl_cmd get secret "${PULL_SECRET_NAME}" -n "${source_ns}" -o json \
  | python3 -c 'import json,sys; src=json.load(sys.stdin); ns="'"${NAMESPACE}"'"; out={"apiVersion":"v1","kind":"Secret","metadata":{"name":src["metadata"]["name"],"namespace":ns},"type":src.get("type"),"data":src.get("data",{})}; print(json.dumps(out))' \
  | kubectl_cmd apply -f -

echo "Synced '${PULL_SECRET_NAME}' into namespace '${NAMESPACE}' from '${source_ns}'."
