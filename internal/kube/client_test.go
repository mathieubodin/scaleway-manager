package kube_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// recorded is one request as the test server's handler saw it.
type recorded struct {
	Method        string
	RequestURI    string
	Authorization []string
	Accept        []string
	ContentType   []string
	Body          []byte
}

// apiServer is an httptest TLS server that records requests and answers
// with a fixed status and body.
type apiServer struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []recorded
	code int
	body []byte
}

const defaultBody = `{"kind":"X","metadata":{"name":"n"},"items":[{"metadata":{"name":"a"}},{"metadata":{"name":"b"}}]}`

func newAPIServer(t *testing.T) *apiServer {
	t.Helper()
	s := &apiServer{code: http.StatusOK, body: []byte(defaultBody)}
	s.srv = httptest.NewTLSServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *apiServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.reqs = append(s.reqs, recorded{
		Method:        r.Method,
		RequestURI:    r.RequestURI,
		Authorization: r.Header.Values("Authorization"),
		Accept:        r.Header.Values("Accept"),
		ContentType:   r.Header.Values("Content-Type"),
		Body:          body,
	})
	code, resp := s.code, s.body
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(resp)
}

func (s *apiServer) answer(code int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.code, s.body = code, body
}

func (s *apiServer) requests() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.reqs...)
}

func (s *apiServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = nil
}

func certPEM(c *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}

func envFor(host, port string) func(string) string {
	env := map[string]string{
		"KUBERNETES_SERVICE_HOST": host,
		"KUBERNETES_SERVICE_PORT": port,
	}
	return func(k string) string { return env[k] }
}

func hostPort(t *testing.T, srv *httptest.Server) (string, string) {
	t.Helper()
	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", srv.Listener.Addr(), err)
	}
	return host, port
}

func (s *apiServer) client(t *testing.T) *kube.Client {
	t.Helper()
	host, port := hostPort(t, s.srv)
	c, err := kube.NewInCluster(envFor(host, port), certPEM(s.srv.Certificate()))
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	return c
}

const tok = kube.Token("tok-123")

func ctx() context.Context { return context.Background() }

func compact(t *testing.T, b []byte) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		t.Fatalf("compact %q: %v", b, err)
	}
	return buf.String()
}

// Matrix row List across namespaces: GET on the all-namespaces path, items returned.
func TestListAcrossNamespaces(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)

	items, err := c.List(ctx(), tok, kube.Instances, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	reqs := s.requests()
	if len(reqs) != 1 {
		t.Fatalf("%d requests, want 1", len(reqs))
	}
	if reqs[0].Method != http.MethodGet || reqs[0].RequestURI != scwGroup+"/instances" {
		t.Errorf("request %s %s, want GET %s", reqs[0].Method, reqs[0].RequestURI, scwGroup+"/instances")
	}
	if len(items) != 2 {
		t.Fatalf("List returned %d items, want 2", len(items))
	}
	if got := compact(t, items[0]); got != `{"metadata":{"name":"a"}}` {
		t.Errorf("item 0 = %s", got)
	}
	if got := compact(t, items[1]); got != `{"metadata":{"name":"b"}}` {
		t.Errorf("item 1 = %s", got)
	}
}

// List on a namespace, and empty or null items.
func TestListNamespacedAndEmpty(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)

	for _, body := range []string{`{"items":[]}`, `{"items":null}`} {
		s.reset()
		s.answer(http.StatusOK, []byte(body))
		items, err := c.List(ctx(), tok, kube.Deployments, "scaleway-system")
		if err != nil {
			t.Fatalf("List with %s: %v", body, err)
		}
		if items == nil || len(items) != 0 {
			t.Errorf("List with %s = %#v, want empty non-nil slice", body, items)
		}
		reqs := s.requests()
		if len(reqs) != 1 || reqs[0].RequestURI != "/apis/apps/v1/namespaces/scaleway-system/deployments" {
			t.Errorf("requests %+v, want one GET of the deployments of scaleway-system", reqs)
		}
	}
}

