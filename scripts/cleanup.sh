#!/usr/bin/env bash
set -euo pipefail

# scripts/cleanup.sh
# Destructive cleanup of team-owned resources (Azure Container App + Blob Container).
# Explicit confirmation required.

ENV="${1:-dev}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TF_DIR="$DIR/infra"
VAR_FILE="$TF_DIR/environments/${ENV}.tfvars"

echo "=========================================================="
echo "⚠️  [Cleanup] DESTRUCTIVE TEARDOWN OF TEAM-OWNED RESOURCES"
echo "Environment: ${ENV}"
echo "=========================================================="

CONFIRM=""
if [ "${2:-}" = "--confirm" ] || [ "${2:-}" = "-y" ]; then
    CONFIRM="CONFIRM"
else
    echo "This operation will destroy the Azure Container App and private Blob Container."
    read -r -p "Type 'CONFIRM' to proceed with resource deletion: " CONFIRM
fi

if [ "$CONFIRM" != "CONFIRM" ]; then
    echo "❌ Cleanup aborted: explicit confirmation not received."
    exit 1
fi

echo "🗑️ Proceeding with cleanup of team-owned resources..."

if az account show >/dev/null 2>&1 && [ -f "$VAR_FILE" ]; then
    echo "☁️ Destroying Terraform resources in Azure (${ENV})..."
    (cd "$TF_DIR" && terraform destroy -var-file="$VAR_FILE" -auto-approve)
    echo "✅ Remote resources destroyed."
else
    echo "ℹ️ Azure credentials not active or local environment. Cleaning up local sandbox artifacts..."
fi

# Clean up local binaries and test databases
rm -f "$DIR/bin/saas-server" "$DIR/saas.db" "$DIR/saas.db-wal" "$DIR/saas.db-shm" "$DIR/test_*.db"
echo "✅ Local sandbox clean up complete."

echo "=========================================================="
echo "🎉 [Cleanup] Teardown completed successfully."
echo "=========================================================="
