package app

import (
	"log/slog"

	"notification-service/config"
	smtpclient "notification-service/internal/client/smtp"
	kafkaconsumer "notification-service/internal/consumer/kafka"
	"notification-service/internal/service"
)

type Container struct {
	Consumer *kafkaconsumer.Consumer
}

func NewContainer(cfg *config.Config, logger *slog.Logger) *Container {
	emailClient := smtpclient.NewClient(cfg, logger)
	notificationService := service.NewNotificationService(emailClient, logger)
	consumer := kafkaconsumer.NewConsumer(cfg, notificationService, logger)

	return &Container{
		Consumer: consumer,
	}
}
