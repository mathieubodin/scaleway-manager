package main

// Tests of story 6 (CAP-8): run's start sequence, guard, loop, /healthz and shutdown.
// Every helper of this file is prefixed with rt so it cannot clash with the helpers
// of the other test files of package main.

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

const (
	rtToken     = "rt-token-0b7e1c9d"
	rtNamespace = "team-a"
	rtProjectID = "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b"
	rtDelay     = 20 * time.Millisecond
)

var rtT0 = time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)

// rtClock is the injected clock: fixed at rtT0 until a test sets it.
type rtClock struct {
	mu  sync.Mutex
	now time.Time
}

func newRTClock() *rtClock { return &rtClock{now: rtT0} }

func (c *rtClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *rtClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// rtBuffer is the stdout given to run: written by run, read by the test.
type rtBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *rtBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *rtBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type rtLine map[string]any

func (l rtLine) str(key string) string {
	s, _ := l[key].(string)
	return s
}

// rtLines parses every stdout line as one slog JSON object; a line that is not fails the test.
func rtLines(t *testing.T, out *rtBuffer) []rtLine {
	t.Helper()
	raw := out.String()
	if raw == "" {
		return nil
	}
	var parsed []rtLine
	for _, s := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		var l rtLine
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

func rtWithMsg(ls []rtLine, msg string) []rtLine {
	var out []rtLine
	for _, l := range ls {
		if l.str("msg") == msg {
			out = append(out, l)
		}
	}
	return out
}

func rtWithLevel(ls []rtLine, level string) []rtLine {
	var out []rtLine
	for _, l := range ls {
		if l.str("level") == level {
			out = append(out, l)
		}
	}
	return out
}

// rtNoSecret fails when stdout holds a token, an Authorization header or a Kubernetes body.
func rtNoSecret(t *testing.T, out *rtBuffer, tokens ...string) {
	t.Helper()
	s := out.String()
	for _, tok := range tokens {
		if tok != "" && strings.Contains(s, tok) {
			t.Errorf("stdout holds the token %q:\n%s", tok, s)
		}
	}
	for _, bad := range []string{"Bearer", "Authorization", "InternalError", `\"kind\"`} {
		if strings.Contains(s, bad) {
			t.Errorf("stdout holds %q (a header or a body):\n%s", bad, s)
		}
	}
}

func rtEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// rtVars is a valid environment for an API server at host:port.
func rtVars(host, port string) map[string]string {
	return map[string]string{
		"KUBERNETES_SERVICE_HOST":  host,
		"KUBERNETES_SERVICE_PORT":  port,
		"FAKE_OPERATOR_STEP_DELAY": rtDelay.String(),
		"HEALTH_ADDR":              "127.0.0.1:0",
	}
}

// rtWriteToken replaces the token file in one rename, so a pass never reads half a file.
func rtWriteToken(t *testing.T, path, tok string) {
	t.Helper()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(tok+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// rtFiles writes the token file and the CA file and returns their paths.
func rtFiles(t *testing.T, tok string, ca []byte) (tokenPath, caPath string) {
	t.Helper()
	dir := t.TempDir()
	tokenPath = filepath.Join(dir, "token")
	caPath = filepath.Join(dir, "ca.crt")
	rtWriteToken(t, tokenPath, tok)
	if err := os.WriteFile(caPath, ca, 0o600); err != nil {
		t.Fatal(err)
	}
	return tokenPath, caPath
}

type rtRunning struct {
	sigs  chan os.Signal
	out   *rtBuffer
	done  chan struct{}
	exit  int
	addr  string
	clock *rtClock
}

// rtStart runs run in a goroutine; it does not wait for anything.
func rtStart(t *testing.T, vars map[string]string, tokenPath, caPath string, clock *rtClock) *rtRunning {
	t.Helper()
	r := &rtRunning{
		sigs:  make(chan os.Signal, 8),
		out:   &rtBuffer{},
		done:  make(chan struct{}),
		clock: clock,
	}
	getenv := rtEnv(vars)
	go func() {
		defer close(r.done)
		r.exit = run(getenv, r.out, r.sigs, tokenPath, caPath, clock, rand.New(rand.NewPCG(1, 2)))
	}()
	t.Cleanup(func() {
		select {
		case r.sigs <- syscall.SIGTERM:
		default:
		}
	})
	return r
}

func (r *rtRunning) returned() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// waitListening waits for the listening line and keeps its addr.
func (r *rtRunning) waitListening(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if l := rtWithMsg(rtLines(t, r.out), "listening"); len(l) > 0 {
			r.addr = l[0].str("addr")
			if l[0].str("level") != "INFO" {
				t.Errorf("listening level %q, want INFO", l[0].str("level"))
			}
			host, _, err := net.SplitHostPort(r.addr)
			if err != nil || host != "127.0.0.1" {
				t.Fatalf("listening addr %q, want 127.0.0.1:<port>", r.addr)
			}
			return
		}
		if r.returned() {
			t.Fatalf("run returned %d before listening; stdout:\n%s", r.exit, r.out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no listening line within 5 s; stdout:\n%s", r.out.String())
}

func (r *rtRunning) wait(t *testing.T, within time.Duration) int {
	t.Helper()
	select {
	case <-r.done:
		return r.exit
	case <-time.After(within):
		t.Fatalf("run did not return within %v; stdout:\n%s", within, r.out.String())
		return -1
	}
}

func (r *rtRunning) notReturned(t *testing.T, during time.Duration) {
	t.Helper()
	select {
	case <-r.done:
		t.Fatalf("run returned %d too early; stdout:\n%s", r.exit, r.out.String())
	case <-time.After(during):
	}
}

// stop sends SIGTERM and expects exit 0 with one INFO stopped line.
func (r *rtRunning) stop(t *testing.T) {
	t.Helper()
	r.sigs <- syscall.SIGTERM
	if code := r.wait(t, 5*time.Second); code != 0 {
		t.Errorf("exit code %d after SIGTERM, want 0; stdout:\n%s", code, r.out.String())
	}
	stopped := rtWithMsg(rtLines(t, r.out), "stopped")
	if len(stopped) != 1 || stopped[0].str("level") != "INFO" {
		t.Errorf("want one INFO stopped line; stdout:\n%s", r.out.String())
	}
}

// health sends method /path to the health server; status -1 on a transport error.
func (r *rtRunning) health(t *testing.T, method, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+r.addr+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return -1, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (r *rtRunning) healthz(t *testing.T) (int, string) {
	t.Helper()
	return r.health(t, http.MethodGet, "/healthz")
}

func rtEventually(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("not reached within %v: %s", within, what)
}

// rtStartFails runs run to its return and checks the start-up failure shape:
// exit 1, exactly one ERROR line, no listening line.
func rtStartFails(t *testing.T, vars map[string]string, tokenPath, caPath string) []rtLine {
	t.Helper()
	r := rtStart(t, vars, tokenPath, caPath, newRTClock())
	if code := r.wait(t, 5*time.Second); code != 1 {
		t.Errorf("exit code %d, want 1; stdout:\n%s", code, r.out.String())
	}
	ls := rtLines(t, r.out)
	errs := rtWithLevel(ls, "ERROR")
	if len(errs) != 1 {
		t.Errorf("%d ERROR line(s), want exactly 1; stdout:\n%s", len(errs), r.out.String())
	}
	if n := len(rtWithMsg(ls, "listening")); n != 0 {
		t.Errorf("%d listening line(s), want 0", n)
	}
	rtNoSecret(t, r.out, rtToken)
	return errs
}

func rtPath(t *testing.T, r kube.Resource, namespace, name string, status bool) string {
	t.Helper()
	p, err := kube.Path(r, namespace, name, status)
	if err != nil {
		t.Fatalf("kube.Path(%v, %q, %q, %v): %v", r, namespace, name, status, err)
	}
	return p
}

func rtReqPath(req kubetest.Request) string {
	p, _, _ := strings.Cut(req.RequestURI, "?")
	return p
}

// rtCount counts the kubetest requests with method (any when "") whose path has prefix.
func rtCount(s *kubetest.Server, method, prefix string) int {
	n := 0
	for _, req := range s.Requests() {
		if (method == "" || req.Method == method) && strings.HasPrefix(rtReqPath(req), prefix) {
			n++
		}
	}
	return n
}

// rtKube starts kubetest expecting rtToken, with the token and CA files.
func rtKube(t *testing.T) (s *kubetest.Server, vars map[string]string, tokenPath, caPath string) {
	t.Helper()
	s = kubetest.New(t)
	s.ExpectToken(rtToken)
	tokenPath, caPath = rtFiles(t, rtToken, s.CAPEM)
	return s, rtVars(s.Host, s.Port), tokenPath, caPath
}

func rtDeployment(namespace, name, appName string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":%q,"namespace":%q,"labels":{"app.kubernetes.io/name":%q}},"spec":{}}`,
		name, namespace, appName))
}

// rtGate holds a request until opened; opening twice is harmless.
type rtGate struct {
	ch   chan struct{}
	once sync.Once
}

func newRTGate() *rtGate { return &rtGate{ch: make(chan struct{})} }

func (g *rtGate) open() { g.once.Do(func() { close(g.ch) }) }

// rtAPI is a TLS API server answering empty lists, which can hold the guard's
// deployments list and the instances list of chosen passes. The instances list
// across namespaces is the first request of a pass.
type rtAPI struct {
	srv  *httptest.Server
	host string
	port string
	ca   []byte

	mu            sync.Mutex
	passes        int
	starts        []time.Time
	tokens        []string
	canceled      []bool // per held pass: the request context was done when released
	hold          map[int]*rtGate
	holdDeploy    *rtGate
	sleep         time.Duration
	entered       chan int
	deployEntered chan struct{}
}

func newRTAPI(t *testing.T) *rtAPI {
	t.Helper()
	a := &rtAPI{
		hold:          map[int]*rtGate{},
		entered:       make(chan int, 1024),
		deployEntered: make(chan struct{}, 16),
	}
	a.srv = httptest.NewTLSServer(a)
	t.Cleanup(func() {
		a.mu.Lock()
		gates := []*rtGate{a.holdDeploy}
		for _, g := range a.hold {
			gates = append(gates, g)
		}
		a.mu.Unlock()
		for _, g := range gates {
			if g != nil {
				g.open()
			}
		}
		a.srv.Close()
	})
	u, err := url.Parse(a.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	a.host, a.port, err = net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	a.ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.srv.Certificate().Raw})
	return a
}

// holdPass holds the instances list of pass n until the returned gate opens.
func (a *rtAPI) holdPass(n int) *rtGate {
	g := newRTGate()
	a.mu.Lock()
	a.hold[n] = g
	a.mu.Unlock()
	return g
}

func (a *rtAPI) holdGuard() *rtGate {
	g := newRTGate()
	a.mu.Lock()
	a.holdDeploy = g
	a.mu.Unlock()
	return g
}

func (a *rtAPI) passCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.passes
}

func (a *rtAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	a.tokens = append(a.tokens, strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
	a.mu.Unlock()

	res, ns, name, status, err := kube.ParsePath(req.URL.Path)
	if err != nil || req.Method != http.MethodGet || name != "" || status {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404,"reason":"NotFound"}`)
		return
	}
	switch {
	case res == kube.Deployments:
		a.mu.Lock()
		g := a.holdDeploy
		a.mu.Unlock()
		if g != nil {
			a.deployEntered <- struct{}{}
			<-g.ch
		}
	case res == kube.Instances && ns == "":
		a.mu.Lock()
		a.passes++
		n := a.passes
		a.starts = append(a.starts, time.Now())
		g := a.hold[n]
		sleep := a.sleep
		a.mu.Unlock()
		select {
		case a.entered <- n:
		default:
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
		if g != nil {
			<-g.ch
			a.mu.Lock()
			a.canceled = append(a.canceled, req.Context().Err() != nil)
			a.mu.Unlock()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"apiVersion":"v1","kind":%q,"metadata":{"resourceVersion":"1"},"items":[]}`, res.Kind()+"List")
}

func (a *rtAPI) start(t *testing.T, vars map[string]string, clock *rtClock) *rtRunning {
	t.Helper()
	tokenPath, caPath := rtFiles(t, rtToken, a.ca)
	return rtStart(t, vars, tokenPath, caPath, clock)
}

// waitPass waits until the instances list of pass n has arrived.
func (a *rtAPI) waitPass(t *testing.T, n int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-a.entered:
			if got >= n {
				return
			}
		case <-deadline:
			t.Fatalf("pass %d did not start within 5 s (passes: %d)", n, a.passCount())
		}
	}
}

// Matrix rows No host and No port: unset, then empty, each exits 1 with one ERROR line.
func TestRunMissingAPIServer(t *testing.T) {
	s := kubetest.New(t)
	tokenPath, caPath := rtFiles(t, rtToken, s.CAPEM)
	cases := []struct {
		name string
		key  string
		del  bool
	}{
		{"host unset", "KUBERNETES_SERVICE_HOST", true},
		{"host empty", "KUBERNETES_SERVICE_HOST", false},
		{"port unset", "KUBERNETES_SERVICE_PORT", true},
		{"port empty", "KUBERNETES_SERVICE_PORT", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vars := rtVars(s.Host, s.Port)
			if tc.del {
				delete(vars, tc.key)
			} else {
				vars[tc.key] = ""
			}
			errs := rtStartFails(t, vars, tokenPath, caPath)
			if len(errs) == 1 && (errs[0].str("msg") != "start failed" || errs[0].str("error") == "") {
				t.Errorf("ERROR line %v, want msg \"start failed\" with a non-empty error", errs[0])
			}
		})
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d request(s) reached the API server, want 0", n)
	}
}

