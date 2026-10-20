package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"configguard/internal/api"
	"configguard/internal/repository"
	"configguard/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("SQLITE_PATH", "saas.db")
	pgURL := os.Getenv("DATABASE_URL")

	var db *sql.DB
	var err error

	if pgURL != "" {
		log.Printf("[SaaS] Connecting to PostgreSQL at %s...\n", pgURL)
		db, err = sql.Open("pgx", pgURL)
		if err != nil {
			log.Fatalf("Failed to open postgres: %v", err)
		}
	} else {
		log.Printf("[SaaS] Initializing SQLite database at %s...\n", dbPath)
		db, err = sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			log.Fatalf("Failed to open sqlite: %v", err)
		}
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	// Initialize schema & seeds
	repo := repository.NewSaasRepository(db)
	if err := repo.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}
	log.Println("[SaaS] Database initialized and seeded successfully")

	compareEngine := service.NewCompareEngine()
	apiHandler := api.NewAPIHandler(repo, compareEngine)

	mux := http.NewServeMux()

	// 1. Mandatory 6 Routes for Advantix Shared SaaS Contract (Page 2 & Page 5: exactly six routes)
	mux.HandleFunc("GET /health", apiHandler.Health)
	mux.HandleFunc("GET /tools", apiHandler.Tools)
	mux.HandleFunc("GET /subscription", apiHandler.Subscription)
	mux.HandleFunc("POST /runs", apiHandler.CreateRun)
	mux.HandleFunc("GET /runs", apiHandler.ListRuns)
	mux.HandleFunc("GET /runs/{id}/result", apiHandler.GetRunResult)

	// 2. UI Screens (Run Tools & Execution History)
	indexPath := getEnv("TEMPLATE_PATH", "web/index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		indexPath = filepath.Join(".", "web", "templates", "app.html")
	}

	renderUI := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, indexPath)
	}

	mux.HandleFunc("GET /", renderUI)
	mux.HandleFunc("GET /history", renderUI)
	mux.HandleFunc("GET /tools-ui", renderUI)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web"))))

	// Defensive Security Headers Middleware
	securityHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		mux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           securityHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[SaaS] Server listening on http://localhost:%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen error: %v", err)
		}
	}()

	<-stop
	log.Println("[SaaS] Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v\n", err)
	}
	log.Println("[SaaS] Server stopped")
}
