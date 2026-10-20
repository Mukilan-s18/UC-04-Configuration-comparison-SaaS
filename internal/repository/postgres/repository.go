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