// Get returns the body as raw JSON.
func TestGetReturnsRawBody(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)
	s.answer(http.StatusOK, []byte(`{"metadata":{"name":"n","namespace":"ns"},"spec":{"x":1}}`))

	obj, err := c.Get(ctx(), tok, kube.Instances, "ns", "n")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := compact(t, obj); got != `{"metadata":{"name":"n","namespace":"ns"},"spec":{"x":1}}` {
		t.Errorf("Get = %s", got)
	}
	reqs := s.requests()
	if len(reqs) != 1 || reqs[0].Method != http.MethodGet || reqs[0].RequestURI != scwGroup+"/namespaces/ns/instances/n" {
		t.Errorf("requests %+v", reqs)
	}
}

// Matrix row Headers: Bearer on all four; merge-patch Content-Type on the patches only;
// patches sent unchanged; no query parameter.
func TestRequestHeadersAndBodies(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)
	patch := []byte(`{"spec":{"x":2},"metadata":{"resourceVersion":"7"}}`)
	statusPatch := []byte(`not even json`)

	if _, err := c.List(ctx(), tok, kube.LoadBalancers, "ns"); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := c.Get(ctx(), tok, kube.LoadBalancers, "ns", "n"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := c.MergePatch(ctx(), tok, kube.LoadBalancers, "ns", "n", patch); err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	if _, err := c.PatchStatus(ctx(), tok, kube.LoadBalancers, "ns", "n", statusPatch); err != nil {
		t.Fatalf("PatchStatus: %v", err)
	}

	want := []struct {
		method string
		uri    string
		ctype  bool
		body   []byte
	}{
		{http.MethodGet, scwGroup + "/namespaces/ns/loadbalancers", false, nil},
		{http.MethodGet, scwGroup + "/namespaces/ns/loadbalancers/n", false, nil},
		{http.MethodPatch, scwGroup + "/namespaces/ns/loadbalancers/n", true, patch},
		{http.MethodPatch, scwGroup + "/namespaces/ns/loadbalancers/n/status", true, statusPatch},
	}
	reqs := s.requests()
	if len(reqs) != len(want) {
		t.Fatalf("%d requests, want %d: %+v", len(reqs), len(want), reqs)
	}
	for i, w := range want {
		r := reqs[i]
		if r.Method != w.method || r.RequestURI != w.uri {
			t.Errorf("request %d: %s %s, want %s %s", i, r.Method, r.RequestURI, w.method, w.uri)
		}
		if strings.Contains(r.RequestURI, "?") {
			t.Errorf("request %d carries a query: %s", i, r.RequestURI)
		}
		if len(r.Authorization) != 1 || r.Authorization[0] != "Bearer tok-123" {
			t.Errorf("request %d: Authorization %q, want [\"Bearer tok-123\"]", i, r.Authorization)
		}
		if len(r.Accept) != 1 || r.Accept[0] != "application/json" {
			t.Errorf("request %d: Accept %q, want [\"application/json\"]", i, r.Accept)
		}
		if w.ctype {
			if len(r.ContentType) != 1 || r.ContentType[0] != "application/merge-patch+json" {
				t.Errorf("request %d: Content-Type %q, want [\"application/merge-patch+json\"]", i, r.ContentType)
			}
		} else if len(r.ContentType) != 0 {
			t.Errorf("request %d: Content-Type %q, want none", i, r.ContentType)
		}
		if !bytes.Equal(r.Body, w.body) && !(len(r.Body) == 0 && len(w.body) == 0) {
			t.Errorf("request %d: body %q, want %q", i, r.Body, w.body)
		}
	}
}

