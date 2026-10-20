package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"configguard/internal/domain"
	"configguard/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SaasRepository provides high-performance PostgreSQL operations with row-level locks
// for the Advantix UC-04 Configuration Comparison SaaS contract.
type SaasRepository struct {
	pool *pgxpool.Pool
}

// NewSaasRepository creates a new SaasRepository backed by pgxpool.
func NewSaasRepository(ctx context.Context, connString string) (*SaasRepository, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &SaasRepository{pool: pool}, nil
}

func (r *SaasRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

func (r *SaasRepository) Ping(ctx context.Context) error {
	if r.pool == nil {
		return fmt.Errorf("postgres pool is not initialized")
	}
	return r.pool.Ping(ctx)
}

// InitSchema ensures the UC-04 schema tables and default seeds exist in PostgreSQL.
func (r *SaasRepository) InitSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			tenant_id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			plan VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS runs (
			run_id VARCHAR(64) PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
			idempotency_key VARCHAR(128) NOT NULL,
			operation VARCHAR(20) NOT NULL,
			input_fingerprint VARCHAR(64) NOT NULL,
			status VARCHAR(20) NOT NULL,
			result_json TEXT,
			error_message TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_tenant_idempotency UNIQUE (tenant_id, idempotency_key)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_tenant_created ON runs(tenant_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS daily_usage (
			tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
			usage_date VARCHAR(10) NOT NULL,
			accepted_runs INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (tenant_id, usage_date)
		);`,
		`INSERT INTO tenants (tenant_id, name, plan, status)
		VALUES 
			('tenant-a', 'Enterprise Alpha (Tenant A)', 'pro', 'active'),
			('tenant-b', 'Startup Beta (Tenant B)', 'basic', 'active'),
			('tenant-suspended', 'Suspended Corp', 'pro', 'suspended')
		ON CONFLICT (tenant_id) DO UPDATE 
		SET plan = EXCLUDED.plan, status = EXCLUDED.status;`,
	}
	for _, q := range queries {
		if _, err := r.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}
	return nil
}

// CheckExistingReplay checks if an idempotency key was previously submitted for this tenant.
func (r *SaasRepository) CheckExistingReplay(ctx context.Context, tenantID, idempotencyKey string) (*domain.Run, error) {
	query := `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at
		FROM runs 
		WHERE tenant_id = $1 AND idempotency_key = $2
	`
	var run domain.Run
	var resultJSON, errMessage *string
	err := r.pool.QueryRow(ctx, query, tenantID, idempotencyKey).Scan(
		&run.RunID,
		&run.TenantID,
		&run.IdempotencyKey,
		&run.Operation,
		&run.InputFingerprint,
		&run.Status,
		&resultJSON,
		&errMessage,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No existing replay
		}
		return nil, err
	}

	if resultJSON != nil {
		run.Result = []byte(*resultJSON)
	}
	if errMessage != nil {
		run.ErrorMessage = *errMessage
	}

	return &run, nil
}

// ReserveSlotAndCreateRun atomically reserves a quota slot using row-level FOR UPDATE locking.
// Concurrency Check A08: Prevents double-debiting under simultaneous requests.
func (r *SaasRepository) ReserveSlotAndCreateRun(ctx context.Context, run *domain.Run, maxQuota int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	todayUTC := time.Now().UTC().Format("2006-01-02")

	// 1. Ensure daily_usage row exists for tenant and today's date
	_, err = tx.Exec(ctx, `
		INSERT INTO daily_usage (tenant_id, usage_date, accepted_runs, created_at, updated_at)
		VALUES ($1, $2, 0, NOW(), NOW())
		ON CONFLICT (tenant_id, usage_date) DO NOTHING
	`, run.TenantID, todayUTC)
	if err != nil {
		return fmt.Errorf("failed to ensure daily_usage row: %w", err)
	}

	// 2. Lock the specific row for update (A08 Concurrency Guarantee)
	var currentRuns int
	err = tx.QueryRow(ctx, `
		SELECT accepted_runs 
		FROM daily_usage 
		WHERE tenant_id = $1 AND usage_date = $2 
		FOR UPDATE
	`, run.TenantID, todayUTC).Scan(&currentRuns)
	if err != nil {
		return fmt.Errorf("failed to lock usage row: %w", err)
	}

	// 3. Enforce quota
	if currentRuns >= maxQuota {
		return repository.ErrQuotaExceeded
	}

	// 4. Increment quota
	_, err = tx.Exec(ctx, `
		UPDATE daily_usage 
		SET accepted_runs = accepted_runs + 1, updated_at = NOW() 
		WHERE tenant_id = $1 AND usage_date = $2
	`, run.TenantID, todayUTC)
	if err != nil {
		return fmt.Errorf("failed to increment quota: %w", err)
	}

	// 5. Insert run with Running status
	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	run.Status = domain.RunStatusRunning

	_, err = tx.Exec(ctx, `
		INSERT INTO runs (run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, run.RunID, run.TenantID, run.IdempotencyKey, run.Operation, run.InputFingerprint, run.Status, run.CreatedAt, run.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique") {
			return repository.ErrKeyCollision
		}
		return fmt.Errorf("failed to insert run: %w", err)
	}

	return tx.Commit(ctx)
}

// FinishRun updates the run to terminal status (Succeeded or Failed).
func (r *SaasRepository) FinishRun(ctx context.Context, runID, status string, result []byte, errMsg string) error {
	var resStr, errStr *string
	if len(result) > 0 {
		s := string(result)
		resStr = &s
	}
	if errMsg != "" {
		errStr = &errMsg
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE runs 
		SET status = $1, result_json = $2, error_message = $3, updated_at = NOW()
		WHERE run_id = $4
	`, status, resStr, errStr, runID)
	return err
}

// GetRunByID fetches a run strictly enforcing tenant ownership.
func (r *SaasRepository) GetRunByID(ctx context.Context, tenantID, runID string) (*domain.Run, error) {
	var run domain.Run
	var resData, errData *string

	err := r.pool.QueryRow(ctx, `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at 
		FROM runs 
		WHERE tenant_id = $1 AND run_id = $2
	`, tenantID, runID).Scan(
		&run.RunID,
		&run.TenantID,
		&run.IdempotencyKey,
		&run.Operation,
		&run.InputFingerprint,
		&run.Status,
		&resData,
		&errData,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	if resData != nil {
		run.Result = []byte(*resData)
	}
	if errData != nil {
		run.ErrorMessage = *errData
	}

	return &run, nil
}

// ListRuns lists all historical runs for the authenticated tenant.
func (r *SaasRepository) ListRuns(ctx context.Context, tenantID string) ([]domain.Run, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT run_id, tenant_id, idempotency_key, operation, input_fingerprint, status, result_json, error_message, created_at, updated_at
		FROM runs 
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		var run domain.Run
		var resData, errData *string
		if err := rows.Scan(
			&run.RunID,
			&run.TenantID,
			&run.IdempotencyKey,
			&run.Operation,
			&run.InputFingerprint,
			&run.Status,
			&resData,
			&errData,
			&run.CreatedAt,
			&run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if resData != nil {
			run.Result = []byte(*resData)
		}
		if errData != nil {
			run.ErrorMessage = *errData
		}
		runs = append(runs, run)
	}
	if runs == nil {
		runs = []domain.Run{}
	}
	return runs, nil
}

// GetTenant retrieves a tenant's session context by ID.
func (r *SaasRepository) GetTenant(ctx context.Context, tenantID string) (*domain.SessionContext, error) {
	var sc domain.SessionContext
	err := r.pool.QueryRow(ctx, `
		SELECT tenant_id, plan, status 
		FROM tenants 
		WHERE tenant_id = $1
	`, tenantID).Scan(&sc.TenantID, &sc.Plan, &sc.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &sc, nil
}

// SetTenantStatus updates a tenant's subscription status.
func (r *SaasRepository) SetTenantStatus(ctx context.Context, tenantID, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenants SET status = $1 WHERE tenant_id = $2`, status, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
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
	err = r.pool.QueryRow(ctx, `
		SELECT accepted_runs 
		FROM daily_usage 
		WHERE tenant_id = $1 AND usage_date = $2
	`, tenantID, todayUTC).Scan(&quotaUsed)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
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
	if _, err := r.pool.Exec(ctx, `DELETE FROM runs;`); err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM daily_usage;`); err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, `UPDATE tenants SET status = 'active' WHERE tenant_id IN ('tenant-a', 'tenant-b');`); err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, `UPDATE tenants SET status = 'suspended' WHERE tenant_id = 'tenant-suspended';`); err != nil {
		return err
	}
	return nil
}

// Type alias for compatibility.
type Repository = SaasRepository

// NewRepository creates a new SaasRepository.
var NewRepository = NewSaasRepository
