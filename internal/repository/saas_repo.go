package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"configguard/internal/domain"

	_ "modernc.org/sqlite"
)

// Repository error definitions are declared in errors.go.

type SaasRepository struct {
	db *sql.DB
	mu sync.Mutex // guarantees atomic serializability across concurrent goroutines for SQLite
}

func NewSaasRepository(db *sql.DB) *SaasRepository {
	return &SaasRepository{db: db}
}

// InitSchema ensures the tables and seeds exist.
func (r *SaasRepository) InitSchema(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			tenant_id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			plan VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS runs (
			run_id VARCHAR(64) PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL,
			idempotency_key VARCHAR(128) NOT NULL,
			operation VARCHAR(20) NOT NULL,
			input_fingerprint VARCHAR(64) NOT NULL,
			status VARCHAR(20) NOT NULL,
			result_json TEXT,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_tenant_idempotency UNIQUE (tenant_id, idempotency_key)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_tenant ON runs(tenant_id);`,
		`CREATE TABLE IF NOT EXISTS daily_usage (
			tenant_id VARCHAR(64) NOT NULL,
			usage_date VARCHAR(10) NOT NULL,
			accepted_runs INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (tenant_id, usage_date)
		);`,
	}

	for _, q := range queries {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	// Seed standard tenants if not present
	seeds := []struct {
		ID     string
		Name   string
		Plan   string
		Status string
	}{
		{"tenant-a", "Enterprise Alpha (Tenant A)", domain.PlanPro, domain.StatusActive},
		{"tenant-b", "Startup Beta (Tenant B)", domain.PlanBasic, domain.StatusActive},
		{"tenant-suspended", "Suspended Corp", domain.PlanPro, domain.StatusSuspended},
	}

	for _, s := range seeds {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO tenants (tenant_id, name, plan, status)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(tenant_id) DO UPDATE SET plan=excluded.plan, status=excluded.status
		`, s.ID, s.Name, s.Plan, s.Status)
		if err != nil {
			// Try postgres syntax if on conflict failed or fallback
			_, _ = r.db.ExecContext(ctx, `
				INSERT INTO tenants (tenant_id, name, plan, status)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT(tenant_id) DO UPDATE SET plan=EXCLUDED.plan, status=EXCLUDED.status
			`, s.ID, s.Name, s.Plan, s.Status)
		}
	}

	return nil
}

