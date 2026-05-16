package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"event-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

type CreateEventParams struct {
	Title           string
	EventDate       time.Time
	Capacity        int
	BookingTTL      time.Duration
	RequiresPayment bool
}

type BookEventParams struct {
	EventID   int64
	UserName  string
	UserEmail string
}

func (r *EventRepository) CreateEvent(ctx context.Context, params CreateEventParams) (domain.Event, error) {
	params.Title = strings.TrimSpace(params.Title)
	if params.Title == "" || params.EventDate.IsZero() || params.Capacity <= 0 || params.BookingTTL <= 0 {
		return domain.Event{}, domain.ErrInvalidInput
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO events (title, event_date, capacity, booking_ttl_seconds, requires_payment)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, event_date, capacity, booking_ttl_seconds, requires_payment, created_at
	`, params.Title, params.EventDate, params.Capacity, int(params.BookingTTL.Seconds()), params.RequiresPayment)

	return scanEvent(row)
}

func (r *EventRepository) ListEventDetails(ctx context.Context) ([]domain.EventDetails, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			e.id, e.title, e.event_date, e.capacity, e.booking_ttl_seconds, e.requires_payment, e.created_at,
			COUNT(b.id) FILTER (WHERE b.status = 'pending') AS pending_count,
			COUNT(b.id) FILTER (WHERE b.status = 'confirmed') AS confirmed_count
		FROM events e
		LEFT JOIN bookings b ON b.event_id = e.id AND b.status <> 'canceled'
		GROUP BY e.id
		ORDER BY e.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.EventDetails, 0)
	for rows.Next() {
		var item domain.EventDetails
		var ttlSeconds int
		if err := rows.Scan(
			&item.Event.ID,
			&item.Event.Title,
			&item.Event.EventDate,
			&item.Event.Capacity,
			&ttlSeconds,
			&item.Event.RequiresPayment,
			&item.Event.CreatedAt,
			&item.PendingCount,
			&item.ConfirmedCount,
		); err != nil {
			return nil, fmt.Errorf("scan event details: %w", err)
		}
		item.Event.BookingTTL = time.Duration(ttlSeconds) * time.Second
		item.FreeSeats = max(item.Event.Capacity-item.PendingCount-item.ConfirmedCount, 0)
		item.Bookings = []domain.Booking{}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

func (r *EventRepository) GetEventDetails(ctx context.Context, eventID int64) (domain.EventDetails, error) {
	event, err := r.getEvent(ctx, eventID)
	if err != nil {
		return domain.EventDetails{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, user_name, user_email, status, expires_at, confirmed_at, created_at
		FROM bookings
		WHERE event_id = $1 AND status <> 'canceled'
		ORDER BY created_at DESC
	`, eventID)
	if err != nil {
		return domain.EventDetails{}, fmt.Errorf("query bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]domain.Booking, 0)
	pendingCount := 0
	confirmedCount := 0
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return domain.EventDetails{}, err
		}
		bookings = append(bookings, booking)
		switch booking.Status {
		case domain.BookingStatusPending:
			pendingCount++
		case domain.BookingStatusConfirmed:
			confirmedCount++
		}
	}
	if err := rows.Err(); err != nil {
		return domain.EventDetails{}, fmt.Errorf("iterate bookings: %w", err)
	}

	occupied := pendingCount + confirmedCount
	return domain.EventDetails{
		Event:          event,
		FreeSeats:      max(event.Capacity-occupied, 0),
		PendingCount:   pendingCount,
		ConfirmedCount: confirmedCount,
		Bookings:       bookings,
	}, nil
}