// Start step 2: an unreadable CA file exits 1 with one ERROR line.
func TestRunUnreadableCA(t *testing.T) {
	s, vars, tokenPath, _ := rtKube(t)
	missing := filepath.Join(t.TempDir(), "absent-ca.crt")
	errs := rtStartFails(t, vars, tokenPath, missing)
	if len(errs) == 1 && (errs[0].str("msg") != "start failed" || errs[0].str("error") == "") {
		t.Errorf("ERROR line %v, want msg \"start failed\" with a non-empty error", errs[0])
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d request(s) reached the API server, want 0", n)
	}
}

// Matrix row Bad delay: an invalid or non-positive FAKE_OPERATOR_STEP_DELAY exits 1
// with one ERROR line, before the guard sends any request.
func TestRunBadStepDelay(t *testing.T) {
	for _, v := range []string{"x", "0s", "-1s", "0"} {
		t.Run(v, func(t *testing.T) {
			s, vars, tokenPath, caPath := rtKube(t)
			vars["FAKE_OPERATOR_STEP_DELAY"] = v
			errs := rtStartFails(t, vars, tokenPath, caPath)
			if len(errs) == 1 && (errs[0].str("msg") != "start failed" || errs[0].str("error") == "") {
				t.Errorf("ERROR line %v, want msg \"start failed\" with a non-empty error", errs[0])
			}
			if n := len(s.Requests()); n != 0 {
				t.Errorf("%d request(s) reached the API server, want 0", n)
			}
		})
	}
}

