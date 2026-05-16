package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"event-service/config"
	"event-service/internal/db"
	"event-service/internal/logger"
	"event-service/internal/notification"
	"event-service/internal/repository"
	"event-service/internal/worker"
)

var (
	ErrConfigLoad = errors.New("failed to load config")
	ErrWorkerRun  = errors.New("failed to run booking expiry worker")
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

	producer := notification.NewProducer(cfg, log)
	defer func() {
		if err := producer.Close(); err != nil {
			log.Error("failed to close notification producer", "error", err)
		}
	}()

	repo := repository.NewEventRepository(pool)
	expiryWorker := worker.NewBookingExpiryWorker(repo, producer, cfg.WorkerInterval, log)
	if err := expiryWorker.Run(ctx); err != nil && !worker.IsStopped(err) {
		log.Error(ErrWorkerRun.Error(), "error", err)
		os.Exit(1)
	}
}
