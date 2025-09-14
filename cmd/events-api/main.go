package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"example.com/goAssignment1/internal/config"
	"example.com/goAssignment1/internal/ingest"
	spg "example.com/goAssignment1/internal/storage/postgres"
	transport "example.com/goAssignment1/internal/transport/http"
)

func main() {
	cfg := config.Parse()
	log.Printf("config: DSN=%s port=%s", cfg.PostgresDSN, cfg.Port)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := spg.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()
	log.Printf("db: connected")

	mig := filepath.Join("migrations", "0001_init.sql")
	if err := db.RunMigration(ctx, mig); err != nil {
		log.Fatalf("migration: %v", err)
	}
	log.Printf("db: migration applied")

	writer := spg.NewWriter(db)
	ingestor := ingest.NewIngestor(writer, cfg.QueueMaxSize, cfg.BatchMaxSize, cfg.BatchMaxWait)
	ingestor.Start(ctx)
	log.Printf("ingest: started (queue=%d batch=%d wait=%s)", cfg.QueueMaxSize, cfg.BatchMaxSize, cfg.BatchMaxWait)

	deps := &transport.ServerDeps{
		Cfg:      cfg,
		Ingestor: ingestor,
		DB:       db,
		Now:      func() time.Time { return time.Now().UTC() },
	}
	h := deps.Router()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	_ = srv.Shutdown(shutdownCtx)
}