// GetTenant retrieves a tenant by ID.
func (r *SaasRepository) GetTenant(ctx context.Context, tenantID string) (*domain.SessionContext, error) {
	row := r.db.QueryRowContext(ctx, `SELECT tenant_id, plan, status FROM tenants WHERE tenant_id = ?`, tenantID)
	var sc domain.SessionContext
	if err := row.Scan(&sc.TenantID, &sc.Plan, &sc.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sc, nil
}

// SetTenantStatus updates a tenant's subscription status (for harness testing A10).
func (r *SaasRepository) SetTenantStatus(ctx context.Context, tenantID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	res, err := r.db.ExecContext(ctx, `UPDATE tenants SET status = ? WHERE tenant_id = ?`, status, tenantID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckExistingReplay checks if an idempotency key was previously submitted for this tenant.
func (r *SaasRepository) CheckExistingReplay(ctx context.Context, tenantID, idempotencyKey string) (*domain.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at
		FROM runs
		WHERE tenant_id = ? AND idempotency_key = ?
	`, tenantID, idempotencyKey)

	var run domain.Run
	var resultJSON, errMsg sql.NullString
	if err := row.Scan(&run.RunID, &run.TenantID, &run.IdempotencyKey, &run.Operation, &run.InputFingerprint, &run.Status, &resultJSON, &errMsg, &run.CreatedAt, &run.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No existing replay
		}
		return nil, err
	}

	if resultJSON.Valid {
		run.Result = json.RawMessage(resultJSON.String)
	}
	if errMsg.Valid {
		run.ErrorMessage = errMsg.String
	}
	return &run, nil
}

// ReserveSlotAndCreateRun atomically checks tenant quota, debits 1 slot, and persists the run in Running state.
func (r *SaasRepository) ReserveSlotAndCreateRun(ctx context.Context, run *domain.Run, maxQuota int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	todayUTC := time.Now().UTC().Format("2006-01-02")

	// 1. Check current daily usage
	var currentAccepted int
	err = tx.QueryRowContext(ctx, `
		SELECT accepted_runs FROM daily_usage WHERE tenant_id = ? AND usage_date = ?
	`, run.TenantID, todayUTC).Scan(&currentAccepted)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if currentAccepted >= maxQuota {
		return ErrQuotaExceeded
	}

	// 2. Increment quota counter
	newAccepted := currentAccepted + 1
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO daily_usage (tenant_id, usage_date, accepted_runs, created_at, updated_at)
			VALUES (?, ?, 1, ?, ?)
		`, run.TenantID, todayUTC, time.Now().UTC(), time.Now().UTC())
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE daily_usage SET accepted_runs = ?, updated_at = ? WHERE tenant_id = ? AND usage_date = ?
		`, newAccepted, time.Now().UTC(), run.TenantID, todayUTC)
	}
	if err != nil {
		return err
	}

	// 3. Persist run with Running status
	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	run.Status = domain.RunStatusRunning

	_, err = tx.ExecContext(ctx, `
		INSERT INTO runs (run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, run.RunID, run.TenantID, run.IdempotencyKey, run.Operation, run.InputFingerprint, run.Status, run.CreatedAt, run.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "constraint") {
			return ErrKeyCollision
		}
		return err
	}

	return tx.Commit()
}

// FinishRun updates the run to terminal status (Succeeded or Failed).
func (r *SaasRepository) FinishRun(ctx context.Context, runID, status string, resultJSON []byte, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	var resStr, errStr sql.NullString
	if len(resultJSON) > 0 {
		resStr = sql.NullString{String: string(resultJSON), Valid: true}
	}
	if errMsg != "" {
		errStr = sql.NullString{String: errMsg, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE runs 
		SET status = ?, result_json = ?, error_message = ?, updated_at = ?
		WHERE run_id = ?
	`, status, resStr, errStr, now, runID)
	return err
}

// GetRunByID fetches a run strictly enforcing tenant ownership.
func (r *SaasRepository) GetRunByID(ctx context.Context, tenantID, runID string) (*domain.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at
		FROM runs
		WHERE tenant_id = ? AND run_id = ?
	`, tenantID, runID)

	var run domain.Run
	var resultJSON, errMsg sql.NullString
	if err := row.Scan(&run.RunID, &run.TenantID, &run.IdempotencyKey, &run.Operation, &run.InputFingerprint, &run.Status, &resultJSON, &errMsg, &run.CreatedAt, &run.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if resultJSON.Valid {
		run.Result = json.RawMessage(resultJSON.String)
	}
	if errMsg.Valid {
		run.ErrorMessage = errMsg.String
	}
	return &run, nil
}

// ListRuns fetches all runs for a specific tenant in reverse chronological order.
func (r *SaasRepository) ListRuns(ctx context.Context, tenantID string) ([]domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at
		FROM runs
		WHERE tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		var run domain.Run
		var resultJSON, errMsg sql.NullString
		if err := rows.Scan(&run.RunID, &run.TenantID, &run.IdempotencyKey, &run.Operation, &run.InputFingerprint, &run.Status, &resultJSON, &errMsg, &run.CreatedAt, &run.UpdatedAt); err != nil {
			return nil, err
		}
		if resultJSON.Valid {
			run.Result = json.RawMessage(resultJSON.String)
		}
		if errMsg.Valid {
			run.ErrorMessage = errMsg.String
		}
		runs = append(runs, run)
	}
	if runs == nil {
		runs = []domain.Run{}
	}
	return runs, nil
}

// GetSubscriptionInfo returns current plan, subscription status, and UTC quota details.
func (r *SaasRepository) GetSubscriptionInfo(ctx context.Context, tenantID string) (*domain.SubscriptionInfo, error) {
	tenant, err := r.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	quotaLimit := domain.ProDailyQuota
	if tenant.Plan == domain.PlanBasic {
		quotaLimit = domain.BasicDailyQuota
	}

	todayUTC := time.Now().UTC().Format("2006-01-02")
	var quotaUsed int
	row := r.db.QueryRowContext(ctx, `
		SELECT accepted_runs FROM daily_usage WHERE tenant_id = ? AND usage_date = ?
	`, tenantID, todayUTC)
	if err := row.Scan(&quotaUsed); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	quotaRemaining := quotaLimit - quotaUsed
	if quotaRemaining < 0 {
		quotaRemaining = 0
	}

	return &domain.SubscriptionInfo{
		TenantID:       tenantID,
		Plan:           tenant.Plan,
		Status:         tenant.Status,
		QuotaLimit:     quotaLimit,
		QuotaUsed:      quotaUsed,
		QuotaRemaining: quotaRemaining,
		DateUTC:        todayUTC,
	}, nil
}

// ResetHarness clears runs and daily quota counters (used by test harness between checks).
func (r *SaasRepository) ResetHarness(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.db.ExecContext(ctx, `DELETE FROM runs;`); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM daily_usage;`); err != nil {
		return err
	}
	// Reset tenant statuses
	if _, err := r.db.ExecContext(ctx, `UPDATE tenants SET status = 'active' WHERE tenant_id IN ('tenant-a', 'tenant-b');`); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE tenants SET status = 'suspended' WHERE tenant_id = 'tenant-suspended';`); err != nil {
		return err
	}
	return nil
}
