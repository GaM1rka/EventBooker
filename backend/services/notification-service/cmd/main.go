package main

import (
	"errors"
	"os"

	"notification-service/config"
	"notification-service/internal/app"
	"notification-service/internal/logger"
)

var (
	ErrConfigLoad           = errors.New("failed to load config")
	ErrServerInitialization = errors.New("failed to initialize server")
	ErrServerRun            = errors.New("failed to run server")
)

func main() {
	logger := logger.New()

	cfg, err := config.Load()
	if err != nil {
		logger.Error(ErrConfigLoad.Error(), "error", err)
		os.Exit(1)
		return
	}

	app, err := app.New(cfg, logger)
	if err != nil {
		logger.Error(ErrServerInitialization.Error(), "error", err)
		os.Exit(1)
		return
	}

	if err = app.Run(); err != nil {
		logger.Error(ErrServerRun.Error(), "error", err)
		os.Exit(1)
		return
	}
}
