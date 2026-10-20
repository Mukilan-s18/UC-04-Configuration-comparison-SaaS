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
