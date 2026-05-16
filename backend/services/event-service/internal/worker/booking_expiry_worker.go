package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"event-service/internal/domain"
)

type BookingRepository interface {
	CancelExpiredBookings(ctx context.Context, limit int) ([]domain.ExpiredBooking, error)
}

type NotificationSender interface {
	SendBookingCanceled(ctx context.Context, booking domain.ExpiredBooking) error
}

type BookingExpiryWorker struct {
	repo     BookingRepository
	notifier NotificationSender
	interval time.Duration
	logger   *slog.Logger
}

func NewBookingExpiryWorker(repo BookingRepository, notifier NotificationSender, interval time.Duration, logger *slog.Logger) *BookingExpiryWorker {
	return &BookingExpiryWorker{
		repo:     repo,
		notifier: notifier,
		interval: interval,
		logger:   logger,
	}
}

func (w *BookingExpiryWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("booking expiry worker started", "interval", w.interval.String())
	if err := w.tick(ctx); err != nil {
		w.logger.Error("booking expiry tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("booking expiry worker stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.Error("booking expiry tick failed", "error", err)
			}
		}
	}
}

func (w *BookingExpiryWorker) tick(ctx context.Context) error {
	expired, err := w.repo.CancelExpiredBookings(ctx, 100)
	if err != nil {
		return err
	}
	if len(expired) == 0 {
		return nil
	}

	w.logger.Info("expired bookings canceled", "count", len(expired))
	for _, booking := range expired {
		if err := w.notifier.SendBookingCanceled(ctx, booking); err != nil {
			w.logger.Error("failed to publish booking cancellation", "bookingID", booking.ID, "error", err)
			continue
		}
	}

	return nil
}

func IsStopped(err error) bool {
	return errors.Is(err, context.Canceled)
}
