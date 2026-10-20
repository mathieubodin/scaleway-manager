package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// syncBuffer is the stdout given to run: written by run, read by the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type logLine map[string]any

func (l logLine) str(key string) string {
	s, _ := l[key].(string)
	return s
}

// lines parses every stdout line as one JSON object; a line that is not fails the test.
func lines(t *testing.T, out *syncBuffer) []logLine {
	t.Helper()
	raw := out.String()
	if raw == "" {
		return nil
	}
	if !strings.HasSuffix(raw, "\n") {
		t.Errorf("stdout does not end with a newline: %q", raw)
	}
	var parsed []logLine
	for _, s := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		var l logLine
		if err := json.Unmarshal([]byte(s), &l); err != nil {
			t.Errorf("stdout line is not one JSON object: %q (%v)", s, err)
			continue
		}
		if l.str("level") == "" || l.str("msg") == "" {
			t.Errorf("stdout line lacks level or msg: %q", s)
		}
		parsed = append(parsed, l)
	}
	return parsed
}

func withMsg(ls []logLine, msg string) []logLine {
	var out []logLine
	for _, l := range ls {
		if l.str("msg") == msg {
			out = append(out, l)
		}
	}
	return out
}

func withLevel(ls []logLine, level string) []logLine {
	var out []logLine
	for _, l := range ls {
		if l.str("level") == level {
			out = append(out, l)
		}
	}
	return out
}

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

type running struct {
	sigs chan os.Signal
	out  *syncBuffer
	code chan int
	addr string
}

