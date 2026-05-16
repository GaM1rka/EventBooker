package smtp

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"notification-service/config"
	"strings"
)

type Client struct {
	host     string
	port     string
	username string
	password string
	from     string
	useTLS   bool
	logger   *slog.Logger
}

func NewClient(config *config.Config, logger *slog.Logger) *Client {
	return &Client{
		host:     config.Client.Host,
		port:     config.Client.Port,
		username: config.Client.Username,
		password: config.Client.Password,
		from:     config.Client.From,
		useTLS:   config.Client.UseTLS,
		logger:   logger,
	}
}

func (c *Client) Send(to, subject, body string) error {
	auth := smtp.PlainAuth("", c.username, c.password, c.host)
	addr := c.host + ":" + c.port
	msg := c.buildMessage(to, subject, body)

	c.logger.Info("sending email", "to", to, "subject", subject)
	if c.useTLS {
		return c.sendWithTLS(addr, auth, to, msg)
	}

	return smtp.SendMail(addr, auth, c.from, []string{to}, msg)
}

func (c *Client) buildMessage(to, subject, body string) []byte {
	msg := []byte(
		"From: " + c.from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"Content-Transfer-Encoding: 8bit\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	return msg
}

func (c *Client) sendWithTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: c.host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("connect smtp tls: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(c.from); err != nil {
		return fmt.Errorf("set sender: %w", err)
	}
	if err := client.Rcpt(strings.TrimSpace(to)); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open message writer: %w", err)
	}
	if _, err := writer.Write(msg); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close message writer: %w", err)
	}

	return client.Quit()
}
