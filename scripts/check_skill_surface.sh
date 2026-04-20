#!/usr/bin/env bash
# Run default contract + tier-1 + pipeline alignment tests.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
go test ./internal/inbound/... ./internal/routing/... -count=1 -run 'Contract|Tier1|Pipeline|ContractAlignment'
echo "slack-orchestrator skill surface OK"
