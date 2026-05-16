package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"event-service/config"
	"event-service/internal/domain"

	"github.com/segmentio/kafka-go"
)

type EmailNotification struct {
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type Producer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

func NewProducer(cfg *config.Config, logger *slog.Logger) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Kafka.Brokers...),
			Topic:    cfg.Kafka.Topic,
			Balancer: &kafka.LeastBytes{},
		},
		logger: logger,
	}
}

func (p *Producer) SendBookingCanceled(ctx context.Context, booking domain.ExpiredBooking) error {
	payload := EmailNotification{
		Email:   booking.UserEmail,
		Subject: "Бронь отменена",
		Message: fmt.Sprintf("Здравствуйте, %s!\n\nВаша бронь на мероприятие \"%s\" отменена, потому что не была подтверждена до дедлайна.", booking.UserName, booking.EventTitle),
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("booking:%d", booking.ID)),
		Value: value,
	}); err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}

	p.logger.Info("booking cancellation notification published", "bookingID", booking.ID, "email", booking.UserEmail)
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
