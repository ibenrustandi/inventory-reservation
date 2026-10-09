// Package config reads service settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds every setting the service reads at startup.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
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
	return Config{Port: port}, nil
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
