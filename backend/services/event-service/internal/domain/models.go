package domain

import "time"

type Event struct {
	ID              int64         `json:"id"`
	Title           string        `json:"title"`
	EventDate       time.Time     `json:"eventDate"`
	Capacity        int           `json:"capacity"`
	BookingTTL      time.Duration `json:"bookingTtl"`
	RequiresPayment bool          `json:"requiresPayment"`
	CreatedAt       time.Time     `json:"createdAt"`
}

type BookingStatus string

const (
	BookingStatusPending   BookingStatus = "pending"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCanceled  BookingStatus = "canceled"
)

type Booking struct {
	ID          int64         `json:"id"`
	EventID     int64         `json:"eventId"`
	UserName    string        `json:"userName"`
	UserEmail   string        `json:"userEmail"`
	Status      BookingStatus `json:"status"`
	ExpiresAt   time.Time     `json:"expiresAt"`
	ConfirmedAt *time.Time    `json:"confirmedAt,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
}

type EventDetails struct {
	Event          Event     `json:"event"`
	FreeSeats      int       `json:"freeSeats"`
	PendingCount   int       `json:"pendingCount"`
	ConfirmedCount int       `json:"confirmedCount"`
	Bookings       []Booking `json:"bookings"`
}

type ExpiredBooking struct {
	Booking
	EventTitle string `json:"eventTitle"`
}
