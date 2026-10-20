package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"configguard/internal/api"
	"configguard/internal/domain"
	"configguard/internal/repository"
	"configguard/internal/service"

	_ "modernc.org/sqlite"
)

func setupTestServer(t *testing.T, dbPath string, resetHarness bool) (*httptest.Server, *repository.SaasRepository, func()) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("Failed to open sqlite db: %v", err)
	}

	repo := repository.NewSaasRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := repo.InitSchema(ctx); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}
	if resetHarness {
		_ = repo.ResetHarness(ctx)
	}

	engine := service.NewCompareEngine()
	apiHandler := api.NewAPIHandler(repo, engine)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", apiHandler.Health)
	mux.HandleFunc("GET /tools", apiHandler.Tools)
	mux.HandleFunc("GET /subscription", apiHandler.Subscription)
	mux.HandleFunc("POST /runs", apiHandler.CreateRun)
	mux.HandleFunc("GET /runs", apiHandler.ListRuns)
	mux.HandleFunc("GET /runs/{id}/result", apiHandler.GetRunResult)
	mux.HandleFunc("POST /harness/reset", apiHandler.HarnessReset)
	mux.HandleFunc("POST /harness/tenant-status", apiHandler.HarnessSetTenantStatus)

	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		db.Close()
	}

	return ts, repo, cleanup
}

func doRequest(ts *httptest.Server, method, path, token, idempotencyKey string, body any, forceFailure bool) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, ts.URL+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if forceFailure {
		req.Header.Set("X-Test-Force-Failure", "true")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return resp, respBody, err
}

// A01: /health is 200 and reveals no tenant data
func TestA01_HealthCheck(t *testing.T) {
	dbPath := "test_a01.db"
	defer os.Remove(dbPath)
	ts, _, cleanup := setupTestServer(t, dbPath, true)
	defer cleanup()

	resp, body, err := doRequest(ts, "GET", "/health", "", "", nil, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]string
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if data["status"] != "UP" {
		t.Errorf("expected UP status, got %s", data["status"])
	}
	// Verify no tenant data is exposed
	bodyStr := string(body)
	if bytes.Contains(body, []byte("tenant")) || bytes.Contains(body, []byte("quota")) {
		t.Errorf("health endpoint reveals tenant data: %s", bodyStr)
	}
}

// A02: validate on the two-row valid fixture reports valid_rows 2, invalid_rows 0 and no issues.
func TestA02_ValidateValidFixture(t *testing.T) {
	dbPath := "test_a02.db"
	defer os.Remove(dbPath)
	ts, _, cleanup := setupTestServer(t, dbPath, true)
	defer cleanup()

	records := service.GetValidFixture()
	payload := map[string]any{
		"operation": "validate",
		"records":   records,
	}

	resp, body, err := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a02", payload, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(body))
	}

	var runResp domain.RunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	var valRes domain.ValidationResult
	if err := json.Unmarshal(runResp.Result, &valRes); err != nil {
		t.Fatalf("invalid result json: %v", err)
	}

	if valRes.ValidRows != 2 || valRes.InvalidRows != 0 || len(valRes.Issues) != 0 {
		t.Errorf("A02 failed: expected 2 valid, 0 invalid, 0 issues; got valid=%d, invalid=%d, issues=%d",
			valRes.ValidRows, valRes.InvalidRows, len(valRes.Issues))
	}
}

// A03: validate on the six-row invalid fixture reports valid_rows 2, invalid_rows 4 and the four specified rule failures.
func TestA03_ValidateInvalidFixture(t *testing.T) {
	dbPath := "test_a03.db"
	defer os.Remove(dbPath)
	ts, _, cleanup := setupTestServer(t, dbPath, true)
	defer cleanup()

	records := service.GetInvalidFixture()
	payload := map[string]any{
		"operation": "validate",
		"records":   records,
	}

	resp, body, err := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a03", payload, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(body))
	}

	var runResp domain.RunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	var valRes domain.ValidationResult
	if err := json.Unmarshal(runResp.Result, &valRes); err != nil {
		t.Fatalf("invalid result json: %v", err)
	}

	if valRes.ValidRows != 2 || valRes.InvalidRows != 4 || len(valRes.Issues) != 4 {
		t.Errorf("A03 failed: expected 2 valid, 4 invalid, 4 issues; got valid=%d, invalid=%d, issues=%d",
			valRes.ValidRows, valRes.InvalidRows, len(valRes.Issues))
	}

	rules := make(map[string]bool)
	for _, iss := range valRes.Issues {
		rules[iss.RuleID] = true
	}
	for _, expectedRule := range []string{"R1", "R2", "R3", "R4"} {
		if !rules[expectedRule] {
			t.Errorf("A03 failed: missing issue for %s", expectedRule)
		}
	}
}

