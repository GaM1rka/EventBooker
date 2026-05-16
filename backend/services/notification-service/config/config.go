package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const defaultEnvPath = ".env"

type Config struct {
	Client ClientConfig
	Kafka  KafkaConfig
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

type ClientConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	UseTLS   bool
}

func Load() (*Config, error) {
	if err := loadDotEnv(defaultEnvPath); err != nil {
		return nil, err
	}

	cfg := &Config{
		Client: ClientConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnv("SMTP_PORT", "587"),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", getEnv("SMTP_USERNAME", "")),
			UseTLS:   getBoolEnv("SMTP_USE_TLS", getEnv("SMTP_PORT", "") == "465"),
		},
		Kafka: KafkaConfig{
			Brokers: splitCSV(getEnv("KAFKA_BROKERS", getEnv("KAFKA_BROKER", "localhost:9092"))),
			Topic:   getEnv("KAFKA_TOPIC", "notification.sent"),
			GroupID: getEnv("KAFKA_GROUP_ID", "notification-service"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Client.Host == "" {
		return errors.New("SMTP_HOST is required")
	}
	if c.Client.Port == "" {
		return errors.New("SMTP_PORT is required")
	}
	if c.Client.Username == "" {
		return errors.New("SMTP_USERNAME is required")
	}
	if c.Client.Password == "" {
		return errors.New("SMTP_PASSWORD is required")
	}
	if c.Client.From == "" {
		return errors.New("SMTP_FROM is required")
	}
	if len(c.Kafka.Brokers) == 0 {
		return errors.New("KAFKA_BROKERS or KAFKA_BROKER is required")
	}
	if c.Kafka.Topic == "" {
		return errors.New("KAFKA_TOPIC is required")
	}
	if c.Kafka.GroupID == "" {
		return errors.New("KAFKA_GROUP_ID is required")
	}

	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNumber)
		}

		key = strings.TrimSpace(key)
		value = cleanEnvValue(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty env key", path, lineNumber)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: set %s: %w", path, lineNumber, key, err)
		}
	}

	return scanner.Err()
}

func cleanEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}

	if before, _, ok := strings.Cut(value, " #"); ok {
		return strings.TrimSpace(before)
	}

	return value
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