// The operation table: what is not allowed returns ErrOperationNotAllowed and sends nothing;
// what is allowed sends one request.
func TestOperationTable(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)

	type op int
	const (
		opList op = iota
		opGet
		opMerge
		opStatus
	)
	opNames := map[op]string{opList: "List", opGet: "Get", opMerge: "MergePatch", opStatus: "PatchStatus"}
	allowed := map[kube.Resource][]op{
		kube.Instances:       {opList, opGet, opMerge, opStatus},
		kube.LoadBalancers:   {opList, opGet, opMerge, opStatus},
		kube.ScalewaySecrets: {opList, opGet, opMerge, opStatus},
		kube.NamespaceRoles:  {opList, opGet},
		kube.Namespaces:      {opList, opGet},
		kube.Secrets:         {},
		kube.CRDs:            {opGet},
		kube.Deployments:     {opList},
	}

	call := func(r kube.Resource, o op) error {
		ns := ""
		if r.Namespaced() {
			ns = "ns"
		}
		var err error
		switch o {
		case opList:
			_, err = c.List(ctx(), tok, r, ns)
		case opGet:
			_, err = c.Get(ctx(), tok, r, ns, "n")
		case opMerge:
			_, err = c.MergePatch(ctx(), tok, r, ns, "n", []byte(`{}`))
		case opStatus:
			_, err = c.PatchStatus(ctx(), tok, r, ns, "n", []byte(`{}`))
		}
		return err
	}

	for _, r := range kube.Resources() {
		for _, o := range []op{opList, opGet, opMerge, opStatus} {
			s.reset()
			err := call(r, o)
			isAllowed := false
			for _, a := range allowed[r] {
				if a == o {
					isAllowed = true
				}
			}
			n := len(s.requests())
			if isAllowed {
				if err != nil {
					t.Errorf("%s %s: error %v, want success", opNames[o], r, err)
				}
				if n != 1 {
					t.Errorf("%s %s: %d requests, want 1", opNames[o], r, n)
				}
			} else {
				if !errors.Is(err, kube.ErrOperationNotAllowed) {
					t.Errorf("%s %s: error %v, want ErrOperationNotAllowed", opNames[o], r, err)
				}
				if n != 0 {
					t.Errorf("%s %s: %d requests, want 0", opNames[o], r, n)
				}
			}
		}
	}
}

