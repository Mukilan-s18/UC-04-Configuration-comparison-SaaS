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
