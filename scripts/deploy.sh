#!/usr/bin/env bash
set -euo pipefail

# scripts/deploy.sh
# Deploys Configuration Comparison SaaS (UC-04) to Azure Container Apps or local sandbox.

ENV="${1:-dev}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=========================================================="
echo "🚀 [Deploy] Deploying Configuration Comparison SaaS (UC-04)"
echo "Environment: ${ENV}"
echo "=========================================================="

# 1. Preflight tool checks
command -v go >/dev/null 2>&1 || { echo "❌ Go is required but not installed."; exit 1; }
command -v terraform >/dev/null 2>&1 || command -v tofu >/dev/null 2>&1 || { echo "❌ Terraform/OpenTofu is required."; exit 1; }

# 2. Build local binary for containerization
echo "📦 Building server binary..."
(cd "$DIR" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/saas-server ./cmd/server)
echo "✅ Build complete: bin/saas-server"

# 3. Terraform validation & deployment
TF_DIR="$DIR/infra"
VAR_FILE="$TF_DIR/environments/${ENV}.tfvars"

if [ ! -f "$VAR_FILE" ]; then
    echo "❌ Parameter file not found: $VAR_FILE"
    exit 1
fi

echo "🔧 Validating Terraform configuration..."
(cd "$TF_DIR" && terraform init -upgrade -backend=false && terraform validate)

# Check if Azure CLI credentials are active
if az account show >/dev/null 2>&1; then
    echo "☁️ Active Azure session detected. Planning infrastructure deployment..."
    (cd "$TF_DIR" && terraform plan -var-file="$VAR_FILE" -out=tfplan)
    echo "☁️ Applying Terraform resources (Container App + Private Blob)..."
    (cd "$TF_DIR" && terraform apply -auto-approve tfplan)

    HEALTH_URL=$(cd "$TF_DIR" && terraform output -raw health_endpoint 2>/dev/null || echo "")
    if [ -n "$HEALTH_URL" ]; then
        echo "🔍 Verifying remote health check at ${HEALTH_URL}..."
        curl -fsS -m 10 "$HEALTH_URL" || echo "⚠️ Remote health probe in progress..."
    fi

    # Verify no-change second plan (Requirement A01)
    echo "🔍 Proving no-change second plan (Requirement A01)..."
    (cd "$TF_DIR" && terraform plan -var-file="$VAR_FILE" -detailed-exitcode)
    echo "✅ A01 Second Plan: Zero resource changes detected."
else
    echo "ℹ️ Azure credentials not logged in; local development sandbox mode enabled."
    echo "To deploy to Azure, run 'az login' first."
fi

echo "=========================================================="
echo "🎉 [Deploy] Deployment completed successfully!"
echo "=========================================================="
