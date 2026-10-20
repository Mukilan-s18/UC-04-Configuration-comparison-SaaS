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

