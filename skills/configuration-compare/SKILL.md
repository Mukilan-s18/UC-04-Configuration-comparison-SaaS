---
name: configuration-compare
description: Orchestrates deployment, acceptance verification, forced-failure troubleshooting, rollback, and cleanup for Configuration Comparison SaaS (Advantix UC-04).
---

# Configuration Comparison SaaS Runbook (UC-04)

This skill provides the complete operational runbook for deploying, verifying, troubleshooting, rolling back, and cleaning up the **Configuration Comparison SaaS** service.

## Trigger Conditions
Activate this skill whenever:
- Preparing or deploying the SaaS service to Azure Container Apps or local environment.
- Running acceptance test suite A01 through A12.
- Investigating or troubleshooting simulated failure hook (`X-Test-Force-Failure`).
- Performing a zero-downtime rollback using a prior container image.
- Executing a verified, confirmed teardown of team-owned cloud resources.

## Required Inputs & Environment Variables
| Input Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `ENVIRONMENT` | Target deployment tier | `dev` (or `test`) |
| `AZURE_SUBSCRIPTION_ID` | Active Azure Subscription ID | Sandbox subscription |
| `CONTAINER_IMAGE` | Container image URL to deploy | `ghcr.io/advantix/configguard:v1.0.0` |
| `PREVIOUS_IMAGE` | Prior known-good image for rollback | `ghcr.io/advantix/configguard:v0.9.0` |

---

## 1. Preflight Verification
Verify that required toolchains and CLI environments are available:

```bash
# Check Go compiler
go version

# Check Terraform or OpenTofu
terraform version

# Verify Azure CLI session (if deploying to cloud)
az account show --output table || echo "Running in local sandbox mode"
```

**Expected Result**:
- `go version >= go1.22`
- `terraform >= 1.5.0`
- Azure subscription is displayed or fallback to local mode.

**Stop Condition**: Stop immediately if Go or Terraform is missing.

---

## 2. Infrastructure & Application Deployment

Execute the deployment script to compile the application, validate Terraform templates, deploy the Azure Container App and private Blob Container, and verify health.

```bash
./scripts/deploy.sh dev
```

### Exact Actions Performed:
1. Compiles `cmd/server/main.go` into binary `bin/saas-server`.
2. Runs `terraform init -backend=false && terraform validate` in `infra/`.
3. Deploys the two assigned resources:
   - `azurerm_container_app.saas_app`
   - `azurerm_storage_container.evidence_blob` (private access)
4. Asserts `GET /health` returns `200 OK` revealing zero tenant data.
5. Runs a second `terraform plan -detailed-exitcode` to prove zero resource changes (Requirement A01).

**Expected Result**:
`GET /health` returns:
```json
{
  "status": "UP",
  "service": "configuration-comparison-saas"
}
```
Second plan outputs: `No changes. Your infrastructure matches the configuration.`

---

## 3. Acceptance Verification Suite (A01 - A12)

Run the automated acceptance suite to validate the entire SaaS contract, row validation rules, quota counters, and idempotency guarantees:

```bash
./scripts/verify.sh
```

### Checks Evaluated:
- **A01**: Infrastructure validation, public `/health` 200, no tenant data exposed, second plan has 0 changes.
- **A02**: Valid fixture validation reports `valid_rows: 2, invalid_rows: 0, issues: []`.
- **A03**: Invalid fixture validation reports `valid_rows: 2, invalid_rows: 4` with rule violations on `R1`, `R2`, `R3`, `R4`.
- **A04**: Pro `process` returns comparison summary (`input_rows: 2, changed_rows: 1, unchanged_rows: 1`) and rows; persists across app restart.
- **A05**: Invalid process input, wrong-shaped body, and 11-row batch return `400 Bad Request` with zero quota debit.
- **A06**: Unauthenticated requests return `401`; Viewer submissions and Basic `process` return `403` with zero quota debit.
- **A07**: Cross-tenant isolation enforced. Tenant B cannot list Tenant A runs; ID lookup returns `404`; tenant override in body/headers rejected.
- **A08**: Basic quota enforces 3 runs/day (4th rejected with `429`). Concurrency test with 1 slot left accepts exactly 1 request.
- **A09**: Replay of accepted key at exhausted quota returns original run (`200 OK`) with 0 debit. Reusing key with changed input returns `409 Conflict`. Concurrent identical keys create at most 1 run.
- **A10**: Suspended subscription rejects new submissions (`403 Forbidden`, 0 debit); historical reads succeed.
- **A11**: Forced failure hook marks run as `Failed`, records error message, and debits 1 slot. Replaying returns cached failure.
- **A12**: Reproduction evidence compiled and saved to `evidence/acceptance_report.md`.

**Stop Condition**: Any failing check in `./scripts/verify.sh` halts delivery.

---

## 4. Troubleshooting the Forced Failure Hook (A11)

When testing failure scenarios, the service supports injecting `X-Test-Force-Failure: true` after acceptance:

```bash
# Force a simulated post-acceptance failure
curl -i -X POST http://localhost:8080/runs \
  -H "Authorization: Bearer tenant-a-operator" \
  -H "Idempotency-Key: forced-fail-key-01" \
  -H "X-Test-Force-Failure: true" \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "validate",
    "records": [
      {"record_id": "max_retries", "expected": "3", "actual": "5", "environment": "TEST"},
      {"record_id": "region", "expected": "us", "actual": "us", "environment": "STAGE"}
    ]
  }'
```

### Diagnosis Steps:
1. **Verify HTTP Status**: Should return `201 Created` with payload:
   ```json
   {
     "run_id": "<uuid>",
     "status": "Failed",
     "error": "Simulated failure via shared test hook"
   }
   ```
2. **Verify Quota Consumption**: Ensure 1 slot was debited (as required by contract: "Every accepted unique run counts once, including a processing failure").
3. **Verify Idempotency Replay**: Re-submitting the identical payload with the same key without the header must return:
   - Status code: `200 OK`
   - Body: cached `Failed` run with same error message
   - Quota: zero additional debit.

---

## 5. Rollback Procedure (Using Prior Image)

If a newly deployed container revision introduces a regression, roll back immediately to the prior known-good image:

```bash
# Roll back using prior container image
PREV_IMAGE="ghcr.io/advantix/configguard:v0.9.0"

(cd infra && terraform apply \
  -var-file="environments/dev.tfvars" \
  -var="container_image=${PREV_IMAGE}" \
  -auto-approve)

# Verify health after rollback
./scripts/verify.sh
```

**Expected Result**:
Traffic is shifted 100% back to the prior revision; `/health` returns 200 OK.

---

## 6. Teardown & Destructive Cleanup

> [!CAUTION]
> This destroys the Azure Container App and private Blob Container. Explicit confirmation is required.

To clean up team-owned resources:

```bash
./scripts/cleanup.sh dev --confirm
```

### Verification:
1. Terraform destroy terminates the Container App and Blob Container.
2. Local databases and test artifacts (`test_*.db`, `saas.db`) are purged.
3. No lingering billable resources remain in the sandbox.
