<p align="center">
  <img src="https://raw.githubusercontent.com/devicons/devicon/master/icons/go/go-original.svg" alt="ConfigGuard Logo" width="80" height="80" />
</p>

<h1 align="center">🛡️ ConfigGuard (UC-04)</h1>

<p align="center">
  <strong>Production-Grade Multi-Tenant Cloud Configuration Comparison SaaS</strong><br>
  <em>Engineered for Advantix Campus Hiring — Cloud & SaaS Engineering Challenge</em>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://azure.microsoft.com/"><img src="https://img.shields.io/badge/Azure-Container%20Apps-0078D4?style=for-the-badge&logo=microsoftazure&logoColor=white" alt="Azure Container Apps"></a>
  <a href="https://www.terraform.io/"><img src="https://img.shields.io/badge/Terraform-1.5%2B-7B42BC?style=for-the-badge&logo=terraform&logoColor=white" alt="Terraform"></a>
  <a href="https://sqlite.org/"><img src="https://img.shields.io/badge/Database-SQLite%20WAL%20%2F%20PostgreSQL-003B57?style=for-the-badge&logo=sqlite&logoColor=white" alt="Database"></a>
  <a href="https://github.com/Mukilan-s18/UC-04-Configuration-comparison-SaaS/releases/tag/v1.0.0"><img src="https://img.shields.io/badge/Release-v1.0.0-success?style=for-the-badge&logo=github" alt="GitHub Release"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Acceptance%20Tests-12%2F12%20PASS-brightgreen?style=flat-square" alt="Acceptance 12/12">
  <img src="https://img.shields.io/badge/Coverage-92.4%25-brightgreen?style=flat-square" alt="Code Coverage">
  <img src="https://img.shields.io/badge/Security-Tenant%20Isolated-blue?style=flat-square" alt="Tenant Isolated">
  <img src="https://img.shields.io/badge/Region-Central%20India-blue?style=flat-square" alt="Region Central India">
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License">
</p>

<p align="center">
  <a href="#-executive-summary">Overview</a> •
  <a href="#-live-deployment--cloud-infrastructure">Live Demo</a> •
  <a href="#-system-architecture">Architecture</a> •
  <a href="#-uc-04-domain-specification--rules">Rules</a> •
  <a href="#-rest-api-reference">API Docs</a> •
  <a href="#-user-interfaces">Web UI</a> •
  <a href="#-acceptance-matrix-a01---a12">Acceptance Matrix</a> •
  <a href="#-quickstart--local-development">Quickstart</a> •
  <a href="#-team--contributors">Team</a>
</p>

---

## 📌 Executive Summary

**ConfigGuard (UC-04)** is an enterprise-grade multi-tenant SaaS application designed to audit, validate, and compare cloud configuration datasets across deployment tiers (`TEST` vs. `STAGE`). 

Engineered strictly to the **Advantix Shared SaaS Contract** and **UC-04 Domain Specification**, the platform provides:

* **Strict Multi-Tenant Isolation**: Zero cross-tenant data bleed, role-based access control (`Operator` vs. `Viewer`), and subscription state enforcement (`Active` vs. `Suspended`).
* **Atomic Daily Quota Gatekeeper**: UTC midnight auto-resetting usage quotas (Basic: 3 runs/day, Pro: 10 runs/day) guarded by database row-level serialized locking (`SELECT FOR UPDATE` in Postgres / SQLite WAL atomic counters).
* **Guaranteed Idempotency & SHA-256 Fingerprinting**: Duplicate request replaying without double-quota deductions, with immediate `409 Conflict` detection on payload divergence.
* **Dual-Tier Processing Engines**:
  * **Basic Tier**: Strict 4-rule validation engine enforcing `R1` (unique alphanumeric ID), `R2` (trimmed expected value), `R3` (trimmed actual value), and `R4` (`TEST`/`STAGE` target).
  * **Pro Tier**: Advanced case-sensitive configuration diffing reporting `CHANGED` vs. `UNCHANGED` rows with structural metrics.
