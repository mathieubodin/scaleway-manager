package server_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/server"
)

const (
	wantCSP     = "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
	wantNosniff = "nosniff"
)

var fixtureIndex = []byte("<!doctype html>\n<html lang=\"en\"><title>fixture</title><p>fixture page</p></html>\n")

func do(t *testing.T, srv *httptest.Server, method, path string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body of %s %s: %v", method, path, err)
	}
	return resp, body
}

func assertSecurityHeaders(t *testing.T, resp *http.Response, label string) {
	t.Helper()
	if got := resp.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != wantCSP {
		t.Errorf("%s: Content-Security-Policy = %q, want exactly [%q]", label, got, wantCSP)
	}
	if got := resp.Header.Values("X-Content-Type-Options"); len(got) != 1 || got[0] != wantNosniff {
		t.Errorf("%s: X-Content-Type-Options = %q, want exactly [%q]", label, got, wantNosniff)
	}
}

// Matrix rows Health, Health HEAD, Page, Unknown, plus the 405 of POST /healthz:
// status, body and the AD-8 headers on each response.
func TestRoutesAndSecurityHeaders(t *testing.T) {
	srv := httptest.NewServer(server.New(fixtureIndex))
	defer srv.Close()

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   []byte // nil: body not checked
	}{
		{"health", http.MethodGet, "/healthz", http.StatusOK, []byte("ok")},
		{"health HEAD", http.MethodHead, "/healthz", http.StatusOK, []byte{}},
		{"page", http.MethodGet, "/", http.StatusOK, fixtureIndex},
		{"unknown", http.MethodGet, "/nope", http.StatusNotFound, nil},
		{"unknown under root", http.MethodGet, "/index.html", http.StatusNotFound, nil},
		{"unknown under healthz", http.MethodGet, "/healthz/x", http.StatusNotFound, nil},
		{"method not allowed", http.MethodPost, "/healthz", http.StatusMethodNotAllowed, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := do(t, srv, tc.method, tc.path)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, resp.StatusCode, tc.wantStatus)
			}
			if tc.wantBody != nil && !bytes.Equal(body, tc.wantBody) {
				t.Errorf("%s %s: body %q, want %q", tc.method, tc.path, body, tc.wantBody)
			}
			assertSecurityHeaders(t, resp, tc.method+" "+tc.path)
		})
	}
}

// The page is served from the bytes given to New, whatever they are.
func TestPageServesGivenBytes(t *testing.T) {
	other := []byte("<p>another page</p>")
	srv := httptest.NewServer(server.New(other))
	defer srv.Close()

	resp, body := do(t, srv, http.MethodGet, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d, want 200", resp.StatusCode)
	}
	if !bytes.Equal(body, other) {
		t.Errorf("GET /: body %q, want %q", body, other)
	}
}

// The health body is exactly "ok": no trailing newline.
func TestHealthBodyHasNoNewline(t *testing.T) {
	srv := httptest.NewServer(server.New(fixtureIndex))
	defer srv.Close()

	_, body := do(t, srv, http.MethodGet, "/healthz")
	if len(body) != 2 || string(body) != "ok" {
		t.Errorf("GET /healthz: body %q (%d bytes), want exactly \"ok\" (2 bytes)", body, len(body))
	}
}
