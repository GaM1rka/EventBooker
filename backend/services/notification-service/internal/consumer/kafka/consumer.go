package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"notification-service/config"
	"notification-service/internal/domain/models"
)

var (
	ErrFailedToUnmarshalMessage = "failed to unmarshal message"
	ErrFailedToSendNotification = "failed to send notification"
)

type NotificationService interface {
	SendEmailNotification(n models.EmailNotification) error
}

type Consumer struct {
	logger  *slog.Logger
	reader  *kafka.Reader
	service NotificationService
}

func NewConsumer(cfg *config.Config, service NotificationService, logger *slog.Logger) *Consumer {
	return &Consumer{
		logger: logger,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Kafka.Brokers,
			Topic:   cfg.Kafka.Topic,
			GroupID: cfg.Kafka.GroupID,
		}),
		service: service,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("Kafka consumer started")
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				c.logger.Info("Kafka consumer stopped by context")
				return err
			}
			return err
		}

		c.logger.Info("Kafka message received", "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)

		var notification models.EmailNotification
		if err := json.Unmarshal(msg.Value, &notification); err != nil {
			c.logger.Error(ErrFailedToUnmarshalMessage, "error", err, "offset", msg.Offset)
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				c.logger.Error("failed to commit invalid Kafka message", "error", commitErr, "offset", msg.Offset)
			}
			continue
		}

		if err := c.service.SendEmailNotification(notification); err != nil {
			c.logger.Error(ErrFailedToSendNotification, "error", err, "email", notification.Email, "offset", msg.Offset)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("failed to commit Kafka message", "error", err, "offset", msg.Offset)
			continue
		}
		c.logger.Info("Kafka message processed", "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)
	}
}

func (c *Consumer) Close() error {
	c.logger.Info("closing Kafka consumer")
	return c.reader.Close()
}
