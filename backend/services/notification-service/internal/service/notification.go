package service

import (
	"errors"
	"log/slog"
	"net/mail"
	"strings"

	"notification-service/internal/domain/models"
)

type EmailSender interface {
	Send(to, subject, body string) error
}

type NotificationService struct {
	emailSender EmailSender
	logger      *slog.Logger
}

func NewNotificationService(emailSender EmailSender, logger *slog.Logger) *NotificationService {
	return &NotificationService{
		emailSender: emailSender,
		logger:      logger,
	}
}

func (s *NotificationService) SendEmailNotification(n models.EmailNotification) error {
	n.Email = strings.TrimSpace(n.Email)
	n.Subject = strings.TrimSpace(n.Subject)
	n.Message = strings.TrimSpace(n.Message)

	if n.Email == "" {
		return errors.New("email is empty")
	}
	addr, err := mail.ParseAddress(n.Email)
	if err != nil {
		return errors.New("email is invalid")
	}
	n.Email = addr.Address
	if n.Subject == "" {
		return errors.New("subject is empty")
	}
	if strings.ContainsAny(n.Subject, "\r\n") {
		return errors.New("subject is invalid")
	}
	if n.Message == "" {
		return errors.New("message is empty")
	}

	if err := s.emailSender.Send(n.Email, n.Subject, n.Message); err != nil {
		return err
	}

	s.logger.Info("email notification sent", "to", n.Email, "subject", n.Subject)
	return nil
}
