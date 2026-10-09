package integration

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// AC-OP-01, for the whole process: builds the real binary, runs it against
// the test database, sends SIGTERM and checks it shuts down in order and
// exits with status 0.
func TestShutdown_ProcessExitsCleanlyOnSIGTERM(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "web")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/web")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build: %s", out)

	// Find a free port for the server.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := probe.Addr().(*net.TCPAddr).Port
	require.NoError(t, probe.Close())

	var logs bytes.Buffer
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"DATABASE_URL="+databaseURL,
		"SWEEP_INTERVAL=10ms",
		"GIN_MODE=release",
	)
	cmd.Stdout, cmd.Stderr = &logs, &logs
	require.NoError(t, cmd.Start())
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		res, err := http.Get(base + "/ready")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 20*time.Second, 20*time.Millisecond, "server never became ready; logs:\n%s", &logs)

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	select {
	case err := <-exited:
		require.NoError(t, err, "process exit status; logs:\n%s", &logs)
	case <-time.After(20 * time.Second):
		t.Fatalf("process did not exit after SIGTERM; logs:\n%s", &logs)
	}

	// The shutdown steps appear in order, with the pool closed last.
	output := logs.String()
	last := -1
	for _, step := range []string{"http server stopped", "sweeper stopped", "database pool closed", `"msg":"server stopped"`} {
		at := strings.Index(output, step)
		require.NotEqual(t, -1, at, "log has no %q; logs:\n%s", step, output)
		require.Greater(t, at, last, "%q is out of order; logs:\n%s", step, output)
		last = at
	}
	require.NotContains(t, output, `"level":"ERROR"`, "errors were logged during shutdown")

	// The database is untouched by the shutdown and still serves this test.
	require.NoError(t, db.Ping(t.Context()))
}
