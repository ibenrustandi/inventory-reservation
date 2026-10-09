// Package sweeper runs the background job that returns the stock of
// reservations past their deadline.
package sweeper

import (
	"context"
	"log/slog"
	"time"
)

// Releaser releases every reservation that is past its deadline and
// reports how many it released.
type Releaser interface {
	ReleaseExpired(ctx context.Context) (int, error)
}

// Sweeper calls a Releaser on a fixed interval.
type Sweeper struct {
	releaser Releaser
	interval time.Duration
}

// New returns a Sweeper that sweeps every interval.
func New(releaser Releaser, interval time.Duration) *Sweeper {
	return &Sweeper{releaser: releaser, interval: interval}
}

// SweepOnce does one sweep, with no waiting, and returns how many
// reservations it released.
func (s *Sweeper) SweepOnce(ctx context.Context) (int, error) {
	return s.releaser.ReleaseExpired(ctx)
}

// sweepTimeout bounds one sweep, so a stuck database cannot stop the
// service from shutting down.
const sweepTimeout = 10 * time.Second

// Run sweeps every interval until ctx is cancelled, then returns. A sweep
// that is running when ctx is cancelled finishes first, so shutting down
// never abandons a release half way. A failed sweep is logged and the next
// one runs as usual.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// When both ctx and the ticker are ready, select picks either.
			// Never start a sweep once ctx is cancelled.
			if ctx.Err() != nil {
				return
			}
			// The sweep gets a context that outlives a cancelled ctx.
			sweepCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sweepTimeout)
			released, err := s.SweepOnce(sweepCtx)
			cancel()
			switch {
			case err != nil:
				slog.ErrorContext(ctx, "sweep failed", "error", err)
			case released > 0:
				slog.InfoContext(ctx, "released expired reservations", "count", released)
			}
		}
	}
}