func (r *EventRepository) BookEvent(ctx context.Context, params BookEventParams) (domain.Booking, error) {
	params.UserName = strings.TrimSpace(params.UserName)
	params.UserEmail = strings.TrimSpace(params.UserEmail)
	if params.EventID <= 0 || params.UserName == "" || params.UserEmail == "" {
		return domain.Booking{}, domain.ErrInvalidInput
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer rollback(ctx, tx)

	event, err := r.getEventForUpdate(ctx, tx, params.EventID)
	if err != nil {
		return domain.Booking{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'canceled', canceled_at = NOW()
		WHERE event_id = $1 AND status = 'pending' AND expires_at <= NOW()
	`, params.EventID); err != nil {
		return domain.Booking{}, fmt.Errorf("cancel expired bookings before booking: %w", err)
	}

	var occupied int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE event_id = $1 AND status IN ('pending', 'confirmed')
	`, params.EventID).Scan(&occupied); err != nil {
		return domain.Booking{}, fmt.Errorf("count occupied seats: %w", err)
	}
	if occupied >= event.Capacity {
		return domain.Booking{}, domain.ErrNoSeatsAvailable
	}

	expiresAt := time.Now().UTC().Add(event.BookingTTL)
	row := tx.QueryRow(ctx, `
		INSERT INTO bookings (event_id, user_name, user_email, status, expires_at)
		VALUES ($1, $2, $3, 'pending', $4)
		RETURNING id, event_id, user_name, user_email, status, expires_at, confirmed_at, created_at
	`, params.EventID, params.UserName, params.UserEmail, expiresAt)

	booking, err := scanBooking(row)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit transaction: %w", err)
	}

	return booking, nil
}

func (r *EventRepository) ConfirmBooking(ctx context.Context, eventID int64, userEmail string) (domain.Booking, error) {
	userEmail = strings.TrimSpace(userEmail)
	if eventID <= 0 || userEmail == "" {
		return domain.Booking{}, domain.ErrInvalidInput
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := r.getEventForUpdate(ctx, tx, eventID); err != nil {
		return domain.Booking{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = 'confirmed', confirmed_at = NOW()
		WHERE id = (
			SELECT id
			FROM bookings
			WHERE event_id = $1 AND lower(user_email) = lower($2) AND status = 'pending' AND expires_at > NOW()
			ORDER BY created_at DESC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_id, user_name, user_email, status, expires_at, confirmed_at, created_at
	`, eventID, userEmail)

	booking, err := scanBooking(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Booking{}, domain.ErrBookingNotFound
		}
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit transaction: %w", err)
	}

	return booking, nil
}

func (r *EventRepository) CancelExpiredBookings(ctx context.Context, limit int) ([]domain.ExpiredBooking, error) {
	if limit <= 0 {
		limit = 100
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		WITH expired AS (
			SELECT id
			FROM bookings
			WHERE status = 'pending' AND expires_at <= NOW()
			ORDER BY expires_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE bookings b
		SET status = 'canceled', canceled_at = NOW()
		FROM expired, events e
		WHERE b.id = expired.id AND e.id = b.event_id
		RETURNING b.id, b.event_id, b.user_name, b.user_email, b.status, b.expires_at, b.confirmed_at, b.created_at, e.title
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("cancel expired bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]domain.ExpiredBooking, 0)
	for rows.Next() {
		var booking domain.ExpiredBooking
		if err := rows.Scan(
			&booking.ID,
			&booking.EventID,
			&booking.UserName,
			&booking.UserEmail,
			&booking.Status,
			&booking.ExpiresAt,
			&booking.ConfirmedAt,
			&booking.CreatedAt,
			&booking.EventTitle,
		); err != nil {
			return nil, fmt.Errorf("scan expired booking: %w", err)
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired bookings: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return bookings, nil
}

func (r *EventRepository) getEvent(ctx context.Context, eventID int64) (domain.Event, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, title, event_date, capacity, booking_ttl_seconds, requires_payment, created_at
		FROM events
		WHERE id = $1
	`, eventID)

	return scanEvent(row)
}

func (r *EventRepository) getEventForUpdate(ctx context.Context, tx pgx.Tx, eventID int64) (domain.Event, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, title, event_date, capacity, booking_ttl_seconds, requires_payment, created_at
		FROM events
		WHERE id = $1
		FOR UPDATE
	`, eventID)

	return scanEvent(row)
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row eventScanner) (domain.Event, error) {
	var event domain.Event
	var bookingTTLSeconds int
	if err := row.Scan(
		&event.ID,
		&event.Title,
		&event.EventDate,
		&event.Capacity,
		&bookingTTLSeconds,
		&event.RequiresPayment,
		&event.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Event{}, domain.ErrEventNotFound
		}
		return domain.Event{}, fmt.Errorf("scan event: %w", err)
	}
	event.BookingTTL = time.Duration(bookingTTLSeconds) * time.Second

	return event, nil
}

type bookingScanner interface {
	Scan(dest ...any) error
}

func scanBooking(row bookingScanner) (domain.Booking, error) {
	var booking domain.Booking
	if err := row.Scan(
		&booking.ID,
		&booking.EventID,
		&booking.UserName,
		&booking.UserEmail,
		&booking.Status,
		&booking.ExpiresAt,
		&booking.ConfirmedAt,
		&booking.CreatedAt,
	); err != nil {
		return domain.Booking{}, fmt.Errorf("scan booking: %w", err)
	}

	return booking, nil
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
