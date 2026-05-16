package main

import (
	"errors"
	"os"

	"event-service/config"
	"event-service/internal/logger"
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

	log.Info("booking expiry worker scaffold ready", "interval", cfg.WorkerInterval.String())
}