// start runs run on 127.0.0.1:0 and waits for its listening line.
func start(t *testing.T, h http.Handler, delay time.Duration) *running {
	t.Helper()
	r := &running{
		sigs: make(chan os.Signal, 8),
		out:  &syncBuffer{},
		code: make(chan int, 1),
	}
	go func() {
		r.code <- run(env(map[string]string{"LISTEN_ADDR": "127.0.0.1:0"}), r.out, r.sigs, h, delay)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case c := <-r.code:
			t.Fatalf("run returned %d before listening; stdout:\n%s", c, r.out.String())
		default:
		}
		if l := withMsg(lines(t, r.out), "listening"); len(l) > 0 {
			r.addr = l[0].str("addr")
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no listening line within 5 s; stdout:\n%s", r.out.String())
	return nil
}

func (r *running) wait(t *testing.T, within time.Duration) int {
	t.Helper()
	select {
	case c := <-r.code:
		return c
	case <-time.After(within):
		t.Fatalf("run did not return within %v; stdout:\n%s", within, r.out.String())
		return -1
	}
}

func (r *running) notReturned(t *testing.T, during time.Duration) {
	t.Helper()
	select {
	case c := <-r.code:
		t.Fatalf("run returned %d too early; stdout:\n%s", c, r.out.String())
	case <-time.After(during):
	}
}

// blockingHandler answers "done" once released; entered is closed when a request arrives.
type blockingHandler struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingHandler() *blockingHandler {
	return &blockingHandler{entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *blockingHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	_, _ = io.WriteString(w, "done")
}

type result struct {
	status int
	body   string
	err    error
}

// inFlight sends GET / and waits until the handler holds it.
func inFlight(t *testing.T, addr string, h *blockingHandler) chan result {
	t.Helper()
	res := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			res <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		res <- result{status: resp.StatusCode, body: string(b), err: err}
	}()
	select {
	case <-h.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the request did not reach the handler within 5 s")
	}
	return res
}

func TestShutdownTimeoutIsTenSeconds(t *testing.T) {
	if shutdownTimeout != 10*time.Second {
		t.Errorf("shutdownTimeout = %v, want 10s", shutdownTimeout)
	}
}

// Matrix row Address.
func TestListenAddr(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"unset", map[string]string{}, ":8080"},
		{"empty", map[string]string{"LISTEN_ADDR": ""}, ":8080"},
		{"set", map[string]string{"LISTEN_ADDR": "127.0.0.1:9999"}, "127.0.0.1:9999"},
		{"ipv6", map[string]string{"LISTEN_ADDR": "[::]:8081"}, "[::]:8081"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := listenAddr(env(tc.vars)); got != tc.want {
				t.Errorf("listenAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

// Matrix rows Start and Stop (no request in flight); a request writes no line.
func TestStartServeStop(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hi") })
			r := start(t, h, 2*time.Second)

			host, port, err := net.SplitHostPort(r.addr)
			if err != nil {
				t.Fatalf("addr %q: %v", r.addr, err)
			}
			if host != "127.0.0.1" {
				t.Errorf("addr host %q, want 127.0.0.1", host)
			}
			if p, err := strconv.Atoi(port); err != nil || p <= 0 {
				t.Errorf("addr port %q, want the bound port (> 0)", port)
			}

			before := len(lines(t, r.out))
			resp, err := http.Get("http://" + r.addr + "/")
			if err != nil {
				t.Fatalf("GET on the logged addr: %v", err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if string(b) != "hi" {
				t.Errorf("body %q, want the injected handler's \"hi\"", b)
			}
			if after := len(lines(t, r.out)); after != before {
				t.Errorf("a request wrote %d line(s); want none:\n%s", after-before, r.out.String())
			}

			r.sigs <- sig
			if code := r.wait(t, 5*time.Second); code != 0 {
				t.Errorf("exit code %d, want 0", code)
			}
			ls := lines(t, r.out)
			if n := len(withMsg(ls, "listening")); n != 1 {
				t.Errorf("%d listening line(s), want 1", n)
			}
			l := withMsg(ls, "listening")
			if len(l) == 1 && l[0].str("level") != "INFO" {
				t.Errorf("listening level %q, want INFO", l[0].str("level"))
			}
			stopped := withMsg(ls, "stopped")
			if len(stopped) != 1 || stopped[0].str("level") != "INFO" {
				t.Errorf("want one INFO stopped line; stdout:\n%s", r.out.String())
			}
			if n := len(withLevel(ls, "ERROR")); n != 0 {
				t.Errorf("%d ERROR line(s), want 0", n)
			}
			if len(ls) != 2 {
				t.Errorf("%d line(s), want exactly listening and stopped:\n%s", len(ls), r.out.String())
			}
		})
	}
}

// Matrix row Stop: a request in flight completes, then stopped, exit 0.
func TestStopWaitsForInFlightRequest(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			h := newBlockingHandler()
			r := start(t, h, 5*time.Second)
			res := inFlight(t, r.addr, h)

			r.sigs <- sig
			r.notReturned(t, 300*time.Millisecond)
			if n := len(withMsg(lines(t, r.out), "stopped")); n != 0 {
				t.Errorf("stopped written while a request is in flight")
			}
			close(h.release)

			got := <-res
			if got.err != nil || got.status != http.StatusOK || got.body != "done" {
				t.Errorf("in-flight request: status %d body %q err %v; want 200 \"done\"", got.status, got.body, got.err)
			}
			if code := r.wait(t, 5*time.Second); code != 0 {
				t.Errorf("exit code %d, want 0", code)
			}
			ls := lines(t, r.out)
			if n := len(withMsg(ls, "stopped")); n != 1 {
				t.Errorf("%d stopped line(s), want 1", n)
			}
			if n := len(withLevel(ls, "ERROR")); n != 0 {
				t.Errorf("%d ERROR line(s), want 0", n)
			}
		})
	}
}

// Matrix row Slow stop: one ERROR shutdown timed out, no stopped line, exit 1.
func TestSlowStop(t *testing.T) {
	h := newBlockingHandler()
	defer close(h.release)
	delay := 300 * time.Millisecond
	r := start(t, h, delay)
	res := inFlight(t, r.addr, h)

	sent := time.Now()
	r.sigs <- syscall.SIGTERM
	code := r.wait(t, 5*time.Second)
	elapsed := time.Since(sent)
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if elapsed < delay {
		t.Errorf("run returned after %v, before the %v delay", elapsed, delay)
	}
	ls := lines(t, r.out)
	errs := withLevel(ls, "ERROR")
	if len(errs) != 1 || errs[0].str("msg") != "shutdown timed out" {
		t.Errorf("want exactly one ERROR line \"shutdown timed out\"; stdout:\n%s", r.out.String())
	}
	if n := len(withMsg(ls, "stopped")); n != 0 {
		t.Errorf("%d stopped line(s), want 0", n)
	}
	// srv.Close() ends the request still held by the handler.
	select {
	case <-res:
	case <-time.After(5 * time.Second):
		t.Error("the held request was not closed after the shutdown timeout")
	}
}

