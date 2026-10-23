package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const (
	defaultStepDelay  = 2 * time.Second
	defaultHealthAddr = ":8081"
	passTimeout       = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
	readHeaderTimeout = 10 * time.Second
	guardNamespace    = "scaleway-system"
	guardLabel        = "app.kubernetes.io/name"
	guardLabelValue   = "scaleway-operator"
)

// run is the start-up guard, the polling loop and /healthz (CAP-8). It returns
// the process exit code: 1 on a start-up failure, 0 after a signal.
func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int {
	log := slog.New(slog.NewJSONHandler(stdout, nil))
	startFailed := func(err error) int {
		log.Error("start failed", "error", err.Error())
		return 1
	}

	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		if _, envErr := kube.NewInCluster(getenv, nil); errors.Is(envErr, kube.ErrNoAPIServer) {
			return startFailed(envErr)
		}
		return startFailed(fmt.Errorf("fake-operator: read CA: %w", err))
	}
	client, err := kube.NewInCluster(getenv, caPEM)
	if err != nil {
		return startFailed(err)
	}

	delay, err := stepDelay(getenv("FAKE_OPERATOR_STEP_DELAY"))
	if err != nil {
		return startFailed(err)
	}

	if code := guard(client, tokenPath, log); code != 0 {
		return code
	}

	addr := getenv("HEALTH_ADDR")
	if addr == "" {
		addr = defaultHealthAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return startFailed(fmt.Errorf("fake-operator: health listen: %w", err))
	}
	h := &health{clock: clock, maxAge: 3 * delay}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.serveHTTP)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderTimeout}
	served := make(chan struct{})
	go func() {
		defer close(served)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server failed", "error", err.Error())
		}
	}()
	log.Info("listening", "addr", ln.Addr().String())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-ctx.Done():
		}
	}()

	op := newOperator(client, clock, rnd, tokenPath, log)
	op.stop = ctx.Done()
	loop(ctx, op, delay, h, log)

	shutCtx, shutCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("health shutdown failed", "error", err.Error())
	}
	<-served
	log.Info("stopped")
	return 0
}

// stepDelay parses FAKE_OPERATOR_STEP_DELAY: default 2s, positive Go duration.
func stepDelay(v string) (time.Duration, error) {
	if v == "" {
		return defaultStepDelay, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("fake-operator: FAKE_OPERATOR_STEP_DELAY: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("fake-operator: FAKE_OPERATOR_STEP_DELAY %s is not positive", d)
	}
	return d, nil
}

// guard refuses to run beside the real operator. It returns 0 when the loop may start.
func guard(c *kube.Client, tokenPath string, log *slog.Logger) int {
	tok, err := readToken(tokenPath)
	if err != nil {
		log.Error("start failed", "error", err.Error())
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), passTimeout)
	defer cancel()
	items, err := c.List(ctx, tok, kube.Deployments, guardNamespace)
	if err != nil {
		log.Error("start failed", "error", err.Error())
		return 1
	}
	for _, raw := range items {
		var d struct {
			Metadata struct {
				Name      string            `json:"name"`
				Namespace string            `json:"namespace"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			log.Error("start failed", "error", fmt.Errorf("fake-operator: decode deployment: %w", err).Error())
			return 1
		}
		if d.Metadata.Labels[guardLabel] == guardLabelValue {
			log.Error("real operator found", "deployment", d.Metadata.Namespace+"/"+d.Metadata.Name)
			return 1
		}
	}
	return 0
}

// loop runs passes until ctx is canceled. The first pass starts at once; the
// next starts one delay after the previous one ends. The signal never cancels
// a request: a pass runs on its own context bounded by passTimeout, and stops
// between two objects once op.stop is closed.
func loop(ctx context.Context, op *operator, delay time.Duration, h *health, log *slog.Logger) {
	for ctx.Err() == nil {
		passCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), passTimeout)
		err := op.pass(passCtx)
		cancel()
		if err != nil {
			log.Error("pass failed", "error", err.Error())
		} else {
			h.ended()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// health holds the end time of the last pass for /healthz.
type health struct {
	clock  Clock
	maxAge time.Duration

	mu      sync.Mutex
	hasEnd  bool
	lastEnd time.Time
}

func (h *health) ended() {
	now := h.clock.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hasEnd = true
	h.lastEnd = now
}

func (h *health) serveHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	ok := h.hasEnd && h.clock.Now().Sub(h.lastEnd) < h.maxAge
	h.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok")
}
