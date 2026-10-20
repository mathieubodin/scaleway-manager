// Command scaleway-manager serves the web interface.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	scalewaymanager "github.com/mathieubodin/scaleway-manager"
	"github.com/mathieubodin/scaleway-manager/internal/server"
)

// shutdownTimeout bounds the wait for in-flight requests after the first signal.
const shutdownTimeout = 10 * time.Second

const defaultListenAddr = ":8080"

func main() {
	// Never stopped: a second signal must not fall back to the default action
	// (immediate exit) while the shutdown runs.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	os.Exit(run(os.Getenv, os.Stdout, sigs, server.New(scalewaymanager.IndexHTML), shutdownTimeout))
}

// listenAddr is LISTEN_ADDR, or :8080 when it is unset or empty.
func listenAddr(getenv func(string) string) string {
	if a := getenv("LISTEN_ADDR"); a != "" {
		return a
	}
	return defaultListenAddr
}

// run serves h until the first signal on sigs, then shuts down within delay.
// Every line written to stdout is one slog JSON object. It returns the exit code.
func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, h http.Handler, delay time.Duration) int {
	handler := slog.NewJSONHandler(stdout, nil)
	logger := slog.New(handler)

	ln, err := net.Listen("tcp", listenAddr(getenv))
	if err != nil {
		logger.Error("listen failed", "error", err.Error())
		return 1
	}

	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(handler, slog.LevelWarn),
	}

	logger.Info("listening", "addr", ln.Addr().String())
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case err := <-served:
		// No Shutdown was called, so this is never http.ErrServerClosed.
		logger.Error("serve failed", "error", err.Error())
		return 1
	case <-sigs:
	}

	// Later signals are drained and ignored: the delay runs from the first one.
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-sigs:
			case <-done:
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), delay)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error("shutdown timed out")
		} else {
			logger.Error("shutdown failed", "error", err.Error())
		}
		_ = srv.Close()
		return 1
	}
	logger.Info("stopped")
	return 0
}
