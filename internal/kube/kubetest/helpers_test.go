package kubetest_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

const (
	testToken      = "test-token"
	mergePatchType = "application/merge-patch+json"
)

// newClient builds a real kube.Client aimed at the fake server.
func newClient(t *testing.T, s *kubetest.Server) *kube.Client {
	t.Helper()
	env := map[string]string{
		"KUBERNETES_SERVICE_HOST": s.Host,
		"KUBERNETES_SERVICE_PORT": s.Port,
	}
	c, err := kube.NewInCluster(func(k string) string { return env[k] }, s.CAPEM)
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	return c
}

// rawDo sends a request the client refuses to send (no token, wrong content type).
// An empty token sends no Authorization header; an empty contentType sends no Content-Type.
func rawDo(t *testing.T, s *kubetest.Server, method, path, token, contentType string, body []byte) (int, []byte) {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(s.CAPEM) {
		t.Fatalf("CAPEM holds no certificate")
	}
	hc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "https://"+net.JoinHostPort(s.Host, s.Port)+path, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, b
}

func mustPath(t *testing.T, r kube.Resource, namespace, name string, status bool) string {
	t.Helper()
	p, err := kube.Path(r, namespace, name, status)
	if err != nil {
		t.Fatalf("Path(%v, %q, %q, %v): %v", r, namespace, name, status, err)
	}
	return p
}

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return m
}

// field walks nested objects by key; ok is false when a key is missing.
func field(m map[string]any, keys ...string) (any, bool) {
	var cur any = m
	for _, k := range keys {
		obj, isObj := cur.(map[string]any)
		if !isObj {
			return nil, false
		}
		v, present := obj[k]
		if !present {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func stringField(t *testing.T, m map[string]any, keys ...string) string {
	t.Helper()
	v, ok := field(m, keys...)
	if !ok {
		t.Fatalf("missing %v in %v", keys, m)
	}
	s, isStr := v.(string)
	if !isStr {
		t.Fatalf("%v is %T, want string", keys, v)
	}
	return s
}

func numberField(t *testing.T, m map[string]any, keys ...string) float64 {
	t.Helper()
	v, ok := field(m, keys...)
	if !ok {
		t.Fatalf("missing %v in %v", keys, m)
	}
	f, isNum := v.(float64)
	if !isNum {
		t.Fatalf("%v is %T, want number", keys, v)
	}
	return f
}

func resourceVersion(t *testing.T, raw []byte) int64 {
	t.Helper()
	rv := stringField(t, decode(t, raw), "metadata", "resourceVersion")
	n, err := strconv.ParseInt(rv, 10, 64)
	if err != nil {
		t.Fatalf("resourceVersion %q is not a decimal string: %v", rv, err)
	}
	return n
}

func mustObject(t *testing.T, s *kubetest.Server, r kube.Resource, namespace, name string) map[string]any {
	t.Helper()
	raw, ok := s.Object(r, namespace, name)
	if !ok {
		t.Fatalf("Object(%v, %q, %q): absent", r, namespace, name)
	}
	return decode(t, raw)
}

func apiCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want *kube.APIError")
	}
	var ae *kube.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not a *kube.APIError", err)
	}
	return ae.Code
}

func apiErr(t *testing.T, err error) *kube.APIError {
	t.Helper()
	var ae *kube.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not a *kube.APIError", err)
	}
	return ae
}

func assertStatusBody(t *testing.T, body []byte, code int, reason string) {
	t.Helper()
	m := decode(t, body)
	if got := stringField(t, m, "kind"); got != "Status" {
		t.Errorf("kind = %q, want Status", got)
	}
	if got := stringField(t, m, "status"); got != "Failure" {
		t.Errorf("status = %q, want Failure", got)
	}
	if got := numberField(t, m, "code"); int(got) != code {
		t.Errorf("code = %v, want %d", got, code)
	}
	if got := stringField(t, m, "reason"); got != reason {
		t.Errorf("reason = %q, want %q", got, reason)
	}
}

var ctx = context.Background()

func instance(namespace, name, extra string) json.RawMessage {
	if extra != "" {
		extra = "," + extra
	}
	return json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"namespace":"` +
		namespace + `","name":"` + name + `"}` + extra + `}`)
}