// Matrix row Not allowed, and the error precedence.
func TestClientErrorsSendNothing(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)
	p := []byte(`{}`)

	cases := []struct {
		label string
		call  func() error
		want  error
	}{
		{"Get Secrets", func() error { _, err := c.Get(ctx(), tok, kube.Secrets, "ns", "n"); return err }, kube.ErrOperationNotAllowed},
		{"MergePatch Deployments", func() error {
			_, err := c.MergePatch(ctx(), tok, kube.Deployments, "ns", "n", p)
			return err
		}, kube.ErrOperationNotAllowed},
		{"Get unknown resource", func() error { _, err := c.Get(ctx(), tok, kube.Resource(0), "ns", "n"); return err }, kube.ErrUnknownResource},
		{"List unknown resource", func() error { _, err := c.List(ctx(), tok, kube.Resource(9), ""); return err }, kube.ErrUnknownResource},
		{"unknown before not allowed and empty token", func() error {
			_, err := c.PatchStatus(ctx(), "", kube.Resource(99), "Bad", "a/b", p)
			return err
		}, kube.ErrUnknownResource},
		{"not allowed before invalid name", func() error { _, err := c.Get(ctx(), tok, kube.Secrets, "Bad", "a/b"); return err }, kube.ErrOperationNotAllowed},
		{"not allowed before empty token", func() error {
			_, err := c.PatchStatus(ctx(), "", kube.Namespaces, "", "n", p)
			return err
		}, kube.ErrOperationNotAllowed},
		{"invalid name before empty token", func() error { _, err := c.Get(ctx(), "", kube.Instances, "ns", "a/b"); return err }, kube.ErrInvalidName},
		{"Get bad namespace", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "Ns", "n"); return err }, kube.ErrInvalidName},
		{"Get namespace a.b", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "a.b", "n"); return err }, kube.ErrInvalidName},
		{"Get namespace 64 chars", func() error {
			_, err := c.Get(ctx(), tok, kube.Instances, strings.Repeat("a", 64), "n")
			return err
		}, kube.ErrInvalidName},
		{"Get name a/b", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "ns", "a/b"); return err }, kube.ErrInvalidName},
		{"Get name a%2Fb", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "ns", "a%2Fb"); return err }, kube.ErrInvalidName},
		{"Get name ..", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "ns", ".."); return err }, kube.ErrInvalidName},
		{"Get name 254 chars", func() error {
			_, err := c.Get(ctx(), tok, kube.Instances, "ns", strings.Repeat("a", 254))
			return err
		}, kube.ErrInvalidName},
		{"MergePatch name a/b", func() error { _, err := c.MergePatch(ctx(), tok, kube.Instances, "ns", "a/b", p); return err }, kube.ErrInvalidName},
		{"PatchStatus name ..", func() error { _, err := c.PatchStatus(ctx(), tok, kube.Instances, "ns", "..", p); return err }, kube.ErrInvalidName},
		{"List bad namespace", func() error { _, err := c.List(ctx(), tok, kube.Instances, "Ns"); return err }, kube.ErrInvalidName},
		{"Get empty name", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "ns", ""); return err }, kube.ErrInvalidName},
		{"MergePatch empty name", func() error { _, err := c.MergePatch(ctx(), tok, kube.Instances, "ns", "", p); return err }, kube.ErrInvalidName},
		{"PatchStatus empty name", func() error { _, err := c.PatchStatus(ctx(), tok, kube.Instances, "ns", "", p); return err }, kube.ErrInvalidName},
		{"Get namespaced without namespace", func() error { _, err := c.Get(ctx(), tok, kube.Instances, "", "n"); return err }, kube.ErrInvalidName},
		{"MergePatch namespaced without namespace", func() error {
			_, err := c.MergePatch(ctx(), tok, kube.Instances, "", "n", p)
			return err
		}, kube.ErrInvalidName},
		{"PatchStatus namespaced without namespace", func() error {
			_, err := c.PatchStatus(ctx(), tok, kube.ScalewaySecrets, "", "n", p)
			return err
		}, kube.ErrInvalidName},
		{"Get cluster kind with namespace", func() error { _, err := c.Get(ctx(), tok, kube.Namespaces, "ns", "n"); return err }, kube.ErrInvalidName},
		{"List cluster kind with namespace", func() error { _, err := c.List(ctx(), tok, kube.NamespaceRoles, "ns"); return err }, kube.ErrInvalidName},
		{"List Deployments without namespace", func() error { _, err := c.List(ctx(), tok, kube.Deployments, ""); return err }, kube.ErrInvalidName},
		{"Get Namespaces name a.b", func() error { _, err := c.Get(ctx(), tok, kube.Namespaces, "", "a.b"); return err }, kube.ErrInvalidName},
		{"List empty token", func() error { _, err := c.List(ctx(), "", kube.Instances, ""); return err }, kube.ErrEmptyToken},
		{"Get empty token", func() error { _, err := c.Get(ctx(), "", kube.Instances, "ns", "n"); return err }, kube.ErrEmptyToken},
		{"MergePatch empty token", func() error { _, err := c.MergePatch(ctx(), "", kube.Instances, "ns", "n", p); return err }, kube.ErrEmptyToken},
		{"PatchStatus empty token", func() error { _, err := c.PatchStatus(ctx(), "", kube.Instances, "ns", "n", p); return err }, kube.ErrEmptyToken},
	}
	for _, tc := range cases {
		s.reset()
		err := tc.call()
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: error %v, want %v", tc.label, err, tc.want)
		}
		if n := len(s.requests()); n != 0 {
			t.Errorf("%s: %d requests sent, want 0", tc.label, n)
		}
	}
}