// A04: Pro process returns exactly the specified rows and summary; its result survives an app restart and is retrieved through the API.
func TestA04_ProProcessAndPersistenceAcrossRestart(t *testing.T) {
	dbPath := "test_a04.db"
	defer os.Remove(dbPath)

	var runID string
	// Step 1: Run process on server instance 1
	{
		ts, _, cleanup := setupTestServer(t, dbPath, true)
		records := service.GetValidFixture()
		payload := map[string]any{
			"operation": "process",
			"records":   records,
		}

		resp, body, err := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a04", payload, false)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(body))
		}

		var runResp domain.RunResponse
		if err := json.Unmarshal(body, &runResp); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		runID = runResp.RunID

		var procRes domain.ProcessResult
		if err := json.Unmarshal(runResp.Result, &procRes); err != nil {
			t.Fatalf("invalid result json: %v", err)
		}

		if procRes.Summary.InputRows != 2 || procRes.Summary.ChangedRows != 1 || procRes.Summary.UnchangedRows != 1 {
			t.Errorf("summary mismatch: %+v", procRes.Summary)
		}
		if len(procRes.Rows) != 2 || procRes.Rows[0].Comparison != "CHANGED" || procRes.Rows[1].Comparison != "UNCHANGED" {
			t.Errorf("rows mismatch: %+v", procRes.Rows)
		}

		cleanup() // Simulates app shutdown
	}

	// Step 2: Restart app on server instance 2 and retrieve via GET /runs/{id}/result
	{
		ts2, _, cleanup2 := setupTestServer(t, dbPath, false)
		defer cleanup2()

		resp, body, err := doRequest(ts2, "GET", "/runs/"+runID+"/result", "tenant-a-operator", "", nil, false)
		if err != nil {
			t.Fatalf("restart retrieval failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK after restart, got %d: %s", resp.StatusCode, string(body))
		}

		var runResp domain.RunResponse
		if err := json.Unmarshal(body, &runResp); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if runResp.RunID != runID || runResp.Status != "Succeeded" {
			t.Errorf("expected Succeeded status for %s, got %s", runID, runResp.Status)
		}

		var procRes domain.ProcessResult
		if err := json.Unmarshal(runResp.Result, &procRes); err != nil {
			t.Fatalf("invalid result json after restart: %v", err)
		}
		if procRes.Summary.InputRows != 2 || procRes.Summary.ChangedRows != 1 || procRes.Summary.UnchangedRows != 1 {
			t.Errorf("A04 persistence failed: summary mismatch after restart: %+v", procRes.Summary)
		}
	}
}

// A05: Invalid process input, a wrong-shaped body and an 11-row batch each return 400 with no new run or quota debit.
func TestA05_InputBoundsAndValidationRejectionNoDebit(t *testing.T) {
	dbPath := "test_a05.db"
	defer os.Remove(dbPath)
	ts, repo, cleanup := setupTestServer(t, dbPath, true)
	defer cleanup()

	// 1. Invalid process input (e.g. invalid fixture on process)
	invalidFixture := service.GetInvalidFixture()
	resp1, _, _ := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a05-1", map[string]any{
		"operation": "process",
		"records":   invalidFixture,
	}, false)
	if resp1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid process input, got %d", resp1.StatusCode)
	}

	// 2. Wrong-shaped body (unexpected field or missing required fields)
	resp2, _, _ := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a05-2", map[string]any{
		"operation":     "validate",
		"unknown_field": "test",
		"records":       service.GetValidFixture(),
	}, false)
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for wrong-shaped body, got %d", resp2.StatusCode)
	}

	// 3. 11-row batch (bounds exceeded)
	var elevenRows []domain.UC04Record
	for i := 0; i < 11; i++ {
		elevenRows = append(elevenRows, domain.UC04Record{
			RecordID:    fmt.Sprintf("rec_%d", i),
			Expected:    "v",
			Actual:      "v",
			Environment: "TEST",
		})
	}
	resp3, _, _ := doRequest(ts, "POST", "/runs", "tenant-a-operator", "key-a05-3", map[string]any{
		"operation": "validate",
		"records":   elevenRows,
	}, false)
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for 11-row batch, got %d", resp3.StatusCode)
	}

	// Verify no runs were created and quota is still 0
	sub, _ := repo.GetSubscriptionInfo(context.Background(), "tenant-a")
	if sub.QuotaUsed != 0 {
		t.Errorf("expected 0 quota used, got %d", sub.QuotaUsed)
	}