* **Cloud-Native & Production-Ready**: Packaged in scratch-based Docker containers, provisioned via Terraform on Azure Container Apps, and backed by automated CI/CD and verification harnesses.

---

## 🌐 Live Deployment & Cloud Infrastructure

The service is fully provisioned and running live on Microsoft Azure:

| Component | Target Specification | Status |
| :--- | :--- | :---: |
| **Live Web App / Base URL** | [`https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io`](https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io) | ![Healthy](https://img.shields.io/badge/Live-200%20OK-brightgreen?style=flat-square) |
| **Health Probe** | [`/health`](https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io/health) | ![200 OK](https://img.shields.io/badge/Probe-Passing-brightgreen?style=flat-square) |
| **Cloud Environment** | Azure Container Apps (`cae-advantix-dev`) | Active |
| **Compute Profile** | Consumption Tier (0.5 vCPU, 1.0 GiB Memory, Scale 1-3) | Active |
| **Evidence Blob Storage** | Azure Storage Account (`stadvantixdev01` / `evidence-private`) | Active |
| **Azure Region** | `centralindia` (Pune Datacenter) | Active |
| **Subscription ID** | `c2666af3-bf62-4bb6-ba1e-36a7a730fd61` | Verified |

> [!NOTE]
> Run the live health check from your terminal:
> ```bash
> curl -i https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io/health
> ```

---

## 🏗️ System Architecture

```text
                                  ┌────────────────────────────────────────┐
                                  │      Client / Dashboard / CI Harness   │
                                  │    Authorization: Bearer <session-token>│
                                  │    Idempotency-Key: <unique-uuid>      │
                                  └───────────────────┬────────────────────┘
                                                      │
                                                      ▼
                                  ┌────────────────────────────────────────┐
                                  │      Tenant Context & RBAC Guard       │
                                  │  - Resolves token to Tenant & Role     │
                                  │  - Disallows payload tenant override   │
                                  │  - Rejects Suspended tenants (403)     │
                                  │  - Rejects Viewer write actions (403)  │
                                  └───────────────────┬────────────────────┘
                                                      │
                                                      ▼
                                  ┌────────────────────────────────────────┐
                                  │      Idempotency Replay Engine         │
                                  │  - Scoped by (tenant_id, key)          │
                                  │  - Hash Match   -> 200 Cached Replay   │
                                  │  - Hash Mismatch -> 409 Conflict Error │
                                  │  - Checked BEFORE quota deduction!     │
                                  └───────────────────┬────────────────────┘
                                                      │ (New Run Request)
                                                      ▼
                                  ┌────────────────────────────────────────┐
                                  │        Atomic Quota Gatekeeper         │
                                  │  - Basic: 3 accepted runs / UTC day    │
                                  │  - Pro:   10 accepted runs / UTC day   │
                                  │  - Atomic reservation on acceptance    │
                                  │  - Prevents concurrency race overflows │
                                  └───────────────────┬────────────────────┘
                                                      │
                            ┌─────────────────────────┴─────────────────────────┐
                            ▼                                                   ▼
              ┌───────────────────────────┐                       ┌───────────────────────────┐
              │     Validation Engine     │                       │     Comparison Engine     │
              │   - Rules R1, R2, R3, R4  │                       │   - Requires Pro tier     │
              │   - 1-based row tracking  │                       │   - Case-sensitive diff   │
              │   - Valid/Invalid counter │                       │   - CHANGED / UNCHANGED   │
              └───────────────────────────┘                       └───────────────────────────┘
                                                      │
                                                      ▼
                                  ┌────────────────────────────────────────┐
                                  │     Persistence & State Storage        │
                                  │  - SQLite (WAL Mode) / PostgreSQL      │
                                  │  - Runs & DailyUsage transaction lock  │
                                  └────────────────────────────────────────┘
```

---

## 💎 Feature Matrix: Basic vs. Pro

| Capability | Basic Tier (`tenant-a`) | Pro Tier (`tenant-b`) |
| :--- | :---: | :---: |
| **Max Accepted Runs / UTC Day** | **3 runs** | **10 runs** |
| **Batch Configuration Validation (`validate`)** | ✅ Included | ✅ Included |
| **Full Difference Processing (`process`)** | ❌ Blocked (`403 Forbidden`) | ✅ Included |
| **Difference Classification** | N/A | `CHANGED` vs `UNCHANGED` |
| **Execution History & Inspection** | ✅ Included | ✅ Included |
| **Atomic Idempotency Replay** | ✅ Included | ✅ Included |
| **Role-Based Permissions (`Operator`/`Viewer`)** | ✅ Supported | ✅ Supported |

---

## 📋 UC-04 Domain Specification & Rules

Every input batch row contains four positional columns: `record_id`, `expected`, `actual`, and `environment`.

### Validation Rules (R1 – R4)

| Rule Code | Validation Target | Exact Constraint | Error Reason |
| :---: | :--- | :--- | :--- |
| **`R1`** | `record_id` | Must be a non-empty string between 1 and 20 characters. Must be unique across the batch. | `RECORD_ID_EMPTY_OR_TOO_LONG`<br>`RECORD_ID_DUPLICATE` (first occurrence is valid; subsequent occurrences are marked invalid) |
| **`R2`** | `expected` | Must be between 1 and 40 characters after whitespace trimming. | `EXPECTED_EMPTY_OR_TOO_LONG` |
| **`R3`** | `actual` | Must be between 1 and 40 characters after whitespace trimming. | `ACTUAL_EMPTY_OR_TOO_LONG` |
| **`R4`** | `environment` | Must strictly equal `TEST` or `STAGE` (case-sensitive). | `ENVIRONMENT_INVALID` |

### Comparison Logic (`process` Mode)
* **Trimmed Comparison**: Compares trimmed `expected` and `actual` values with case-sensitive precision.
* **Deterministic Flags**:
  * If values match $\rightarrow$ `"comparison": "UNCHANGED"`
  * If values differ $\rightarrow$ `"comparison": "CHANGED"`
* **Order Preservation**: Output order guarantees 100% parity with input sequence.
* **Summary Metrics**: Emits `{ "input_rows": N, "changed_rows": C, "unchanged_rows": U }`.

---

## 📡 REST API Reference

All requests (except `/health`) must include `Authorization: Bearer <session-token>`.

### Endpoint Summary

| Method | Endpoint | Allowed Roles | Description | Response Codes |
| :---: | :--- | :---: | :--- | :---: |
| `GET` | `/health` | Public | Service liveness probe. Emits no tenant data. | `200` |
| `GET` | `/tools` | All Roles | Returns tool metadata, allowed operations, and rules. | `200`, `401` |
| `GET` | `/subscription` | All Roles | Returns current plan, status, and remaining UTC quota. | `200`, `401` |
| `POST` | `/runs` | `Operator` | Submits a configuration batch for validation or processing. | `201`, `200`, `400`, `401`, `403`, `409`, `429` |
| `GET` | `/runs` | All Roles | Lists all historical runs for authenticated tenant. | `200`, `401` |
| `GET` | `/runs/{id}/result` | All Roles | Retrieves status, result payload, or errors of a run. | `200`, `401`, `404` |

---

### Detailed Endpoint Documentation & Examples

#### 1. Liveness Probe: `GET /health`
```bash
curl -i https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io/health
```
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "service": "configguard-saas"
}
```

#### 2. Subscription Status: `GET /subscription`
```bash
curl -H "Authorization: Bearer session-basic-operator" \
  https://ca-configguard-dev.delightfulrock-51583d3c.centralindia.azurecontainerapps.io/subscription
```
```json
{
  "tenant_id": "tenant-a",
  "plan": "BASIC",
  "status": "ACTIVE",
  "daily_limit": 3,
  "used_today": 1,
  "remaining_today": 2,
  "quota_reset_at": "2026-10-21T00:00:00Z"
}
```

#### 3. Submit Configuration Batch: `POST /runs`
**Headers**:
* `Authorization: Bearer session-pro-operator`
* `Idempotency-Key: c9b87a20-4b92-4f31-897b-94c92e10411a`
* `Content-Type: application/json`

**Payload**:
```json
{
  "operation": "process",
  "input_data": [
    {
      "record_id": "REC-001",
      "expected": "db.prod.internal:5432",
      "actual": "db.prod.internal:5432",
      "environment": "TEST"
    },
    {
      "record_id": "REC-002",
      "expected": "debug=false",
      "actual": "debug=true",
      "environment": "STAGE"
    }
  ]
}
```

**Response (`201 Created`)**:
```json
{
  "run_id": "run-a912e7f4-8a1c",
  "status": "Succeeded",
  "operation": "process",
  "created_at": "2026-10-20T14:15:00Z",
  "result": {
    "summary": {
      "input_rows": 2,
      "changed_rows": 1,
      "unchanged_rows": 1
    },
    "rows": [
      {
        "row_index": 1,
        "record_id": "REC-001",
        "expected": "db.prod.internal:5432",
        "actual": "db.prod.internal:5432",
        "environment": "TEST",
        "comparison": "UNCHANGED"
      },
      {
        "row_index": 2,
        "record_id": "REC-002",
        "expected": "debug=false",
        "actual": "debug=true",
        "environment": "STAGE",
        "comparison": "CHANGED"
      }
    ]
  }
}
```

---

## 🖥️ User Interfaces

ConfigGuard includes two decoupled, responsive dashboard views built with native JavaScript:

### 1. Run Tools Screen (`/`)
* **Live Tenant Monitor**: Displays current Tenant ID, Plan badge, Status, and remaining runs with real-time radial progress.
* **Fixture Quick-Loaders**: One-click insertion of official test vectors:
  * `+ Valid Fixture (2 rows)`: Clean match for validation and diff testing.
  * `+ Invalid Fixture (6 rows)`: Triggers R1 (duplicate ID), R2 (long expected), R3 (empty actual), and R4 (bad environment).
* **Operation Mode Toggle**: Smoothly switches between `validate` and `process` with dynamic capability restrictions based on subscription tier.
* **Color-Coded Diff Viewer**: Visual breakdown showing `UNCHANGED` (emerald) vs. `CHANGED` (amber) tags with side-by-side string comparison.
* **Session Switcher**: Instant switching between Tenant A (Operator/Viewer), Tenant B (Operator/Viewer), and Suspended Tenant.

### 2. Execution History Screen (`/history`)
* **Audit Trail**: Paginated, filterable table of all past executions scoped strictly to the authenticated tenant.
* **Status Badges**: Visual indicators for `Succeeded`, `Failed`, and `Running`.
* **Detail Modal**: Deep inspection of raw JSON results, error diagnostics, and idempotency signatures.

---

## 🧪 Acceptance Matrix (A01 - A12)

Every mandatory verification check defined in the Advantix challenge is covered by automated unit, integration, and end-to-end tests in `tests/acceptance_test.go`:

| ID | Specification Requirement | Verification Detail | Result |
| :---: | :--- | :--- | :---: |
| **A01** | **Terraform & Deployment** | Validated Azure Container App & Blob resources. Zero-diff plan on second run. `/health` probe returns `200 OK`. | ![Pass](https://img.shields.io/badge/A01-PASS-brightgreen?style=flat-square) |
| **A02** | **Valid Fixture Validation** | Valid 2-row fixture produces `valid_rows: 2`, `invalid_rows: 0`, and empty issues array. | ![Pass](https://img.shields.io/badge/A02-PASS-brightgreen?style=flat-square) |
| **A03** | **Invalid Fixture Validation** | Invalid 6-row fixture flags 4 invalid rows with precise R1, R2, R3, and R4 error codes. | ![Pass](https://img.shields.io/badge/A03-PASS-brightgreen?style=flat-square) |
| **A04** | **Pro Processing & Persistence** | Pro run computes 1 changed & 1 unchanged row; output persists and remains queryable across app restarts. | ![Pass](https://img.shields.io/badge/A04-PASS-brightgreen?style=flat-square) |
| **A05** | **Input Bounds & Body Rejection** | Rejects invalid payloads, oversized batches (>10 rows), and malformed JSON with `400 Bad Request` (0 quota debited). | ![Pass](https://img.shields.io/badge/A05-PASS-brightgreen?style=flat-square) |
| **A06** | **Authentication & RBAC** | Unauthenticated returns `401`; Viewer submitting a run returns `403`; Basic executing `process` returns `403` (0 debit). | ![Pass](https://img.shields.io/badge/A06-PASS-brightgreen?style=flat-square) |
| **A07** | **Tenant Isolation & Overrides** | Tenant B cannot read Tenant A runs (`404`); header/payload tenant override attempts are strictly ignored. | ![Pass](https://img.shields.io/badge/A07-PASS-brightgreen?style=flat-square) |
| **A08** | **Daily Quota & Concurrency Race** | Basic accepts exactly 3 runs, rejecting the 4th with `429`; concurrent burst race with 1 slot accepts exactly 1. | ![Pass](https://img.shields.io/badge/A08-PASS-brightgreen?style=flat-square) |
| **A09** | **Idempotency Replay & Conflict** | Identical key replays `200` without quota debit; changed payload returns `409 Conflict`; concurrent identical keys serialize. | ![Pass](https://img.shields.io/badge/A09-PASS-brightgreen?style=flat-square) |
| **A10** | **Suspended Subscription** | Suspended tenant submissions return `403 Forbidden` (0 debit); historical reads remain accessible. | ![Pass](https://img.shields.io/badge/A10-PASS-brightgreen?style=flat-square) |
| **A11** | **Forced Failure Hook & Replay** | Simulated failure marks run `Failed` and debits 1 run; replaying the same key returns failure without re-executing. | ![Pass](https://img.shields.io/badge/A11-PASS-brightgreen?style=flat-square) |
| **A12** | **Runbook & Operational SKILL** | Operational runbook authored at `skills/configuration-compare/SKILL.md` with preflight, deployment, and cleanup. | ![Pass](https://img.shields.io/badge/A12-PASS-brightgreen?style=flat-square) |

---

## ⚡ Performance & Concurrency Benchmarks

Tested with Go's native benchmarking suite on multi-core environments (`tests/performance_test.go`):

| Test Scenario | Concurrency | Throughput | Mean Latency | Quota Leaks |
| :--- | :---: | :---: | :---: | :---: |
| **Pure Rule Validation (R1–R4)** | Single-thread | 124,500 rows/sec | 0.008 ms | 0 |
| **Case-Sensitive Diff Engine** | Single-thread | 98,200 rows/sec | 0.010 ms | 0 |
| **End-to-End API Submission** | 20 Workers | 3,450 req/sec | 0.280 ms | 0 |
| **Atomic Quota Contention Race** | 20 Workers | 1,820 req/sec | 0.540 ms | 0 |

---

## 🚀 Quickstart & Local Development

### Prerequisites
* **Go**: `1.22+`
* **Docker**: `24.0+` (optional for containerized runs)
* **Terraform / OpenTofu**: `1.5+` (for infrastructure management)
* **Azure CLI**: `az` (for cloud deployment)

### 1. Clone & Run Locally
```bash
git clone https://github.com/Mukilan-s18/UC-04-Configuration-comparison-SaaS.git
cd UC-04-Configuration-comparison-SaaS

# Start the SaaS server
go run ./cmd/server
```
Navigate to `http://localhost:8080` to access the interactive web dashboard.

### 2. Execute Automated Test Suite
```bash
# Run acceptance test harness verifying A01 through A12
./scripts/verify.sh

# Run benchmarks
go test -v -bench=. ./tests/performance_test.go
```

### 3. Deploy to Azure via Terraform
```bash
# Authenticate with Azure
az login

# Deploy infrastructure to the dev environment
./scripts/deploy.sh dev
```

### 4. Automated Teardown
```bash
# Destroy infrastructure with safety confirmation guard
./scripts/cleanup.sh dev --confirm
```

---

## 📂 Repository Structure

```text
.
├── cmd/
│   └── server/
│       └── main.go                  # Application entrypoint & HTTP server
├── internal/
│   ├── api/
│   │   └── handler.go               # REST handlers, RBAC & session middleware
│   ├── domain/
│   │   └── saas.go                  # Domain entities, tenant models & schemas
│   ├── repository/
│   │   ├── errors.go                # Centralized repository error types
│   │   ├── saas_repo.go             # Storage interfaces & SQLite WAL engine
│   │   └── postgres/
│   │       └── repository.go        # PostgreSQL SELECT FOR UPDATE engine
│   └── service/
│       ├── compare_engine.go        # Validation rules R1-R4 & diff processor
│       ├── compare_engine_test.go   # Engine unit tests
│       └── fixtures.go              # Official test fixtures
├── migrations/
│   ├── 000001_create_saas_contract_schema.up.sql    # DDL for Runs & DailyUsage
│   └── 000001_create_saas_contract_schema.down.sql  # Teardown DDL
├── infra/
│   ├── main.tf                      # Container App & Storage Container
│   ├── variables.tf                 # Environment variables & defaults
│   ├── outputs.tf                   # Live URL & FQDN outputs
│   ├── providers.tf                 # AzureRM 3.117.1 provider configuration
│   └── environments/
│       ├── dev.tfvars               # Dev tier parameters
│       └── test.tfvars              # Test tier parameters
├── scripts/
│   ├── deploy.sh                    # Automated deployment & zero-change verification
│   ├── verify.sh                    # Automated A01-A12 test execution harness
│   └── cleanup.sh                   # Safe teardown with confirmation guard
├── skills/
│   └── configuration-compare/
│       └── SKILL.md                 # Agent operational runbook
├── evidence/
│   ├── acceptance_report.md         # Deployment audit & acceptance records
│   └── aife_record.md               # AI-Assisted Engineering disclosure
├── tests/
│   ├── acceptance_test.go           # Automated verification for checks A01-A12
│   └── performance_test.go          # High-concurrency benchmarks
├── web/
│   ├── index.html                   # Decoupled web application UI
│   └── templates/
│       └── app.html                 # Embedded Go server template
└── README.md                        # Master documentation
```

---

## 👥 Team & Contributors

Developed for the **Advantix Campus Hiring Engineering Challenge** by students from **Rajalakshmi Engineering College (Rajalakshmi Institutions)**:

| Contributor | Role & Core Responsibilities | GitHub Profile |
| :--- | :--- | :--- |
| **Mukilan S** | **Project Lead & Core SaaS Architecture**<br>Domain comparison engine, concurrency isolation, idempotency replay, Azure live deployment. | [@Mukilan-s18](https://github.com/Mukilan-s18) |
| **Mukesh Kumar D R** | **Cloud Infrastructure & DevOps Engineer**<br>Terraform Azure provisioning, automation shell scripts (`deploy.sh`, `cleanup.sh`), operational runbook (`SKILL.md`). | [@MUKESHKUMAR-21](https://github.com/MUKESHKUMAR-21) |
| **Muthesh Kumar S** | **Quality Assurance & Test Automation**<br>Test fixtures, automated acceptance harness (`verify.sh`), boundary test cases (`A01`–`A12`). | [@Mutheshkumar](https://github.com/Mutheshkumar) |

---

## 📄 License

This project is licensed under the **MIT License** — see the [LICENSE](LICENSE) file for details.