// Matrix row API error: 404, 409, 401 give *APIError with that code.
func TestAPIError(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)

	for _, code := range []int{http.StatusNotFound, http.StatusConflict, http.StatusUnauthorized, http.StatusInternalServerError} {
		body := []byte(`{"kind":"Status","code":` + itoa(code) + `,"message":"body-secret-marker"}`)
		s.answer(code, body)

		checks := []struct {
			label  string
			method string
			path   string
			call   func() error
		}{
			{"List", http.MethodGet, scwGroup + "/namespaces/ns/instances", func() error {
				_, err := c.List(ctx(), tok, kube.Instances, "ns")
				return err
			}},
			{"Get", http.MethodGet, "/api/v1/namespaces/x", func() error {
				_, err := c.Get(ctx(), tok, kube.Namespaces, "", "x")
				return err
			}},
			{"MergePatch", http.MethodPatch, scwGroup + "/namespaces/ns/instances/n", func() error {
				_, err := c.MergePatch(ctx(), tok, kube.Instances, "ns", "n", []byte(`{}`))
				return err
			}},
			{"PatchStatus", http.MethodPatch, scwGroup + "/namespaces/ns/instances/n/status", func() error {
				_, err := c.PatchStatus(ctx(), tok, kube.Instances, "ns", "n", []byte(`{}`))
				return err
			}},
		}
		for _, ch := range checks {
			err := ch.call()
			var apiErr *kube.APIError
			if !errors.As(err, &apiErr) {
				t.Errorf("%d %s: error %v (%T), want *kube.APIError", code, ch.label, err, err)
				continue
			}
			if apiErr.Code != code {
				t.Errorf("%d %s: Code %d", code, ch.label, apiErr.Code)
			}
			if apiErr.Method != ch.method || apiErr.Path != ch.path {
				t.Errorf("%d %s: Method %q Path %q, want %q %q", code, ch.label, apiErr.Method, apiErr.Path, ch.method, ch.path)
			}
			if !bytes.Equal(apiErr.Body, body) {
				t.Errorf("%d %s: Body %q, want %q", code, ch.label, apiErr.Body, body)
			}
			wantMsg := "kube: " + ch.method + " " + ch.path + ": " + itoa(code)
			if got := apiErr.Error(); got != wantMsg {
				t.Errorf("%d %s: Error() = %q, want %q", code, ch.label, got, wantMsg)
			}
			if strings.Contains(err.Error(), "body-secret-marker") || strings.Contains(err.Error(), "tok-123") {
				t.Errorf("%d %s: error text leaks body or token: %q", code, ch.label, err.Error())
			}
		}
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// ErrBadResponse: a 2xx body that is not JSON, a list without items, a body over 16 MiB.
func TestBadResponse(t *testing.T) {
	s := newAPIServer(t)
	c := s.client(t)

	s.answer(http.StatusOK, []byte(`<html>not json</html>`))
	if _, err := c.Get(ctx(), tok, kube.Instances, "ns", "n"); !errors.Is(err, kube.ErrBadResponse) {
		t.Errorf("Get non-JSON: error %v, want ErrBadResponse", err)
	}
	if _, err := c.MergePatch(ctx(), tok, kube.Instances, "ns", "n", []byte(`{}`)); !errors.Is(err, kube.ErrBadResponse) {
		t.Errorf("MergePatch non-JSON: error %v, want ErrBadResponse", err)
	}
	if _, err := c.List(ctx(), tok, kube.Instances, "ns"); !errors.Is(err, kube.ErrBadResponse) {
		t.Errorf("List non-JSON: error %v, want ErrBadResponse", err)
	}

	for _, body := range []string{`{"kind":"InstanceList"}`, `{"items":{}}`, `{"items":"x"}`, `[]`} {
		s.answer(http.StatusOK, []byte(body))
		if _, err := c.List(ctx(), tok, kube.Instances, "ns"); !errors.Is(err, kube.ErrBadResponse) {
			t.Errorf("List %s: error %v, want ErrBadResponse", body, err)
		}
	}

	const limit = 16 << 20
	big := make([]byte, 0, limit+64)
	big = append(big, `{"pad":"`...)
	big = append(big, bytes.Repeat([]byte("a"), limit)...)
	big = append(big, `"}`...)
	s.answer(http.StatusOK, big)
	if _, err := c.Get(ctx(), tok, kube.Instances, "ns", "n"); !errors.Is(err, kube.ErrBadResponse) {
		t.Errorf("Get over 16 MiB: error %v, want ErrBadResponse", err)
	}

	s.answer(http.StatusNotFound, big)
	_, err := c.Get(ctx(), tok, kube.Instances, "ns", "n")
	var apiErr *kube.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("404 over 16 MiB: error %v, want *kube.APIError", err)
	}
	if apiErr.Code != http.StatusNotFound || len(apiErr.Body) != limit || !bytes.Equal(apiErr.Body, big[:limit]) {
		t.Errorf("404 over 16 MiB: Code %d, len(Body) %d, want 404 and the first %d bytes", apiErr.Code, len(apiErr.Body), limit)
	}
}