// Start step 3: FAKE_OPERATOR_STEP_DELAY unset or empty is 2s, seen through the
// /healthz threshold of 3 × 2s on the injected clock.
func TestRunDefaultStepDelay(t *testing.T) {
	for _, tc := range []struct {
		name string
		del  bool
	}{{"unset", true}, {"empty", false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := newRTAPI(t)
			second := a.holdPass(2)
			vars := rtVars(a.host, a.port)
			if tc.del {
				delete(vars, "FAKE_OPERATOR_STEP_DELAY")
			} else {
				vars["FAKE_OPERATOR_STEP_DELAY"] = ""
			}
			clock := newRTClock()
			r := a.start(t, vars, clock)
			r.waitListening(t)
			rtEventually(t, 5*time.Second, "/healthz 200 after the first pass", func() bool {
				code, _ := r.healthz(t)
				return code == http.StatusOK
			})

			clock.set(rtT0.Add(6*time.Second - time.Nanosecond))
			if code, body := r.healthz(t); code != http.StatusOK || body != "ok" {
				t.Errorf("at 6s - 1ns: /healthz %d %q, want 200 \"ok\" (default delay 2s)", code, body)
			}
			clock.set(rtT0.Add(6 * time.Second))
			if code, _ := r.healthz(t); code != http.StatusServiceUnavailable {
				t.Errorf("at 6s: /healthz %d, want 503 (default delay 2s)", code)
			}

			r.sigs <- syscall.SIGTERM
			second.open()
			if code := r.wait(t, 5*time.Second); code != 0 {
				t.Errorf("exit code %d, want 0", code)
			}
		})
	}
}

