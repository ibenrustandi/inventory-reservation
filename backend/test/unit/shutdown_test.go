package unit

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/sweeper"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

// slowServer runs infrastructure.Serve with a handler that reports when a
// request has arrived and then waits to be released.
type slowServer struct {
	url      string
	arrived  chan struct{}
	release  chan struct{}
	shutdown context.CancelFunc
	result   chan error
}

func startSlowServer(t *testing.T, shutdownTimeout time.Duration) *slowServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &slowServer{
		url:     "http://" + listener.Addr().String(),
		arrived: make(chan struct{}),
		release: make(chan struct{}),
		result:  make(chan error, 1),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(s.arrived)
		<-s.release
		_, _ = w.Write([]byte("done"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	s.shutdown = cancel
	go func() { s.result <- infrastructure.Serve(ctx, listener, handler, shutdownTimeout) }()
	t.Cleanup(cancel)
	return s
}

type response struct {
	status int
	body   string
	err    error
}

// get sends a request in the background and delivers its outcome.
func (s *slowServer) get() <-chan response {
	out := make(chan response, 1)
	go func() {
		// A fresh transport, so the request cannot reuse an idle connection.
		client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
		res, err := client.Get(s.url)
		if err != nil {
			out <- response{err: err}
			return
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		out <- response{status: res.StatusCode, body: string(body), err: err}
	}()
	return out
}

// AC-OP-01
func TestShutdown_Graceful(t *testing.T) {
	server := startSlowServer(t, 5*time.Second)
	inFlight := server.get()
	<-server.arrived

	server.shutdown()

	// While the request is in flight the server keeps running, but takes
	// no new connections.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", server.url[len("http://"):], time.Second)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	}, 5*time.Second, 5*time.Millisecond, "server still accepts connections after shutdown began")
	select {
	case err := <-server.result:
		t.Fatalf("Serve returned (%v) while a request was still in flight", err)
	default:
	}

	close(server.release)

	// The request completes with its normal response, and only then does
	// the server exit, without error.
	got := <-inFlight
	require.NoError(t, got.err)
	require.Equal(t, http.StatusOK, got.status)
	require.Equal(t, "done", got.body)
	select {
	case err := <-server.result:
		require.NoError(t, err, "Serve after a clean shutdown")
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the in-flight request finished")
	}
}

// A request that outlives the shutdown timeout is cut off, and Serve says so.
func TestShutdown_TimesOut(t *testing.T) {
	server := startSlowServer(t, 20*time.Millisecond)
	inFlight := server.get()
	<-server.arrived

	server.shutdown()

	select {
	case err := <-server.result:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the shutdown timeout")
	}
	close(server.release)
	require.Error(t, (<-inFlight).err, "the request that was cut off")
}

// Part of AC-OP-01: stopping the sweeper lets a sweep in progress finish.
func TestSweeperRun_FinishesCurrentSweep(t *testing.T) {
	sweeping := make(chan struct{})
	finish := make(chan struct{})
	var sweepCtxErr error
	releaser := &mocks.InventoryUsecase{
		ReleaseExpiredFunc: func(ctx context.Context) (int, error) {
			close(sweeping)
			<-finish
			sweepCtxErr = ctx.Err()
			return 1, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		sweeper.New(releaser, time.Millisecond).Run(ctx)
	}()
	<-sweeping

	cancel()

	select {
	case <-stopped:
		t.Fatal("Run returned while a sweep was still in progress")
	case <-time.After(50 * time.Millisecond):
	}
	close(finish)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its sweep finished")
	}
	require.NoError(t, sweepCtxErr, "the sweep in progress was cancelled by shutdown")
}
