package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-service/config"
	"event-service/internal/db"
	transporthttp "event-service/internal/http"
	"event-service/internal/logger"
	"event-service/internal/repository"
	"event-service/internal/service"
)

var (
	ErrConfigLoad = errors.New("failed to load config")
	ErrServerRun  = errors.New("failed to run api server")
)

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error(ErrConfigLoad.Error(), "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.NewEventRepository(pool)
	eventService := service.NewEventService(repo, cfg)
	handler := transporthttp.NewHandler(eventService, log)

	server := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown api server", "error", err)
		}
	}()

	log.Info("event api service started", "addr", cfg.ServerAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error(ErrServerRun.Error(), "error", err)
		os.Exit(1)
	}
	log.Info("event api service stopped")
}
