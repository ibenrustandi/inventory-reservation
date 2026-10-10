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
	// DatabaseMaxConns is the most connections this instance opens to
	// PostgreSQL. Every instance has its own pool, so the total across
	// instances must stay below the server's max_connections.
	DatabaseMaxConns int
	// ReservationTTL is how long a reservation holds stock before it expires.
	ReservationTTL time.Duration
	// SweepInterval is how often the background sweeper releases
	// reservations that are past their deadline.
	SweepInterval time.Duration
	// RequestTimeout is the longest one request may run. A request that
	// is still waiting on the database when it passes is cancelled, which
	// frees its connection, and answered with 503.
	RequestTimeout time.Duration
	// ShutdownTimeout is how long requests in flight get to finish after
	// the service is asked to stop. It is longer than RequestTimeout, so
	// shutting down never has to cut a request off.
	ShutdownTimeout time.Duration
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
	databaseMaxConns, err := intEnv("DB_MAX_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	if databaseMaxConns < 1 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be at least 1, got %d", databaseMaxConns)
	}
	reservationTTL, err := durationEnv("RESERVATION_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	if reservationTTL <= 0 {
		return Config{}, fmt.Errorf("RESERVATION_TTL must be positive, got %s", reservationTTL)
	}
	sweepInterval, err := durationEnv("SWEEP_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}
	if sweepInterval <= 0 {
		return Config{}, fmt.Errorf("SWEEP_INTERVAL must be positive, got %s", sweepInterval)
	}
	shutdownTimeout, err := durationEnv("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	if shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %s", shutdownTimeout)
	}
	requestTimeout, err := durationEnv("REQUEST_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	if requestTimeout <= 0 {
		return Config{}, fmt.Errorf("REQUEST_TIMEOUT must be positive, got %s", requestTimeout)
	}
	if requestTimeout >= shutdownTimeout {
		return Config{}, fmt.Errorf("REQUEST_TIMEOUT (%s) must be below SHUTDOWN_TIMEOUT (%s)", requestTimeout, shutdownTimeout)
	}
	return Config{
		Port:             port,
		DatabaseURL:      databaseURL,
		DatabaseMaxConns: databaseMaxConns,
		ReservationTTL:   reservationTTL,
		SweepInterval:    sweepInterval,
		RequestTimeout:   requestTimeout,
		ShutdownTimeout:  shutdownTimeout,
	}, nil
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
