package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultEnvPath = ".env"

type Config struct {
	ServerAddr              string
	DatabaseURL             string
	DefaultBookingTTL       time.Duration
	WorkerInterval          time.Duration
	GracefulShutdownTimeout time.Duration
	Kafka                   KafkaConfig
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
}

func Load() (*Config, error) {
	if err := loadDotEnv(defaultEnvPath); err != nil {
		return nil, err
	}

	cfg := &Config{
		ServerAddr:              getEnv("SERVER_ADDR", ":8080"),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/eventbooker?sslmode=disable"),
		DefaultBookingTTL:       getDurationEnv("DEFAULT_BOOKING_TTL", 15*time.Minute),
		WorkerInterval:          getDurationEnv("WORKER_INTERVAL", 10*time.Second),
		GracefulShutdownTimeout: getDurationEnv("GRACEFUL_SHUTDOWN_TIMEOUT", 10*time.Second),
		Kafka: KafkaConfig{
			Brokers: splitCSV(getEnv("KAFKA_BROKERS", getEnv("KAFKA_BROKER", "localhost:9092"))),
			Topic:   getEnv("KAFKA_TOPIC", "notification.sent"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.ServerAddr == "" {
		return errors.New("SERVER_ADDR is required")
	}
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.DefaultBookingTTL <= 0 {
		return errors.New("DEFAULT_BOOKING_TTL must be positive")
	}
	if c.WorkerInterval <= 0 {
		return errors.New("WORKER_INTERVAL must be positive")
	}
	if len(c.Kafka.Brokers) == 0 {
		return errors.New("KAFKA_BROKERS or KAFKA_BROKER is required")
	}
	if c.Kafka.Topic == "" {
		return errors.New("KAFKA_TOPIC is required")
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

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}

	minutes, err := strconv.Atoi(value)
	if err == nil {
		return time.Duration(minutes) * time.Minute
	}

	return fallback
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
