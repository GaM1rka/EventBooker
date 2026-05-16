package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"notification-service/config"
)

type App struct {
	container *Container
	logger    *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	return &App{
		container: NewContainer(cfg, logger),
		logger:    logger,
	}, nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a.logger.Info("notification service started")
	defer a.logger.Info("notification service stopped")

	err := a.container.Consumer.Run(ctx)
	closeErr := a.container.Consumer.Close()
	if closeErr != nil {
		a.logger.Error("failed to close Kafka consumer", "error", closeErr)
	}

	if errors.Is(err, context.Canceled) {
		return closeErr
	}
	return err
}