// Matrix row Environment: host or port unset or empty gives ErrNoAPIServer, before the CA.
func TestNewInClusterEnvironment(t *testing.T) {
	s := newAPIServer(t)
	ca := certPEM(s.srv.Certificate())

	cases := []struct {
		label string
		env   map[string]string
	}{
		{"both unset", map[string]string{}},
		{"host unset", map[string]string{"KUBERNETES_SERVICE_PORT": "443"}},
		{"port unset", map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"}},
		{"host empty", map[string]string{"KUBERNETES_SERVICE_HOST": "", "KUBERNETES_SERVICE_PORT": "443"}},
		{"port empty", map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1", "KUBERNETES_SERVICE_PORT": ""}},
	}
	for _, tc := range cases {
		getenv := func(k string) string { return tc.env[k] }
		if c, err := kube.NewInCluster(getenv, ca); !errors.Is(err, kube.ErrNoAPIServer) || c != nil {
			t.Errorf("%s: %v, %v; want nil, ErrNoAPIServer", tc.label, c, err)
		}
		if _, err := kube.NewInCluster(getenv, nil); !errors.Is(err, kube.ErrNoAPIServer) {
			t.Errorf("%s with no CA: %v, want ErrNoAPIServer first", tc.label, err)
		}
	}
}

// ErrNoCA: PEM with no certificate.
func TestNewInClusterNoCA(t *testing.T) {
	env := envFor("10.0.0.1", "443")
	keyBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}})
	for _, ca := range [][]byte{nil, {}, []byte("not pem"), keyBlock} {
		if c, err := kube.NewInCluster(env, ca); !errors.Is(err, kube.ErrNoCA) || c != nil {
			t.Errorf("CA %q: %v, %v; want nil, ErrNoCA", ca, c, err)
		}
	}
}

// Matrix row IPv6: host ::1 gives the base https://[::1]:<port>.
func TestNewInClusterIPv6(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	s := &apiServer{code: http.StatusOK, body: []byte(`{"metadata":{"name":"n"}}`)}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.handle))
	s.srv.Listener.Close()
	s.srv.Listener = ln
	s.srv.StartTLS()
	t.Cleanup(s.srv.Close)

	_, port := hostPort(t, s.srv)
	c, err := kube.NewInCluster(envFor("::1", port), certPEM(s.srv.Certificate()))
	if err != nil {
		t.Fatalf("NewInCluster with ::1: %v", err)
	}
	if _, err := c.Get(ctx(), tok, kube.Namespaces, "", "n"); err != nil {
		t.Fatalf("Get through [::1]:%s: %v", port, err)
	}
	if reqs := s.requests(); len(reqs) != 1 || reqs[0].RequestURI != "/api/v1/namespaces/n" {
		t.Errorf("requests %+v, want one GET /api/v1/namespaces/n", reqs)
	}
}

