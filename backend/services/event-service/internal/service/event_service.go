package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"event-service/config"
	"event-service/internal/domain"
	"event-service/internal/repository"
)

type EventRepository interface {
	CreateEvent(ctx context.Context, params repository.CreateEventParams) (domain.Event, error)
	ListEventDetails(ctx context.Context) ([]domain.EventDetails, error)
	GetEventDetails(ctx context.Context, eventID int64) (domain.EventDetails, error)
	BookEvent(ctx context.Context, params repository.BookEventParams) (domain.Booking, error)
	ConfirmBooking(ctx context.Context, eventID int64, userEmail string) (domain.Booking, error)
}

type EventService struct {
	repo              EventRepository
	defaultBookingTTL time.Duration
}

func NewEventService(repo EventRepository, cfg *config.Config) *EventService {
	return &EventService{
		repo:              repo,
		defaultBookingTTL: cfg.DefaultBookingTTL,
	}
}

type CreateEventRequest struct {
	Title             string `json:"title"`
	EventDate         string `json:"eventDate"`
	Capacity          int    `json:"capacity"`
	BookingTTLMinutes int    `json:"bookingTtlMinutes"`
	RequiresPayment   *bool  `json:"requiresPayment"`
}

type BookEventRequest struct {
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
}

type ConfirmBookingRequest struct {
	UserEmail string `json:"userEmail"`
}

func (s *EventService) CreateEvent(ctx context.Context, req CreateEventRequest) (domain.Event, error) {
	title := strings.TrimSpace(req.Title)
	eventDate, err := parseEventDate(req.EventDate)
	if err != nil {
		return domain.Event{}, domain.ErrInvalidInput
	}

	bookingTTL := s.defaultBookingTTL
	if req.BookingTTLMinutes > 0 {
		bookingTTL = time.Duration(req.BookingTTLMinutes) * time.Minute
	}

	requiresPayment := true
	if req.RequiresPayment != nil {
		requiresPayment = *req.RequiresPayment
	}

	return s.repo.CreateEvent(ctx, repository.CreateEventParams{
		Title:           title,
		EventDate:       eventDate,
		Capacity:        req.Capacity,
		BookingTTL:      bookingTTL,
		RequiresPayment: requiresPayment,
	})
}

func (s *EventService) ListEventDetails(ctx context.Context) ([]domain.EventDetails, error) {
	return s.repo.ListEventDetails(ctx)
}

func (s *EventService) GetEventDetails(ctx context.Context, eventID int64) (domain.EventDetails, error) {
	return s.repo.GetEventDetails(ctx, eventID)
}

func (s *EventService) BookEvent(ctx context.Context, eventID int64, req BookEventRequest) (domain.Booking, error) {
	if _, err := mail.ParseAddress(strings.TrimSpace(req.UserEmail)); err != nil {
		return domain.Booking{}, domain.ErrInvalidInput
	}

	return s.repo.BookEvent(ctx, repository.BookEventParams{
		EventID:   eventID,
		UserName:  req.UserName,
		UserEmail: req.UserEmail,
	})
}

func (s *EventService) ConfirmBooking(ctx context.Context, eventID int64, req ConfirmBookingRequest) (domain.Booking, error) {
	if _, err := mail.ParseAddress(strings.TrimSpace(req.UserEmail)); err != nil {
		return domain.Booking{}, domain.ErrInvalidInput
	}

	return s.repo.ConfirmBooking(ctx, eventID, req.UserEmail)
}

func parseEventDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("event date is empty")
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, errors.New("invalid event date")
}
