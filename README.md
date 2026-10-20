# Advantix | UC-04: Configuration Comparison SaaS

> Cloud and SaaS Engineering Challenge — Revision 2 Common SaaS Scope.  
> Service that checks environment settings and reports differences between expected and actual values.

---

## 🌟 Key Features

* **Strict Validation (R1–R4)**:
  * **R1**: `record_id` is a non-empty string of $\le 20$ characters, unique within batch.
  * **R2**: `expected` is a string of 1 to 40 characters after trimming whitespace.
  * **R3**: `actual` is a string of 1 to 40 characters after trimming whitespace.
  * **R4**: `environment` is `TEST` or `STAGE`.
* **Pro Comparison Engine**:
  * Case-sensitive equality check producing `CHANGED` or `UNCHANGED` classifications.
  * Batch summary calculation (`input_rows`, `changed_rows`, `unchanged_rows`).
* **SaaS Multi-Tenant Contract**:
  * **Tenants**: Tenant A (Pro plan, 10 runs/UTC day) & Tenant B (Basic plan, 3 runs/UTC day, validate only).
  * **Roles**: `Operator` (submit & read history) and `Viewer` (read-only history).
  * **Atomic Quota Reservation**: Slots debited upon acceptance; rejected batches do not debit quota.
  * **Request Replay & Idempotency**: Strict tenant-scoped `Idempotency-Key` header with terminal caching and fingerprint comparison.
  * **Forced Failure Hook**: `X-Test-Force-Failure: true` hook for testability (A11).
* **Enterprise Web Dashboard**:
  * **Screen 1 (Run Tools)**: Real-time quota gauge, preset fixtures, JSON batch editor, and live execution.
  * **Screen 2 (Run History & Diff Inspector)**: Audit logs, status badges, visual diff breakdown, and **CSV Export** for audit compliance.
* **100% Acceptance Pass (A01 - A12)**:
  * Full Go test suite validating all 12 rubric requirements.

---

## 🚀 Quickstart

### Prerequisites
* Go 1.22+
* Terraform 1.5+ (optional for Azure deployment)
* Azure CLI (optional for cloud deployment)

### Running Locally
```bash
# Start server (defaults to port 8080 with SQLite storage)
go run ./cmd/server/main.go
```
Open [http://localhost:8080](http://localhost:8080) to interact with the SaaS dashboard.

### Running Acceptance Tests
```bash
# Run all tests (A01 - A12)
go test -v ./tests/...

# Or run the verification harness
./scripts/verify.sh
# or on Windows PowerShell:
.\scripts\verify.ps1
```

---

## 📡 API Contract

| Route | Method | Access | Description |
| :--- | :---: | :--- | :--- |
| `/health` | `GET` | Public | Returns service health status without exposing tenant data. |
| `/tools` | `GET` | Protected | Returns metadata and supported operations (`validate`, `process`). |
| `/subscription` | `GET` | Protected | Returns tenant plan, status, and remaining UTC daily quota. |
| `/runs` | `POST` | Protected (`Operator`) | Atomically reserves quota, checks idempotency, and executes batch. |
| `/runs` | `GET` | Protected | Lists execution history strictly for the authenticated tenant. |
| `/runs/{id}/result` | `GET` | Protected | Retrieves result or recorded error for a run ID. |

---

## 👥 Team
* **Use Case**: UC-04 (Configuration Comparison SaaS)
* **Team Branch**: `Mukesh`