// otherCA returns a self-signed CA certificate unrelated to httptest's.
func otherCA(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "other test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:              []string{"localhost", "example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// Matrix row Trust: a server certificate from another CA gives a TLS error, no response read.
// The CA comes only from caPEM: the system pool is not trusted either.
func TestTrustOnlyGivenCA(t *testing.T) {
	s := newAPIServer(t)
	host, port := hostPort(t, s.srv)

	c, err := kube.NewInCluster(envFor(host, port), otherCA(t))
	if err != nil {
		t.Fatalf("NewInCluster with other CA: %v", err)
	}
	_, err = c.Get(ctx(), tok, kube.Namespaces, "", "n")
	if err == nil {
		t.Fatal("Get with a server certificate from another CA: no error")
	}
	var apiErr *kube.APIError
	if errors.As(err, &apiErr) {
		t.Errorf("TLS failure reported as *APIError: %v", err)
	}
	var unknownAuth x509.UnknownAuthorityError
	var certErr *tls.CertificateVerificationError
	if !errors.As(err, &unknownAuth) && !errors.As(err, &certErr) && !strings.Contains(err.Error(), "certificate") {
		t.Errorf("error %v, want a certificate verification error", err)
	}
	if n := len(s.requests()); n != 0 {
		t.Errorf("handler saw %d requests, want 0", n)
	}
	if strings.Contains(err.Error(), "tok-123") {
		t.Errorf("TLS error leaks the token: %v", err)
	}

	// A second httptest server: the same client still refuses it.
	s2 := newAPIServer(t)
	h2, p2 := hostPort(t, s2.srv)
	c2, err := kube.NewInCluster(envFor(h2, p2), otherCA(t))
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	if _, err := c2.List(ctx(), tok, kube.Namespaces, ""); err == nil {
		t.Error("List against an untrusted server: no error")
	}
	if n := len(s2.requests()); n != 0 {
		t.Errorf("second handler saw %d requests, want 0", n)
	}
}

// The client never logs: no output on the default loggers, whatever happens.
func TestClientNeverLogs(t *testing.T) {
	var logBuf, slogBuf bytes.Buffer
	oldOut, oldFlags := log.Writer(), log.Flags()
	oldSlog := slog.Default()
	log.SetOutput(&logBuf)
	slog.SetDefault(slog.New(slog.NewTextHandler(&slogBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
		slog.SetDefault(oldSlog)
	})

	s := newAPIServer(t)
	c := s.client(t)
	_, _ = c.List(ctx(), tok, kube.Instances, "")
	_, _ = c.Get(ctx(), tok, kube.Instances, "ns", "n")
	_, _ = c.MergePatch(ctx(), tok, kube.Instances, "ns", "n", []byte(`{"a":1}`))
	_, _ = c.PatchStatus(ctx(), tok, kube.Instances, "ns", "n", []byte(`{"a":1}`))
	s.answer(http.StatusUnauthorized, []byte(`{"kind":"Status"}`))
	_, _ = c.Get(ctx(), tok, kube.Instances, "ns", "n")
	s.answer(http.StatusOK, []byte(`garbage`))
	_, _ = c.Get(ctx(), tok, kube.Instances, "ns", "n")
	_, _ = c.Get(ctx(), "", kube.Instances, "ns", "n")
	_, _ = c.Get(ctx(), tok, kube.Secrets, "ns", "n")

	if logBuf.Len() != 0 {
		t.Errorf("log output: %q", logBuf.String())
	}
	if slogBuf.Len() != 0 {
		t.Errorf("slog output: %q", slogBuf.String())
	}
}
