# AI-Assisted Engineering (AIFE) Record

**Event**: Advantix | Rajalakshmi University Campus Hiring — Cloud and SaaS Engineering Challenges  
**Use Case**: UC-04 (Configuration comparison SaaS)  
**Service Name**: ConfigGuard  
**Timestamp**: 2026-10-20  

---

## 1. Executive Summary & Compliance
In accordance with Page 4 of the official Advantix challenge specification:
> *"Record useful AI assistance, corrections and verification as part of AIFE."*

This document provides a factual, transparent record of how generative AI pair programming assistance was leveraged, corrected, and verified across all four build dimensions (Cloud, App, SaaS Contract, and Automation).

---

## 2. Itemized AI Assistance & Implementations

### A. Postgres Concurrency Controls & Row-Level Locking (A08)
- **AI Contribution**: 
  - Designed the atomic transaction pattern in PostgreSQL using `SELECT ... FOR UPDATE` row-level locks on the `daily_usage` table.
  - Implemented the double-debiting guard in `ReserveSlotAndCreateRun` ensuring that under high concurrent contention (with 1 quota slot remaining), exactly one request receives `201 Created` and all concurrent racers receive `429 Too Many Requests`.
- **Human/Engineering Corrections**:
  - Identified and corrected column schema discrepancies where initial suggestions referenced `result_data` and `utc_date` instead of the canonical migration schema columns `result_json`, `error_message`, `usage_date`, and `accepted_runs`.
  - Replaced multi-statement query batching with separated atomic `INSERT ... ON CONFLICT DO NOTHING` followed by `SELECT ... FOR UPDATE` to avoid pgx prepared statement collisions.
- **Verification**:
  - Passed `TestA08_BasicQuotaAndConcurrency` simulating concurrent goroutines competing for the final quota slot.

---

### B. Validation Rules (R1–R4) & Pure Comparator Engine (A02, A03, A04)
- **AI Contribution**:
  - Mapped the exact business rules R1 through R4 in `CompareEngine.ValidateBatch`:
    - **R1**: `record_id` non-empty string $\le 20$ chars, unique within batch (case-sensitive; only later occurrences flagged as duplicate).
    - **R2**: `expected` string 1 to 40 characters after space trimming.
    - **R3**: `actual` string 1 to 40 characters after space trimming.
    - **R4**: `environment` strictly `TEST` or `STAGE`.
  - Constructed the pure comparison helper `CompareEngine.ProcessBatch` performing case-sensitive equality on trimmed strings, outputting `"CHANGED"` or `"UNCHANGED"`, and preserving row order.
- **Human/Engineering Corrections**:
  - Ensured invalid row counting strictly deduplicates multi-rule failures on the same row (`invalid_rows` counts distinct rows with issues).
  - Explicitly returned empty arrays (`issues: []`) rather than `null` in JSON output to ensure strict compliance with automated testing harnesses.
- **Verification**:
  - Valid fixture (2 rows): Validated 2 valid, 0 invalid; Processed: 1 CHANGED, 1 UNCHANGED.
  - Invalid fixture (6 rows): Validated 2 valid, 4 invalid, flagging exactly R1, R2, R3, R4.
  - Passed `TestA02_ValidateValidFixture`, `TestA03_ValidateInvalidFixture`, and `TestA04_ProProcessAndPersistenceAcrossRestart`.

---

### C. Vanilla JavaScript Frontend & HTMX Decoupling
- **AI Contribution**:
  - Built the two required screens in `web/index.html` (Run Tools and Execution History) using pure static HTML and native JavaScript `fetch()` calls.
  - Directly decoupled the frontend from server-side rendering, ensuring that all API endpoints (`/runs`, `/subscription`, `/health`) remain pure JSON endpoints.
- **Rationale**:
  - Avoided HTMX fragment-swapping libraries that would expect HTML responses and break automated evaluator testing suites expecting strict JSON bodies.
- **Human/Engineering Corrections**:
  - Enhanced authorization header extraction in `AuthenticateSession` to automatically handle both raw token formats (`tenant-a-operator`) and standard `Bearer <token>` prefixes.
  - Implemented automatic UI quota meter updates, random idempotency key generation, and quick-load buttons for official fixtures.
- **Verification**:
  - Tested live end-to-end execution loop in browser and CLI, confirming quota updates from 10 to 9 upon submission and automatic population of execution history.

---

### D. Infrastructure Scaffolding & Automation Runbook
- **AI Contribution**:
  - Configured the two assigned Terraform resources in `infra/main.tf` (`azurerm_container_app` and `azurerm_storage_container`).
  - Authored the operational runbook in `skills/configuration-compare/SKILL.md` with complete YAML frontmatter, preflight checks, step-by-step deploy/verify commands, zero-downtime rollback with prior container image, and explicit confirmation guards for destructive cleanup.
- **Human/Engineering Corrections**:
  - Fixed Terraform AzureRM provider container memory parameter validation error (`1.0Gi` changed to `1Gi`).
  - Implemented `-detailed-exitcode` on second `terraform plan` to programmatically prove 0 resource changes (Requirement A01).
- **Verification**:
  - Verified `terraform validate` and `terraform plan` pass cleanly.
  - Automated verification via `./scripts/verify.sh` running tests A01 through A12 with redacted token output.

---

## 3. Summary Table

| Build Area | AI Assistance Provided | Verification Evidence |
| :--- | :--- | :--- |
| **Concurrency & Storage** | Postgres `FOR UPDATE` lock; SQLite WAL serializability; idempotency tracking. | `TestA08_BasicQuotaAndConcurrency`, `TestA09_ExhaustedQuotaReplayAndKeyConflict` |
| **Engine & Logic** | Implemented R1–R4 validation rules and pure diff comparator. | `TestA02_ValidateValidFixture`, `TestA03_ValidateInvalidFixture`, `TestA04_ProProcessAndPersistenceAcrossRestart` |
| **Frontend UI** | Native JS decoupled UI (`web/index.html`) serving Screen 1 (Run Tools) & Screen 2 (History). | End-to-end browser & curl execution loop verified |
| **DevOps & Runbook** | Terraform templates, `deploy.sh`, `verify.sh`, `cleanup.sh`, and `SKILL.md`. | `terraform validate`, `scripts/verify.sh` (A01–A12 PASS) |
