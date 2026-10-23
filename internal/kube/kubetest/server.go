// Package kubetest is the fake Kubernetes API server of the Go tests (AD-15).
package kubetest

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const mergePatchType = "application/merge-patch+json"

// Server is a TLS test server holding objects in memory.
type Server struct {
	Host  string
	Port  string
	CAPEM []byte

	t        testing.TB
	mu       sync.Mutex
	rv       int64
	objects  map[key]map[string]any
	expected string
	requests []Request
	faults   []fault
}

// Request is one recorded request.
type Request struct {
	Method      string
	RequestURI  string
	ContentType string
	Token       string
	Body        []byte
}

type key struct {
	r         kube.Resource
	namespace string
	name      string
}

type fault struct {
	method string
	prefix string
	code   int
}

// New starts a TLS server and closes it at cleanup.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{t: t, objects: map[key]map[string]any{}}
	srv := httptest.NewTLSServer(http.HandlerFunc(s.serve))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("kubetest: parse server URL: %v", err)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("kubetest: split server host: %v", err)
	}
	s.Host, s.Port = host, port
	s.CAPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	return s
}

// Put seeds or replaces an object, keyed by metadata.namespace and metadata.name.
func (s *Server) Put(r kube.Resource, obj json.RawMessage) {
	s.t.Helper()
	m, ok := decodeObject(obj)
	if !ok {
		s.t.Fatalf("kubetest: Put(%v): not a JSON object: %s", r, obj)
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta == nil {
		s.t.Fatalf("kubetest: Put(%v): no metadata", r)
	}
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	if name == "" {
		s.t.Fatalf("kubetest: Put(%v): no metadata.name", r)
	}
	if r.Namespaced() && namespace == "" {
		s.t.Fatalf("kubetest: Put(%v): namespaced kind without metadata.namespace", r)
	}
	if !r.Namespaced() && namespace != "" {
		s.t.Fatalf("kubetest: Put(%v): cluster kind with metadata.namespace %q", r, namespace)
	}
	if _, ok := meta["generation"]; !ok {
		meta["generation"] = json.Number("1")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meta["resourceVersion"] = s.nextRV()
	s.objects[key{r, namespace, name}] = m
}

// Object returns the stored object.
func (s *Server) Object(r kube.Resource, namespace, name string) (json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.objects[key{r, namespace, name}]
	if !ok {
		return nil, false
	}
	return encode(m), true
}

// Delete removes an object without finalizers, or sets metadata.deletionTimestamp on one with
// finalizers. It does nothing on an absent object or on one already deleting.
func (s *Server) Delete(r kube.Resource, namespace, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key{r, namespace, name}
	m, ok := s.objects[k]
	if !ok {
		return
	}
	meta, _ := m["metadata"].(map[string]any)
	if _, deleting := meta["deletionTimestamp"]; deleting {
		return
	}
	if !hasFinalizers(meta) {
		delete(s.objects, k)
		return
	}
	meta["deletionTimestamp"] = time.Now().UTC().Format(time.RFC3339)
	meta["resourceVersion"] = s.nextRV()
}

// ExpectToken makes every request with another token get 401.
func (s *Server) ExpectToken(tok string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expected = tok
}

// Tokens returns the token of every received request, in order; "" when absent.
func (s *Server) Tokens() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.requests))
	for i, r := range s.requests {
		out[i] = r.Token
	}
	return out
}

// Requests returns every received request, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	for i, r := range s.requests {
		r.Body = bytes.Clone(r.Body)
		out[i] = r
	}
	return out
}

// Fail answers the requests whose method matches (method "" matches any) and whose URL path
// starts with pathPrefix with code, until ClearFaults.
func (s *Server) Fail(method, pathPrefix string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults = append(s.faults, fault{method, pathPrefix, code})
}

// ClearFaults removes every fault.
func (s *Server) ClearFaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults = nil
}

func (s *Server) nextRV() string {
	s.rv++
	return strconv.FormatInt(s.rv, 10)
}

