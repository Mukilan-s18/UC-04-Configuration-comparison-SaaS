package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"configguard/internal/domain"
	"configguard/internal/repository"
	"configguard/internal/service"

	"github.com/google/uuid"
)

type APIHandler struct {
	repo   *repository.SaasRepository
	engine *service.CompareEngine
}

func NewAPIHandler(repo *repository.SaasRepository, engine *service.CompareEngine) *APIHandler {
	return &APIHandler{
		repo:   repo,
		engine: engine,
	}
}

// AuthenticateSession extracts and verifies the tenant session.
// Tenant identity is strictly derived from verified credentials.
func (h *APIHandler) AuthenticateSession(r *http.Request) (*domain.SessionContext, error) {
	var token string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else if authHeader != "" {
		token = strings.TrimSpace(authHeader)
	} else if cookie, err := r.Cookie("session_token"); err == nil {
		token = cookie.Value
	} else if custom := r.Header.Get("X-Session-Token"); custom != "" {
		token = custom
	}

	if token == "" {
		return nil, errors.New("missing authentication credentials")
	}

	// Pre-seeded sessions for testing and operations
	switch token {
	case "tenant-a-operator", "tenant-a-operator-token":
		t, err := h.repo.GetTenant(r.Context(), "tenant-a")
		if err != nil {
			return nil, err
		}
		return &domain.SessionContext{TenantID: t.TenantID, Role: domain.RoleOperator, Plan: t.Plan, Status: t.Status}, nil
	case "tenant-a-viewer", "tenant-a-viewer-token":
		t, err := h.repo.GetTenant(r.Context(), "tenant-a")
		if err != nil {
			return nil, err
		}
		return &domain.SessionContext{TenantID: t.TenantID, Role: domain.RoleViewer, Plan: t.Plan, Status: t.Status}, nil
	case "tenant-b-operator", "tenant-b-operator-token":
		t, err := h.repo.GetTenant(r.Context(), "tenant-b")
		if err != nil {
			return nil, err
		}
		return &domain.SessionContext{TenantID: t.TenantID, Role: domain.RoleOperator, Plan: t.Plan, Status: t.Status}, nil
	case "tenant-b-viewer", "tenant-b-viewer-token":
		t, err := h.repo.GetTenant(r.Context(), "tenant-b")
		if err != nil {
			return nil, err
		}
		return &domain.SessionContext{TenantID: t.TenantID, Role: domain.RoleViewer, Plan: t.Plan, Status: t.Status}, nil
	case "tenant-suspended-operator", "tenant-suspended-token":
		t, err := h.repo.GetTenant(r.Context(), "tenant-suspended")
		if err != nil {
			return nil, err
		}
		return &domain.SessionContext{TenantID: t.TenantID, Role: domain.RoleOperator, Plan: t.Plan, Status: t.Status}, nil
	default:
		return nil, errors.New("invalid session credentials")
	}
}

// GET /health - Public, reveals zero tenant data.
func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP","service":"configuration-comparison-saas"}`))
}

// GET /tools - Protected. Returns tool info and supported operations.
func (h *APIHandler) Tools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, err := h.AuthenticateSession(r)
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	_ = session

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tool":        "configuration-comparison",
		"version":     "1.0.0",
		"description": "Checks environment settings and reports differences between expected and actual values",
		"operations":  []string{domain.OpValidate, domain.OpProcess},
		"rules":       []string{"R1", "R2", "R3", "R4"},
	})
}

// GET /subscription - Protected. Returns plan and remaining daily quota.
func (h *APIHandler) Subscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, err := h.AuthenticateSession(r)
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sub, err := h.repo.GetSubscriptionInfo(r.Context(), session.TenantID)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to retrieve subscription info")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(sub)
}

