package domain

import "errors"

var (
	ErrEventNotFound    = errors.New("event not found")
	ErrBookingNotFound  = errors.New("booking not found")
	ErrNoSeatsAvailable = errors.New("no seats available")
	ErrInvalidInput     = errors.New("invalid input")
)
