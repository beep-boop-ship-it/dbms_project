package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"college-management/backend/internal/config"
	"college-management/backend/internal/store"
	"college-management/backend/internal/transport"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	var repo store.Repository
	if cfg.DataMode == "demo" {
		repo = store.NewMemory()
		log.Print("starting with in-memory demo data (reset on restart)")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		repo, err = store.NewPostgres(ctx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			log.Fatalf("connect PostgreSQL: %v", err)
		}
	}
	defer repo.Close()
	srv := &http.Server{Addr: cfg.ServerAddr, Handler: transport.New(repo, cfg.JWTSecret, cfg.FrontendURL), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("college management API listening on %s (mode=%s)", cfg.ServerAddr, cfg.DataMode)
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			log.Fatalf("http server: %v", e)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}
