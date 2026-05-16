package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"strconv"
	"strings"

	"event-service/internal/domain"
	"event-service/internal/service"
)

type EventService interface {
	CreateEvent(ctx context.Context, req service.CreateEventRequest) (domain.Event, error)
	ListEventDetails(ctx context.Context) ([]domain.EventDetails, error)
	GetEventDetails(ctx context.Context, eventID int64) (domain.EventDetails, error)
	BookEvent(ctx context.Context, eventID int64, req service.BookEventRequest) (domain.Booking, error)
	ConfirmBooking(ctx context.Context, eventID int64, req service.ConfirmBookingRequest) (domain.Booking, error)
}

type Handler struct {
	service EventService
	logger  *slog.Logger
}

func NewHandler(service EventService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Routes() nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/events", h.events)
	mux.HandleFunc("/events/", h.eventByID)

	return h.withCORS(mux)
}

func (h *Handler) health(w nethttp.ResponseWriter, r *nethttp.Request) {
	writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) events(w nethttp.ResponseWriter, r *nethttp.Request) {
	switch r.Method {
	case nethttp.MethodGet:
		events, err := h.service.ListEventDetails(r.Context())
		if err != nil {
			h.writeError(w, err)
			return
		}
		writeJSON(w, nethttp.StatusOK, events)
	case nethttp.MethodPost:
		var req service.CreateEventRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, nethttp.StatusBadRequest, errorResponse("invalid json body"))
			return
		}
		event, err := h.service.CreateEvent(r.Context(), req)
		if err != nil {
			h.writeError(w, err)
			return
		}
		writeJSON(w, nethttp.StatusCreated, event)
	default:
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
	}
}

func (h *Handler) eventByID(w nethttp.ResponseWriter, r *nethttp.Request) {
	eventID, action, ok := parseEventPath(r.URL.Path)
	if !ok {
		nethttp.NotFound(w, r)
		return
	}

	switch {
	case r.Method == nethttp.MethodGet && action == "":
		event, err := h.service.GetEventDetails(r.Context(), eventID)
		if err != nil {
			h.writeError(w, err)
			return
		}
		writeJSON(w, nethttp.StatusOK, event)
	case r.Method == nethttp.MethodPost && action == "book":
		var req service.BookEventRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, nethttp.StatusBadRequest, errorResponse("invalid json body"))
			return
		}
		booking, err := h.service.BookEvent(r.Context(), eventID, req)
		if err != nil {
			h.writeError(w, err)
			return
		}
		writeJSON(w, nethttp.StatusCreated, booking)
	case r.Method == nethttp.MethodPost && action == "confirm":
		var req service.ConfirmBookingRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, nethttp.StatusBadRequest, errorResponse("invalid json body"))
			return
		}
		booking, err := h.service.ConfirmBooking(r.Context(), eventID, req)
		if err != nil {
			h.writeError(w, err)
			return
		}
		writeJSON(w, nethttp.StatusOK, booking)
	default:
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
	}
}

func (h *Handler) writeError(w nethttp.ResponseWriter, err error) {
	h.logger.Error("request failed", "error", err)

	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeJSON(w, nethttp.StatusBadRequest, errorResponse(err.Error()))
	case errors.Is(err, domain.ErrEventNotFound), errors.Is(err, domain.ErrBookingNotFound):
		writeJSON(w, nethttp.StatusNotFound, errorResponse(err.Error()))
	case errors.Is(err, domain.ErrNoSeatsAvailable):
		writeJSON(w, nethttp.StatusConflict, errorResponse(err.Error()))
	default:
		writeJSON(w, nethttp.StatusInternalServerError, errorResponse("internal server error"))
	}
}

func (h *Handler) withCORS(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == nethttp.MethodOptions {
			w.WriteHeader(nethttp.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseEventPath(path string) (int64, string, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "events" {
		return 0, "", false
	}

	eventID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || eventID <= 0 {
		return 0, "", false
	}
	action := ""
	if len(parts) == 3 {
		action = parts[2]
	}

	return eventID, action, true
}

func readJSON(r *nethttp.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeJSON(w nethttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func errorResponse(message string) map[string]string {
	return map[string]string{"error": message}
}