// Matrix row Real operator: a Deployment labelled app.kubernetes.io/name: scaleway-operator
// in scaleway-system exits 1 with one ERROR line naming it, before any pass.
func TestRunRealOperatorGuard(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	s.Put(kube.Deployments, rtDeployment("scaleway-system", "aaa-other", "something-else"))
	s.Put(kube.Deployments, rtDeployment("scaleway-system", "scaleway-operator", "scaleway-operator"))

	errs := rtStartFails(t, vars, tokenPath, caPath)
	if len(errs) == 1 {
		if errs[0].str("msg") != "real operator found" {
			t.Errorf("ERROR msg %q, want \"real operator found\"", errs[0].str("msg"))
		}
		if got := errs[0].str("deployment"); got != "scaleway-system/scaleway-operator" {
			t.Errorf("ERROR deployment %q, want \"scaleway-system/scaleway-operator\"", got)
		}
	}
	deployments := rtPath(t, kube.Deployments, "scaleway-system", "", false)
	for _, req := range s.Requests() {
		if req.Method != http.MethodGet || rtReqPath(req) != deployments {
			t.Errorf("request %s %s, want only GET %s before the exit", req.Method, req.RequestURI, deployments)
		}
	}
	if n := rtCount(s, "", deployments); n != 1 {
		t.Errorf("%d deployments list(s), want 1", n)
	}
}

