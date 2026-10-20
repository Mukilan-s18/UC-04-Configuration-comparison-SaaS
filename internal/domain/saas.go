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
