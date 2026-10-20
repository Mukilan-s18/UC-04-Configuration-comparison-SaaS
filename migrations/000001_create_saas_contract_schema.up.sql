-- 000002_create_saas_contract_schema.up.sql
-- Schema for Configuration Comparison SaaS (Advantix UC-04)

CREATE TABLE IF NOT EXISTS tenants (
    tenant_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    plan VARCHAR(20) NOT NULL CHECK (plan IN ('basic', 'pro')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS runs (
    run_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
    idempotency_key VARCHAR(128) NOT NULL,
    operation VARCHAR(20) NOT NULL CHECK (operation IN ('validate', 'process')),
    input_fingerprint VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('Running', 'Succeeded', 'Failed')),
    result_json TEXT,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_idempotency UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_runs_tenant_created ON runs(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS daily_usage (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
    usage_date VARCHAR(10) NOT NULL, -- Format: YYYY-MM-DD (UTC)
    accepted_runs INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, usage_date)
);

-- Seed Tenant Fixtures
INSERT INTO tenants (tenant_id, name, plan, status)
VALUES 
    ('tenant-a', 'Enterprise Alpha (Tenant A)', 'pro', 'active'),
    ('tenant-b', 'Startup Beta (Tenant B)', 'basic', 'active'),
    ('tenant-suspended', 'Suspended Corp', 'pro', 'suspended')
ON CONFLICT (tenant_id) DO UPDATE 
SET plan = EXCLUDED.plan, status = EXCLUDED.status;