// Matrix row Second signal: no line, no change, same exit code (clean stop).
func TestSecondSignalDuringCleanStop(t *testing.T) {
	h := newBlockingHandler()
	r := start(t, h, 5*time.Second)
	res := inFlight(t, r.addr, h)

	r.sigs <- syscall.SIGTERM
	time.Sleep(100 * time.Millisecond)
	before := len(lines(t, r.out))
	r.sigs <- syscall.SIGINT
	r.sigs <- syscall.SIGTERM
	r.notReturned(t, 300*time.Millisecond)
	if after := len(lines(t, r.out)); after != before {
		t.Errorf("a second signal wrote %d line(s); want none:\n%s", after-before, r.out.String())
	}
	close(h.release)

	if got := <-res; got.err != nil || got.body != "done" {
		t.Errorf("in-flight request: body %q err %v; want \"done\"", got.body, got.err)
	}
	if code := r.wait(t, 5*time.Second); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
	ls := lines(t, r.out)
	if len(ls) != 2 || len(withMsg(ls, "listening")) != 1 || len(withMsg(ls, "stopped")) != 1 {
		t.Errorf("want exactly listening and stopped; stdout:\n%s", r.out.String())
	}
}

// Matrix row Second signal during a slow stop: the delay still runs from the first
// signal, one ERROR line, exit 1.
func TestSecondSignalDuringSlowStop(t *testing.T) {
	h := newBlockingHandler()
	defer close(h.release)
	delay := 600 * time.Millisecond
	r := start(t, h, delay)
	inFlight(t, r.addr, h)

	sent := time.Now()
	r.sigs <- syscall.SIGTERM
	time.Sleep(400 * time.Millisecond)
	r.sigs <- syscall.SIGTERM
	code := r.wait(t, 5*time.Second)
	elapsed := time.Since(sent)
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	// A second signal that restarted the delay would end at 400 ms + 600 ms or later.
	if elapsed >= 400*time.Millisecond+delay {
		t.Errorf("run returned %v after the first signal; the delay must run from the first signal", elapsed)
	}
	ls := lines(t, r.out)
	if errs := withLevel(ls, "ERROR"); len(errs) != 1 || errs[0].str("msg") != "shutdown timed out" {
		t.Errorf("want exactly one ERROR line \"shutdown timed out\"; stdout:\n%s", r.out.String())
	}
	if len(ls) != 2 {
		t.Errorf("%d line(s), want exactly listening and shutdown timed out:\n%s", len(ls), r.out.String())
	}
}

// Matrix row Bind failure.
func TestBindFailure(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()

	out := &syncBuffer{}
	sigs := make(chan os.Signal, 1)
	code := make(chan int, 1)
	go func() {
		code <- run(env(map[string]string{"LISTEN_ADDR": taken.Addr().String()}), out, sigs, http.NotFoundHandler(), time.Second)
	}()
	select {
	case c := <-code:
		if c != 1 {
			t.Errorf("exit code %d, want 1", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("run did not return on a bind failure; stdout:\n%s", out.String())
	}
	ls := lines(t, out)
	if len(ls) != 1 {
		t.Errorf("%d line(s), want exactly one:\n%s", len(ls), out.String())
	}
	errs := withLevel(ls, "ERROR")
	if len(errs) != 1 || errs[0].str("msg") != "listen failed" || errs[0].str("error") == "" {
		t.Errorf("want one ERROR line \"listen failed\" with a field error; stdout:\n%s", out.String())
	}
	if n := len(withMsg(ls, "listening")); n != 0 {
		t.Errorf("%d listening line(s), want 0", n)
	}
}

// http.Server.ErrorLog: net/http's own errors (here a handler panic) are slog JSON at WARN.
func TestNetHTTPErrorsAreWarnJSON(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	r := start(t, h, 2*time.Second)

	if resp, err := http.Get("http://" + r.addr + "/"); err == nil {
		resp.Body.Close()
	}
	deadline := time.Now().Add(5 * time.Second)
	var warn []logLine
	for time.Now().Before(deadline) {
		warn = withLevel(lines(t, r.out), "WARN")
		if len(warn) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(warn) == 0 || !strings.Contains(warn[0].str("msg"), "panic") {
		t.Errorf("want a WARN line from net/http about the panic; stdout:\n%s", r.out.String())
	}

	r.sigs <- syscall.SIGTERM
	if code := r.wait(t, 5*time.Second); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
	if n := len(withLevel(lines(t, r.out), "ERROR")); n != 0 {
		t.Errorf("%d ERROR line(s), want 0", n)
	}
}
