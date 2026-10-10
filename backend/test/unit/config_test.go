package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/config"
)

func TestConfigLoad(t *testing.T) {
	// clearEnv makes every setting unset except the required one.
	clearEnv := func(t *testing.T) {
		t.Helper()
		for _, key := range []string{"PORT", "DB_MAX_CONNS", "RESERVATION_TTL", "SWEEP_INTERVAL", "SHUTDOWN_TIMEOUT"} {
			t.Setenv(key, "")
		}
		t.Setenv("DATABASE_URL", "postgres://localhost/inventory")
	}

	t.Run("applies defaults", func(t *testing.T) {
		clearEnv(t)

		cfg, err := config.Load()

		require.NoError(t, err)
		require.Equal(t, config.Config{
			Port:             8080,
			DatabaseURL:      "postgres://localhost/inventory",
			DatabaseMaxConns: 20,
			ReservationTTL:   5 * time.Minute,
			SweepInterval:    time.Second,
			ShutdownTimeout:  15 * time.Second,
		}, cfg)
	})

	t.Run("reads every setting", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("PORT", "9090")
		t.Setenv("DB_MAX_CONNS", "8")
		t.Setenv("RESERVATION_TTL", "30s")
		t.Setenv("SWEEP_INTERVAL", "250ms")
		t.Setenv("SHUTDOWN_TIMEOUT", "1m")

		cfg, err := config.Load()

		require.NoError(t, err)
		require.Equal(t, 9090, cfg.Port)
		require.Equal(t, 8, cfg.DatabaseMaxConns)
		require.Equal(t, 30*time.Second, cfg.ReservationTTL)
		require.Equal(t, 250*time.Millisecond, cfg.SweepInterval)
		require.Equal(t, time.Minute, cfg.ShutdownTimeout)
	})

	invalid := []struct {
		name  string
		key   string
		value string
	}{
		{"missing database url", "DATABASE_URL", ""},
		{"port is not a number", "PORT", "http"},
		{"port out of range", "PORT", "70000"},
		{"pool size is zero", "DB_MAX_CONNS", "0"},
		{"pool size is not a number", "DB_MAX_CONNS", "many"},
		{"ttl is not a duration", "RESERVATION_TTL", "5"},
		{"ttl is negative", "RESERVATION_TTL", "-1s"},
		{"sweep interval is zero", "SWEEP_INTERVAL", "0s"},
		{"shutdown timeout is zero", "SHUTDOWN_TIMEOUT", "0s"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := config.Load()

			require.ErrorContains(t, err, tt.key)
		})
	}
}
