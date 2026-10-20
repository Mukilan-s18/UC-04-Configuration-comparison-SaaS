package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func BenchmarkCompareEngine_Validate(b *testing.B) {
	engine := service.NewCompareEngine()
	records := []domain.UC04Record{
		{RecordID: "max_retries", Expected: "3", Actual: "5", Environment: "TEST"},
		{RecordID: "region", Expected: "us", Actual: "us", Environment: "STAGE"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := engine.ValidateBatch(records)
		if res.ValidRows != 2 || res.InvalidRows != 0 || len(res.Issues) != 0 {
			b.Fatalf("Unexpected validate result")
		}
	}
}

func BenchmarkCompareEngine_Process(b *testing.B) {
	engine := service.NewCompareEngine()
	records := []domain.UC04Record{
		{RecordID: "max_retries", Expected: "3", Actual: "5", Environment: "TEST"},
		{RecordID: "region", Expected: "us", Actual: "us", Environment: "STAGE"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := engine.ProcessBatch(records)
		if res.Summary.ChangedRows != 1 || res.Summary.UnchangedRows != 1 || len(res.Rows) != 2 {
			b.Fatalf("Unexpected process result")
		}
	}
}

func BenchmarkAPI_HealthCheck(b *testing.B) {
	dbPath := fmt.Sprintf("test_bench_health_%d.db", time.Now().UnixNano())
	defer os.Remove(dbPath)

	db, _ := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	defer db.Close()
	repo := repository.NewSaasRepository(db)
	_ = repo.InitSchema(context.Background())
	engine := service.NewCompareEngine()
	handler := api.NewAPIHandler(repo, engine)

	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.Health(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("Health returned %d", rec.Code)
		}
	}
}

func TestPerformance_HighLoadConcurrency(t *testing.T) {
	dbPath := fmt.Sprintf("test_load_%d.db", time.Now().UnixNano())
	defer func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	}()

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("Failed to open sqlite: %v", err)
	}
	defer db.Close()

	repo := repository.NewSaasRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema error: %v", err)
	}

	engine := service.NewCompareEngine()
	apiHandler := api.NewAPIHandler(repo, engine)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /runs", apiHandler.CreateRun)
	mux.HandleFunc("GET /health", apiHandler.Health)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	const totalRequests = 200
	const concurrency = 20

	var (
		wg          sync.WaitGroup
		statusCodes = make(map[int]int)
		statusMapMu sync.Mutex
		latencies   = make([]time.Duration, totalRequests)
	)

	start := time.Now()
	sem := make(chan struct{}, concurrency)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()

			// Use tenant-a-operator (Pro plan, quota 10)
			// Replay same key periodically to exercise cache hits and concurrency
			key := fmt.Sprintf("bench-key-%d", idx%15)
			payload := map[string]any{
				"operation": "process",
				"records": []map[string]string{
					{"record_id": "max_retries", "expected": "3", "actual": "5", "environment": "TEST"},
					{"record_id": "region", "expected": "us", "actual": "us", "environment": "STAGE"},
				},
			}
			bodyBytes, _ := json.Marshal(payload)

			req, _ := http.NewRequest("POST", server.URL+"/runs", bytes.NewReader(bodyBytes))
			req.Header.Set("Authorization", "Bearer tenant-a-operator")
			req.Header.Set("Idempotency-Key", key)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			latencies[idx] = time.Since(reqStart)

			if err != nil {
				return
			}
			defer resp.Body.Close()

			statusMapMu.Lock()
			statusCodes[resp.StatusCode]++
			statusMapMu.Unlock()
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	var totalLat time.Duration
	var minLat = time.Hour
	var maxLat time.Duration
	for _, l := range latencies {
		totalLat += l
		if l < minLat && l > 0 {
			minLat = l
		}
		if l > maxLat {
			maxLat = l
		}
	}
	avgLat := totalLat / totalRequests
	throughput := float64(totalRequests) / totalDuration.Seconds()

	t.Logf("=== Load & Concurrency Performance Results ===")
	t.Logf("Total Requests: %d across %d concurrent workers", totalRequests, concurrency)
	t.Logf("Total Elapsed Time: %v", totalDuration)
	t.Logf("Throughput: %.2f requests/second", throughput)
	t.Logf("Average Latency: %v", avgLat)
	t.Logf("Min Latency: %v | Max Latency: %v", minLat, maxLat)
	t.Logf("Status Code Counts: %v", statusCodes)

	totalProcessed := 0
	for _, count := range statusCodes {
		totalProcessed += count
	}
	if totalProcessed != totalRequests {
		t.Errorf("Mismatch in expected responses: got %d, expected %d", totalProcessed, totalRequests)
	}
}