// Matrix row Other deployment: other labels in scaleway-system, or the operator's label
// in another namespace, let the loop start.
func TestRunOtherDeployment(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	s.Put(kube.Deployments, rtDeployment("scaleway-system", "scaleway-operator", "not-the-operator"))
	s.Put(kube.Deployments, rtDeployment(rtNamespace, "scaleway-operator", "scaleway-operator"))

	r := rtStart(t, vars, tokenPath, caPath, newRTClock())
	r.waitListening(t)
	instances := rtPath(t, kube.Instances, "", "", false)
	rtEventually(t, 5*time.Second, "a pass lists instances", func() bool {
		return rtCount(s, http.MethodGet, instances) > 0
	})
	r.stop(t)
	if n := len(rtWithMsg(rtLines(t, r.out), "real operator found")); n != 0 {
		t.Errorf("%d real operator found line(s), want 0", n)
	}
	rtNoSecret(t, r.out, rtToken)
}

// Start step 4: a failed guard list exits 1 with one ERROR line.
func TestRunGuardListError(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	deployments := rtPath(t, kube.Deployments, "scaleway-system", "", false)
	s.Fail(http.MethodGet, deployments, http.StatusInternalServerError)

	rtStartFails(t, vars, tokenPath, caPath)
	if n := rtCount(s, http.MethodGet, rtPath(t, kube.Instances, "", "", false)); n != 0 {
		t.Errorf("%d instances list(s) after a failed guard, want 0", n)
	}
}

