package main

import (
	"errors"
	"os"

	"event-service/config"
	"event-service/internal/logger"
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

	log.Info("event api service scaffold ready", "addr", cfg.ServerAddr)
}