func (s *Server) serve(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		body = nil
	}
	tok := ""
	if h := req.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok = strings.TrimPrefix(h, "Bearer ")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, Request{
		Method:      req.Method,
		RequestURI:  req.RequestURI,
		ContentType: req.Header.Get("Content-Type"),
		Token:       tok,
		Body:        body,
	})

	if tok == "" || (s.expected != "" && tok != s.expected) {
		writeStatus(w, http.StatusUnauthorized)
		return
	}
	for _, f := range s.faults {
		if (f.method == "" || f.method == req.Method) && strings.HasPrefix(req.URL.Path, f.prefix) {
			writeStatus(w, f.code)
			return
		}
	}
	if req.Method != http.MethodGet && req.Method != http.MethodPatch {
		writeStatus(w, http.StatusMethodNotAllowed)
		return
	}
	if req.Method == http.MethodPatch {
		mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil || mt != mergePatchType {
			writeStatus(w, http.StatusUnsupportedMediaType)
			return
		}
	}
	r, namespace, name, status, err := kube.ParsePath(req.URL.Path)
	if err != nil {
		writeStatus(w, http.StatusNotFound)
		return
	}
	if req.Method == http.MethodGet {
		if name == "" {
			s.list(w, r, namespace)
			return
		}
		m, ok := s.objects[key{r, namespace, name}]
		if !ok {
			writeStatus(w, http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, m)
		return
	}
	if name == "" {
		writeStatus(w, http.StatusMethodNotAllowed)
		return
	}
	s.patch(w, key{r, namespace, name}, status, body)
}

func (s *Server) list(w http.ResponseWriter, r kube.Resource, namespace string) {
	var keys []key
	for k := range s.objects {
		if k.r == r && (namespace == "" || k.namespace == namespace) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].namespace != keys[j].namespace {
			return keys[i].namespace < keys[j].namespace
		}
		return keys[i].name < keys[j].name
	})
	items := make([]any, 0, len(keys))
	for _, k := range keys {
		items = append(items, s.objects[k])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion": "v1",
		"kind":       r.Kind() + "List",
		"metadata":   map[string]any{"resourceVersion": strconv.FormatInt(s.rv, 10)},
		"items":      items,
	})
}

func (s *Server) patch(w http.ResponseWriter, k key, statusOnly bool, body []byte) {
	p, ok := decodeObject(body)
	if !ok {
		writeStatus(w, http.StatusBadRequest)
		return
	}
	old, ok := s.objects[k]
	if !ok {
		writeStatus(w, http.StatusNotFound)
		return
	}
	oldMeta, _ := old["metadata"].(map[string]any)
	if pm, ok := p["metadata"].(map[string]any); ok {
		if rv, present := pm["resourceVersion"]; present && rv != nil && rv != oldMeta["resourceVersion"] {
			writeStatus(w, http.StatusConflict)
			return
		}
	}

	eff := map[string]any{}
	if statusOnly {
		if st, present := p["status"]; present {
			eff["status"] = st
		}
	} else {
		for f, v := range p {
			eff[f] = v
		}
		if hasStatusSubresource(k.r) {
			delete(eff, "status")
		}
		if pm, isMap := eff["metadata"].(map[string]any); isMap {
			delete(pm, "name")
			delete(pm, "namespace")
			delete(pm, "resourceVersion")
		} else {
			delete(eff, "metadata")
		}
	}

	updated, _ := mergePatch(deepCopy(old), eff).(map[string]any)
	if reflect.DeepEqual(updated, old) {
		writeJSON(w, http.StatusOK, old)
		return
	}
	meta, _ := updated["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		updated["metadata"] = meta
	}
	if !reflect.DeepEqual(old["spec"], updated["spec"]) {
		meta["generation"] = bumpGeneration(meta["generation"])
	}
	meta["resourceVersion"] = s.nextRV()
	if _, deleting := meta["deletionTimestamp"]; deleting && !hasFinalizers(meta) {
		delete(s.objects, k)
	} else {
		s.objects[k] = updated
	}
	writeJSON(w, http.StatusOK, updated)
}

func hasStatusSubresource(r kube.Resource) bool {
	return r == kube.Instances || r == kube.LoadBalancers || r == kube.ScalewaySecrets
}

func hasFinalizers(meta map[string]any) bool {
	f, _ := meta["finalizers"].([]any)
	return len(f) > 0
}

func bumpGeneration(v any) json.Number {
	var n int64
	switch g := v.(type) {
	case json.Number:
		n, _ = g.Int64()
	case float64:
		n = int64(g)
	}
	return json.Number(strconv.FormatInt(n+1, 10))
}

// decodeObject decodes a JSON object with UseNumber.
func decodeObject(b []byte) (map[string]any, bool) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil || m == nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return m, true
}

func encode(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// Values come from a JSON decoder, so they always re-encode.
		panic("kubetest: encode: " + err.Error())
	}
	return b
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(encode(v))
}

// writeStatus answers with a Kubernetes Status object.
func writeStatus(w http.ResponseWriter, code int) {
	writeJSON(w, code, map[string]any{
		"kind":       "Status",
		"apiVersion": "v1",
		"status":     "Failure",
		"code":       code,
		"reason":     reason(code),
	})
}

func reason(code int) string {
	if code == http.StatusInternalServerError {
		return "InternalError"
	}
	return strings.ReplaceAll(http.StatusText(code), " ", "")
}
