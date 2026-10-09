// Package config reads service settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every setting the service reads at startup.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// ReservationTTL is how long a reservation holds stock before it expires.
	ReservationTTL time.Duration
}

// Load reads the configuration from the environment, applying defaults
// for anything that is not set.
func Load() (Config, error) {
	port, err := intEnv("PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	if port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535, got %d", port)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	reservationTTL, err := durationEnv("RESERVATION_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	if reservationTTL <= 0 {
		return Config{}, fmt.Errorf("RESERVATION_TTL must be positive, got %s", reservationTTL)
	}
	return Config{Port: port, DatabaseURL: databaseURL, ReservationTTL: reservationTTL}, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 5m or 30s, got %q", key, raw)
	}
	return v, nil
}

func intEnv(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return v, nil
}
