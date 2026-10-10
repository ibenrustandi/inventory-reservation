package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Connection timeouts. They protect the server from clients that send or
// read slowly. writeTimeout must stay above the request timeout set in the
// route package (10 seconds), so a request that runs out of time still gets
// its error response written.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

// Serve answers HTTP requests on the listener until ctx is cancelled, then
// shuts down gracefully: it stops accepting new connections and waits up
// to shutdownTimeout for requests in flight to finish. It returns nil
// after a clean shutdown.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	select {
	case err := <-serveErr:
		// The server stopped by itself; it was never asked to shut down.
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// ctx is already cancelled, so the deadline needs a fresh context.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests were still running at the deadline; cut them off.
		_ = srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