// Start step 4: the guard reads the token first; an unreadable token exits 1 with one
// ERROR line and sends nothing.
func TestRunGuardUnreadableToken(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content *string
	}{{"absent", nil}, {"empty", new(string)}} {
		t.Run(tc.name, func(t *testing.T) {
			s, vars, tokenPath, caPath := rtKube(t)
			if err := os.Remove(tokenPath); err != nil {
				t.Fatal(err)
			}
			if tc.content != nil {
				if err := os.WriteFile(tokenPath, []byte(*tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			rtStartFails(t, vars, tokenPath, caPath)
			if n := len(s.Requests()); n != 0 {
				t.Errorf("%d request(s) reached the API server, want 0", n)
			}
		})
	}
}

// Start step 4: the guard runs at start only; a real operator appearing later does
// not stop the loop.
func TestRunGuardAtStartOnly(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	r := rtStart(t, vars, tokenPath, caPath, newRTClock())
	r.waitListening(t)
	instances := rtPath(t, kube.Instances, "", "", false)
	rtEventually(t, 5*time.Second, "a first pass", func() bool { return rtCount(s, http.MethodGet, instances) > 0 })

	s.Put(kube.Deployments, rtDeployment("scaleway-system", "scaleway-operator", "scaleway-operator"))
	after := rtCount(s, http.MethodGet, instances)
	rtEventually(t, 5*time.Second, "three more passes", func() bool { return rtCount(s, http.MethodGet, instances) >= after+3 })
	if r.returned() {
		t.Fatalf("run returned %d after a real operator appeared; stdout:\n%s", r.exit, r.out.String())
	}
	if n := rtCount(s, "", rtPath(t, kube.Deployments, "scaleway-system", "", false)); n != 1 {
		t.Errorf("%d deployments list(s), want 1 (the guard at start only)", n)
	}
	r.stop(t)
}

// Start step 5: a bind failure of HEALTH_ADDR exits 1 with one ERROR line.
func TestRunHealthBindFailure(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	s, vars, tokenPath, caPath := rtKube(t)
	vars["HEALTH_ADDR"] = taken.Addr().String()

	errs := rtStartFails(t, vars, tokenPath, caPath)
	if len(errs) == 1 && (errs[0].str("msg") != "start failed" || errs[0].str("error") == "") {
		t.Errorf("ERROR line %v, want msg \"start failed\" with a non-empty error", errs[0])
	}
	if n := rtCount(s, http.MethodGet, rtPath(t, kube.Instances, "", "", false)); n != 0 {
		t.Errorf("%d instances list(s) after a bind failure, want 0", n)
	}
}

// Matrix row Health before: /healthz answers 503 with an empty body until the first
// pass ends, then 200 ok.
func TestRunHealthBeforeFirstPass(t *testing.T) {
	a := newRTAPI(t)
	first := a.holdPass(1)
	r := a.start(t, rtVars(a.host, a.port), newRTClock())
	r.waitListening(t)
	a.waitPass(t, 1)

	if code, body := r.healthz(t); code != http.StatusServiceUnavailable || body != "" {
		t.Errorf("during the first pass: /healthz %d %q, want 503 with an empty body", code, body)
	}
	first.open()
	rtEventually(t, 5*time.Second, "/healthz 200 ok after the first pass", func() bool {
		code, body := r.healthz(t)
		return code == http.StatusOK && body == "ok"
	})
	r.stop(t)
	rtNoSecret(t, r.out, rtToken)
}

// Matrix rows Health fresh and Health stale: on the injected clock, 200 ok while the
// last pass ended less than 3 × the delay ago, 503 at 3 × the delay and beyond.
func TestRunHealthFreshAndStale(t *testing.T) {
	a := newRTAPI(t)
	second := a.holdPass(2)
	clock := newRTClock()
	r := a.start(t, rtVars(a.host, a.port), clock)
	r.waitListening(t)
	a.waitPass(t, 2) // pass 1 ended at rtT0; pass 2 is held

	if code, body := r.healthz(t); code != http.StatusOK || body != "ok" {
		t.Errorf("fresh: /healthz %d %q, want 200 \"ok\"", code, body)
	}
	clock.set(rtT0.Add(3*rtDelay - time.Nanosecond))
	if code, body := r.healthz(t); code != http.StatusOK || body != "ok" {
		t.Errorf("at 3 × delay - 1ns: /healthz %d %q, want 200 \"ok\"", code, body)
	}
	clock.set(rtT0.Add(3 * rtDelay))
	if code, body := r.healthz(t); code != http.StatusServiceUnavailable || body != "" {
		t.Errorf("at 3 × delay: /healthz %d %q, want 503 with an empty body", code, body)
	}
	later := rtT0.Add(time.Hour)
	clock.set(later)
	if code, _ := r.healthz(t); code != http.StatusServiceUnavailable {
		t.Errorf("an hour later: /healthz %d, want 503", code)
	}

	// The end of pass 2 is read from the injected clock, now an hour later.
	second.open()
	rtEventually(t, 5*time.Second, "/healthz 200 once pass 2 ends", func() bool {
		code, _ := r.healthz(t)
		return code == http.StatusOK
	})
	r.stop(t)
}

// GET /healthz only: another method gets 405, another path 404.
func TestRunHealthRoutes(t *testing.T) {
	a := newRTAPI(t)
	r := a.start(t, rtVars(a.host, a.port), newRTClock())
	r.waitListening(t)
	if code, _ := r.health(t, http.MethodPost, "/healthz"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST /healthz: %d, want 405", code)
	}
	if code, _ := r.health(t, http.MethodGet, "/other"); code != http.StatusNotFound {
		t.Errorf("GET /other: %d, want 404", code)
	}
	r.stop(t)
}

// Loop: a pass that fails before listing (unreadable token) is logged, does not count
// as ended, and the loop goes on; the next readable token resumes the passes.
func TestRunPassErrorBeforeListing(t *testing.T) {
	a := newRTAPI(t)
	guard := a.holdGuard()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	caPath := filepath.Join(dir, "ca.crt")
	rtWriteToken(t, tokenPath, rtToken)
	if err := os.WriteFile(caPath, a.ca, 0o600); err != nil {
		t.Fatal(err)
	}
	r := rtStart(t, rtVars(a.host, a.port), tokenPath, caPath, newRTClock())

	select {
	case <-a.deployEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the guard did not list deployments within 5 s")
	}
	if err := os.Remove(tokenPath); err != nil {
		t.Fatal(err)
	}
	guard.open()
	r.waitListening(t)

	rtEventually(t, 5*time.Second, "a pass error logged", func() bool {
		return len(rtLines(t, r.out)) > 1
	})
	time.Sleep(10 * rtDelay)
	if code, _ := r.healthz(t); code != http.StatusServiceUnavailable {
		t.Errorf("after passes that failed before listing: /healthz %d, want 503", code)
	}
	if n := a.passCount(); n != 0 {
		t.Errorf("%d instances list(s) without a token, want 0", n)
	}
	if r.returned() {
		t.Fatalf("run returned %d on a pass error; stdout:\n%s", r.exit, r.out.String())
	}

	const next = "rt-token-second-4c21"
	rtWriteToken(t, tokenPath, next)
	rtEventually(t, 5*time.Second, "/healthz 200 once the token is back", func() bool {
		code, _ := r.healthz(t)
		return code == http.StatusOK
	})
	r.stop(t)
	rtNoSecret(t, r.out, rtToken, next)
}

// Matrix row Token rotation: each pass rereads the token file; after a rewrite, the
// next pass sends the new token and no later request carries the old one.
func TestRunTokenRotation(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	r := rtStart(t, vars, tokenPath, caPath, newRTClock())
	r.waitListening(t)
	instances := rtPath(t, kube.Instances, "", "", false)
	withToken := func(tok string) int {
		n := 0
		for _, req := range s.Requests() {
			if req.Method == http.MethodGet && rtReqPath(req) == instances && req.Token == tok {
				n++
			}
		}
		return n
	}
	rtEventually(t, 5*time.Second, "a pass with the first token", func() bool { return withToken(rtToken) > 0 })

	const rotated = "rt-token-rotated-93ad"
	rtWriteToken(t, tokenPath, rotated)
	s.ExpectToken(rotated)
	rtEventually(t, 5*time.Second, "a pass with the rotated token", func() bool { return withToken(rotated) >= 2 })

	r.stop(t)
	reqs := s.Requests()
	seen := false
	for _, req := range reqs {
		switch req.Token {
		case rotated:
			seen = true
		case rtToken:
			if seen {
				t.Errorf("request %s %s carries the old token after the rotated one", req.Method, req.RequestURI)
			}
		default:
			t.Errorf("request %s %s carries token %q, want one of the two written", req.Method, req.RequestURI, req.Token)
		}
	}
	rtNoSecret(t, r.out, rtToken, rotated)
}

// Matrix row Failed list: kubetest fails the instances list; the other kinds are
// still patched, the failure is logged, and the pass counts as ended.
func TestRunFailedListOfOneKind(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	s.Put(kube.Namespaces, json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":%q,"annotations":{"scaleway.mathieubodin.io/project-id":%q}}}`,
		rtNamespace, rtProjectID)))
	s.Put(kube.NamespaceRoles, json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"NamespaceRole","metadata":{"name":%q},"spec":{"scaleway_role":"Editor"}}`,
		rtNamespace)))
	s.Put(kube.Instances, json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"vm-1","namespace":%q},"spec":{"name":"vm-1","zone":"fr-par-1"}}`,
		rtNamespace)))
	s.Put(kube.LoadBalancers, json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"LoadBalancer","metadata":{"name":"lb-1","namespace":%q},"spec":{"name":"lb-1","zone":"fr-par-1","lb_type":"LB-S"}}`,
		rtNamespace)))
	s.Put(kube.ScalewaySecrets, json.RawMessage(fmt.Sprintf(
		`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"sec-1","namespace":%q},"spec":{"region":"fr-par","source":{"kubernetes_secret":{"name":"src","key":"k"}}}}`,
		rtNamespace)))
	s.Fail(http.MethodGet, rtPath(t, kube.Instances, "", "", false), http.StatusInternalServerError)

	clock := newRTClock()
	r := rtStart(t, vars, tokenPath, caPath, clock)
	r.waitListening(t)

	lb := rtPath(t, kube.LoadBalancers, rtNamespace, "lb-1", false)
	sec := rtPath(t, kube.ScalewaySecrets, rtNamespace, "sec-1", false)
	rtEventually(t, 5*time.Second, "the LoadBalancer and the ScalewaySecret patched", func() bool {
		return rtCount(s, http.MethodPatch, lb) > 0 && rtCount(s, http.MethodPatch, sec) > 0
	})
	if code, body := r.healthz(t); code != http.StatusOK || body != "ok" {
		t.Errorf("after passes with a failed instances list: /healthz %d %q, want 200 \"ok\"", code, body)
	}
	if n := rtCount(s, http.MethodPatch, rtPath(t, kube.Instances, rtNamespace, "vm-1", false)); n != 0 {
		t.Errorf("%d PATCH(es) on the Instance whose list fails, want 0", n)
	}
	if obj, ok := s.Object(kube.Instances, rtNamespace, "vm-1"); !ok || strings.Contains(string(obj), "finalizer") {
		t.Errorf("Instance changed while its list fails: %s", obj)
	}
	r.stop(t)
	if n := len(rtWithMsg(rtLines(t, r.out), "list failed")); n == 0 {
		t.Errorf("no list failed line; stdout:\n%s", r.out.String())
	}
	rtNoSecret(t, r.out, rtToken)
}

// Loop: the first pass starts at once, and a signal during the wait between passes
// stops run without waiting for the delay.
func TestRunFirstPassAtOnceAndStopDuringWait(t *testing.T) {
	a := newRTAPI(t)
	vars := rtVars(a.host, a.port)
	vars["FAKE_OPERATOR_STEP_DELAY"] = "10s"
	r := a.start(t, vars, newRTClock())
	r.waitListening(t)
	listened := time.Now()
	a.waitPass(t, 1)
	if d := time.Since(listened); d > time.Second {
		t.Errorf("first pass started %v after listening, want at once (delay 10s)", d)
	}
	rtEventually(t, 5*time.Second, "the first pass ends", func() bool {
		code, _ := r.healthz(t)
		return code == http.StatusOK
	})

	sent := time.Now()
	r.stop(t)
	if d := time.Since(sent); d > 2*time.Second {
		t.Errorf("run returned %v after SIGTERM during the 10s wait, want at once", d)
	}
	if n := a.passCount(); n != 1 {
		t.Errorf("%d passes, want 1", n)
	}
}

// Loop: the next pass starts one delay after the previous one ends (not at a fixed rate).
func TestRunNextPassOneDelayAfterEnd(t *testing.T) {
	a := newRTAPI(t)
	const passTime = 150 * time.Millisecond
	const delay = 100 * time.Millisecond
	a.sleep = passTime
	vars := rtVars(a.host, a.port)
	vars["FAKE_OPERATOR_STEP_DELAY"] = delay.String()
	r := a.start(t, vars, newRTClock())
	r.waitListening(t)
	a.waitPass(t, 4)
	r.stop(t)

	a.mu.Lock()
	starts := append([]time.Time(nil), a.starts...)
	a.mu.Unlock()
	for i := 1; i < 4; i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < passTime+delay {
			t.Errorf("pass %d started %v after pass %d, want at least %v (pass time + delay)", i+1, gap, i, passTime+delay)
		}
	}
}

// Matrix row SIGTERM (and SIGINT): the in-flight request finishes uncanceled, then the
// health server stops, INFO stopped, exit 0, and no new pass starts.
func TestRunSignalWaitsForInFlightPass(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			a := newRTAPI(t)
			first := a.holdPass(1)
			r := a.start(t, rtVars(a.host, a.port), newRTClock())
			r.waitListening(t)
			a.waitPass(t, 1)

			r.sigs <- sig
			r.notReturned(t, 300*time.Millisecond)
			if n := len(rtWithMsg(rtLines(t, r.out), "stopped")); n != 0 {
				t.Errorf("stopped written while a pass is in flight")
			}
			first.open()
			if code := r.wait(t, 5*time.Second); code != 0 {
				t.Errorf("exit code %d, want 0", code)
			}

			ls := rtLines(t, r.out)
			stopped := rtWithMsg(ls, "stopped")
			if len(stopped) != 1 || stopped[0].str("level") != "INFO" {
				t.Errorf("want one INFO stopped line; stdout:\n%s", r.out.String())
			}
			if n := len(rtWithLevel(ls, "ERROR")); n != 0 {
				t.Errorf("%d ERROR line(s), want 0; stdout:\n%s", n, r.out.String())
			}
			a.mu.Lock()
			canceled := append([]bool(nil), a.canceled...)
			a.mu.Unlock()
			if len(canceled) != 1 || canceled[0] {
				t.Errorf("held request canceled = %v, want one request finished uncanceled", canceled)
			}
			time.Sleep(5 * rtDelay)
			if n := a.passCount(); n != 1 {
				t.Errorf("%d passes, want 1 (no pass after the signal)", n)
			}
			if code, _ := r.healthz(t); code != -1 {
				t.Errorf("/healthz answered %d after the stop, want the health server shut down", code)
			}
			rtNoSecret(t, r.out, rtToken)
		})
	}
}

// SIGTERM: a second signal is ignored; run still waits for the in-flight pass and
// writes one stopped line.
func TestRunSecondSignalIgnored(t *testing.T) {
	a := newRTAPI(t)
	first := a.holdPass(1)
	r := a.start(t, rtVars(a.host, a.port), newRTClock())
	r.waitListening(t)
	a.waitPass(t, 1)

	r.sigs <- syscall.SIGTERM
	time.Sleep(50 * time.Millisecond)
	r.sigs <- syscall.SIGINT
	r.sigs <- syscall.SIGTERM
	r.notReturned(t, 300*time.Millisecond)
	first.open()
	if code := r.wait(t, 5*time.Second); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
	if n := len(rtWithMsg(rtLines(t, r.out), "stopped")); n != 1 {
		t.Errorf("%d stopped line(s), want 1; stdout:\n%s", n, r.out.String())
	}
}

// Matrix row SIGTERM: during the loop against kubetest, INFO stopped and exit 0, with
// no ERROR line and no secret in the logs.
func TestRunStopsOnSIGTERM(t *testing.T) {
	s, vars, tokenPath, caPath := rtKube(t)
	r := rtStart(t, vars, tokenPath, caPath, newRTClock())
	r.waitListening(t)
	instances := rtPath(t, kube.Instances, "", "", false)
	rtEventually(t, 5*time.Second, "two passes", func() bool { return rtCount(s, http.MethodGet, instances) >= 2 })
	r.stop(t)
	if n := len(rtWithLevel(rtLines(t, r.out), "ERROR")); n != 0 {
		t.Errorf("%d ERROR line(s), want 0; stdout:\n%s", n, r.out.String())
	}
	rtNoSecret(t, r.out, rtToken)
}
