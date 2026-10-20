package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Subscription Plans
const (
	PlanBasic = "basic"
	PlanPro   = "pro"
)

// Roles
const (
	RoleOperator = "Operator"
	RoleViewer   = "Viewer"
)

// Subscription Status
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

// Run Operations
const (
	OpValidate = "validate"
	OpProcess  = "process"
)

// Run Status
const (
	RunStatusRunning   = "Running"
	RunStatusSucceeded = "Succeeded"
	RunStatusFailed    = "Failed"
)

// Quota Limits per UTC Day
const (
	BasicDailyQuota = 3
	ProDailyQuota   = 10
)

// UC04Record represents a single configuration record.
type UC04Record struct {
	RecordID    string `json:"record_id"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Environment string `json:"environment"`
}

// RunRequest is the payload sent to POST /runs.
type RunRequest struct {
	Operation string       `json:"operation"`
	Records   []UC04Record `json:"records"`
}

// ComputeFingerprint generates a canonical SHA256 hex string for the operation and records.
func (r *RunRequest) ComputeFingerprint() string {
	b, _ := json.Marshal(struct {
		Operation string       `json:"operation"`
		Records   []UC04Record `json:"records"`
	}{
		Operation: r.Operation,
		Records:   r.Records,
	})
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

// Issue represents a single validation error on a row.
type Issue struct {
	RowIndex int    `json:"row_index"` // 1-based
	Field    string `json:"field"`
	RuleID   string `json:"rule_id"`
}

// ValidationResult is returned by validate operation.
type ValidationResult struct {
	ValidRows   int     `json:"valid_rows"`
	InvalidRows int     `json:"invalid_rows"`
	Issues      []Issue `json:"issues"`
}

// ProcessedRow represents an evaluated row in a process run.
type ProcessedRow struct {
	RecordID    string `json:"record_id"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Environment string `json:"environment"`
	Comparison  string `json:"comparison"` // "CHANGED" or "UNCHANGED"
}

// ProcessSummary holds summary counts for a process run.
type ProcessSummary struct {
	InputRows     int `json:"input_rows"`
	ChangedRows   int `json:"changed_rows"`
	UnchangedRows int `json:"unchanged_rows"`
}

// ProcessResult is the payload saved on successful process operation.
type ProcessResult struct {
	Summary ProcessSummary `json:"summary"`
	Rows    []ProcessedRow `json:"rows"`
	Issues  []Issue        `json:"issues"`
}

// Run represents a persisted run record in the database.
type Run struct {
	RunID            string          `json:"run_id"`
	TenantID         string          `json:"tenant_id,omitempty"`
	IdempotencyKey   string          `json:"idempotency_key,omitempty"`
	Operation        string          `json:"operation"`
	InputFingerprint string          `json:"input_fingerprint,omitempty"`
	Status           string          `json:"status"`
	Result           json.RawMessage `json:"result,omitempty"`
	ErrorMessage     string          `json:"error,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// RunResponse is the standard response returned for POST /runs and GET /runs/{id}/result.
type RunResponse struct {
	RunID  string          `json:"run_id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// SubscriptionInfo represents the tenant's current subscription and quota status.
type SubscriptionInfo struct {
	TenantID       string `json:"tenant_id"`
	Plan           string `json:"plan"`
	Status         string `json:"status"`
	QuotaLimit     int    `json:"quota_limit"`
	QuotaUsed      int    `json:"quota_used"`
	QuotaRemaining int    `json:"quota_remaining"`
	DateUTC        string `json:"date_utc"`
}

// SessionContext represents the authenticated user and tenant session.
type SessionContext struct {
	TenantID string
	Role     string // "Operator" or "Viewer"
	Plan     string // "basic" or "pro"
	Status   string // "active" or "suspended"
}