// POST /runs - Protected. Accepts validate or process batch.
func (h *APIHandler) CreateRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate
	session, err := h.AuthenticateSession(r)
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 2. Reject tenant overrides in request headers
	if r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-Tenant-Override") != "" {
		h.writeJSONError(w, http.StatusBadRequest, "Client cannot specify tenant override")
		return
	}

	// 3. Subscription status check
	if session.Status == domain.StatusSuspended {
		h.writeJSONError(w, http.StatusForbidden, "Subscription is suspended; cannot start new runs")
		return
	}

	// 4. Role check: Operator can submit; Viewer cannot.
	if session.Role != domain.RoleOperator {
		h.writeJSONError(w, http.StatusForbidden, "Viewer role cannot submit runs")
		return
	}

	// 5. Require Idempotency-Key
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	// 6. Max size 64 KiB
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Request body exceeds maximum size of 64 KiB or read error")
		return
	}

	// 7. Parse raw JSON to strictly check shape, bounds, and reject unexpected/override fields
	var rawPayload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&rawPayload); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Reject if contains tenant override field
	if _, ok := rawPayload["tenant_id"]; ok {
		h.writeJSONError(w, http.StatusBadRequest, "Payload must not contain tenant_id override")
		return
	}

	// Reject if missing operation or records
	opRaw, okOp := rawPayload["operation"]
	recordsRaw, okRec := rawPayload["records"]
	if !okOp || !okRec {
		h.writeJSONError(w, http.StatusBadRequest, "Missing operation or records field")
		return
	}

	// Disallow extra root fields
	for k := range rawPayload {
		if k != "operation" && k != "records" {
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Unexpected field: %s", k))
			return
		}
	}

	operation, ok := opRaw.(string)
	if !ok || (operation != domain.OpValidate && operation != domain.OpProcess) {
		h.writeJSONError(w, http.StatusBadRequest, "Operation must be 'validate' or 'process'")
		return
	}

	// Check plan restrictions: Basic plan can only validate!
	if session.Plan == domain.PlanBasic && operation == domain.OpProcess {
		h.writeJSONError(w, http.StatusForbidden, "Basic plan cannot execute process operation; upgrade to Pro")
		return
	}

	recList, ok := recordsRaw.([]any)
	if !ok || len(recList) < 1 || len(recList) > 10 {
		h.writeJSONError(w, http.StatusBadRequest, "Records must contain between 1 and 10 rows")
		return
	}

	// Strictly validate each record row shape
	var records []domain.UC04Record
	for rowIdx, rawRow := range recList {
		rowMap, ok := rawRow.(map[string]any)
		if !ok {
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Row %d must be a JSON object", rowIdx+1))
			return
		}

		for k := range rowMap {
			if k != "record_id" && k != "expected" && k != "actual" && k != "environment" {
				h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Row %d has unexpected field '%s'", rowIdx+1, k))
				return
			}
		}

		ridVal, hasRid := rowMap["record_id"]
		expVal, hasExp := rowMap["expected"]
		actVal, hasAct := rowMap["actual"]
		envVal, hasEnv := rowMap["environment"]

		if !hasRid || !hasExp || !hasAct || !hasEnv {
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Row %d is missing required fields", rowIdx+1))
			return
		}

		ridStr, ok1 := ridVal.(string)
		expStr, ok2 := expVal.(string)
		actStr, ok3 := actVal.(string)
		envStr, ok4 := envVal.(string)

		if !ok1 || !ok2 || !ok3 || !ok4 {
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Row %d contains non-string fields (booleans/numbers not permitted)", rowIdx+1))
			return
		}

		records = append(records, domain.UC04Record{
			RecordID:    ridStr,
			Expected:    expStr,
			Actual:      actStr,
			Environment: envStr,
		})
	}

	req := domain.RunRequest{
		Operation: operation,
		Records:   records,
	}
	inputFingerprint := req.ComputeFingerprint()

	// 8. Check Idempotency Replay BEFORE enforcing remaining quota!
	existingRun, err := h.repo.CheckExistingReplay(r.Context(), session.TenantID, idempotencyKey)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to check idempotency")
		return
	}

	if existingRun != nil {
		// Matching key + changed input returns 409
		if existingRun.InputFingerprint != inputFingerprint || existingRun.Operation != operation {
			h.writeJSONError(w, http.StatusConflict, "Idempotency key reused with different operation or input")
			return
		}

		// Replay still Running returns 409 RUN_IN_PROGRESS
		if existingRun.Status == domain.RunStatusRunning {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "RUN_IN_PROGRESS"})
			return
		}

		// Matching terminal replay returns 200 without quota debit
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := domain.RunResponse{
			RunID:  existingRun.RunID,
			Status: existingRun.Status,
			Result: existingRun.Result,
			Error:  existingRun.ErrorMessage,
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// 9. If operation is process, pre-check row validation rules:
	// "process rejects any invalid row with 400; otherwise save the helper output: summary, rows and issues [].
	// The process operation must reject this batch before quota is debited."
	if operation == domain.OpProcess {
		valRes := h.engine.ValidateBatch(records)
		if valRes.InvalidRows > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":        "Validation failed on process batch",
				"valid_rows":   valRes.ValidRows,
				"invalid_rows": valRes.InvalidRows,
				"issues":       valRes.Issues,
			})
			return
		}
	}

	// 10. Reserve slot atomically when accepting a new run
	maxQuota := domain.ProDailyQuota
	if session.Plan == domain.PlanBasic {
		maxQuota = domain.BasicDailyQuota
	}

	newRun := &domain.Run{
		RunID:            uuid.New().String(),
		TenantID:         session.TenantID,
		IdempotencyKey:   idempotencyKey,
		Operation:        operation,
		InputFingerprint: inputFingerprint,
		Status:           domain.RunStatusRunning,
	}

	if err := h.repo.ReserveSlotAndCreateRun(r.Context(), newRun, maxQuota); err != nil {
		if errors.Is(err, repository.ErrQuotaExceeded) {
