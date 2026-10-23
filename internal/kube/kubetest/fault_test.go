package kubetest_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

// Fail answers matching requests with the code until ClearFaults.
func TestFail(t *testing.T) {
	objPath := func(t *testing.T) string { return mustPath(t, kube.Instances, "ns", "i", false) }
	instancesPrefix := "/apis/scaleway.mathieubodin.io/v1/namespaces/ns/instances"

	cases := []struct {
		name       string
		method     string
		prefix     string
		reqMethod  string
		path       func(t *testing.T) string
		wantCode   int
		wantReason string
	}{
		{"any method matches GET", "", instancesPrefix, http.MethodGet, objPath, 500, "InternalError"},
		{"any method matches PATCH", "", instancesPrefix, http.MethodPatch, objPath, 503, "ServiceUnavailable"},
		{"method GET matches GET", http.MethodGet, instancesPrefix, http.MethodGet, objPath, 404, "NotFound"},
		{"method PATCH skips GET", http.MethodPatch, instancesPrefix, http.MethodGet, objPath, 200, ""},
		{"method GET skips PATCH", http.MethodGet, instancesPrefix, http.MethodPatch, objPath, 200, ""},
		{"prefix of the status path", http.MethodPatch, instancesPrefix + "/i/status", http.MethodPatch,
			func(t *testing.T) string { return mustPath(t, kube.Instances, "ns", "i", true) }, 409, "Conflict"},
		{"status prefix skips object path", "", instancesPrefix + "/i/status", http.MethodPatch, objPath, 200, ""},
		{"other prefix does not match", "", "/api/v1/namespaces", http.MethodGet, objPath, 200, ""},
		{"code 401", "", "/apis/", http.MethodGet, objPath, 401, "Unauthorized"},
		{"code 418 reason from status text", "", "/apis/", http.MethodGet, objPath, 418, "I'mateapot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := kubetest.New(t)
			s.Put(kube.Instances, instance("ns", "i", `"spec":{"x":0}`))
			faultCode := tc.wantCode
			if tc.wantReason == "" {
				// A non-matching fault: the request must still succeed.
				faultCode = http.StatusInternalServerError
			}
			s.Fail(tc.method, tc.prefix, faultCode)
			p := tc.path(t)
			var body []byte
			if tc.reqMethod == http.MethodPatch {
				body = []byte(`{"spec":{"x":1},"status":{"x":1}}`)
			}
			ct := ""
			if body != nil {
				ct = mergePatchType
			}
			code, resp := rawDo(t, s, tc.reqMethod, p, testToken, ct, body)
			want := tc.wantCode
			if tc.wantReason == "" {
				want = http.StatusOK
			}
			if code != want {
				t.Fatalf("%s %s: code %d, want %d (body %s)", tc.reqMethod, p, code, want, resp)
			}
			if tc.wantReason != "" {
				assertStatusBody(t, resp, tc.wantCode, tc.wantReason)
				if tc.reqMethod == http.MethodPatch {
					if x := numberField(t, mustObject(t, s, kube.Instances, "ns", "i"), "spec", "x"); x != 0 {
						t.Errorf("faulted patch applied: spec.x = %v", x)
					}
				}
			}

			// Faults persist until ClearFaults.
			if tc.wantReason != "" {
				if again, _ := rawDo(t, s, tc.reqMethod, p, testToken, ct, body); again != tc.wantCode {
					t.Errorf("second request: code %d, want %d", again, tc.wantCode)
				}
			}
			s.ClearFaults()
			if code, resp := rawDo(t, s, tc.reqMethod, p, testToken, ct, body); code != http.StatusOK {
				t.Errorf("after ClearFaults: code %d, body %s", code, resp)
			}
		})
	}
}

// Faults come after the token check, and are seen by the client as *kube.APIError.
func TestFailOrderAndClient(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))
	s.Fail("", "/", http.StatusInternalServerError)

	if code, _ := rawDo(t, s, http.MethodGet, mustPath(t, kube.Instances, "ns", "i", false), "", "", nil); code != http.StatusUnauthorized {
		t.Errorf("no token with a fault: code %d, want 401", code)
	}
	s.ExpectToken("good")
	c := newClient(t, s)
	if _, err := c.Get(ctx, kube.Token("bad"), kube.Instances, "ns", "i"); apiCode(t, err) != http.StatusUnauthorized {
		t.Errorf("wrong token with a fault: want 401")
	}
	if _, err := c.Get(ctx, kube.Token("good"), kube.Instances, "ns", "i"); apiCode(t, err) != http.StatusInternalServerError {
		t.Errorf("good token with a fault: want 500")
	}
	// Faults come before 415.
	code, _ := rawDo(t, s, http.MethodPatch, mustPath(t, kube.Instances, "ns", "i", false), "good", "application/json", []byte(`{}`))
	if code != http.StatusInternalServerError {
		t.Errorf("wrong content type with a fault: code %d, want 500", code)
	}
}

// Requests() records every request in order, rejected ones included.
func TestRequestsLog(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))
	c := newClient(t, s)

	if _, err := c.List(ctx, kube.Token("t1"), kube.Instances, ""); err != nil {
		t.Fatalf("List: %v", err)
	}
	patch := []byte(`{"status":{"a":1}}`)
	if _, err := c.PatchStatus(ctx, kube.Token("t2"), kube.Instances, "ns", "i", patch); err != nil {
		t.Fatalf("PatchStatus: %v", err)
	}
	rawDo(t, s, http.MethodPatch, mustPath(t, kube.Instances, "ns", "i", false), "", "application/json", []byte(`{"x":1}`))

	got := s.Requests()
	if len(got) != 3 {
		t.Fatalf("got %d requests, want 3: %+v", len(got), got)
	}
	want := []kubetest.Request{
		{Method: http.MethodGet, RequestURI: mustPath(t, kube.Instances, "", "", false), Token: "t1"},
		{Method: http.MethodPatch, RequestURI: mustPath(t, kube.Instances, "ns", "i", true), ContentType: mergePatchType, Token: "t2", Body: patch},
		{Method: http.MethodPatch, RequestURI: mustPath(t, kube.Instances, "ns", "i", false), ContentType: "application/json", Token: "", Body: []byte(`{"x":1}`)},
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Method != w.Method || g.RequestURI != w.RequestURI || g.Token != w.Token || string(g.Body) != string(w.Body) {
			t.Errorf("request %d = {%s %s tok=%q body=%s}, want {%s %s tok=%q body=%s}",
				i, g.Method, g.RequestURI, g.Token, g.Body, w.Method, w.RequestURI, w.Token, w.Body)
		}
		if w.ContentType != "" && !strings.HasPrefix(g.ContentType, w.ContentType) {
			t.Errorf("request %d content type = %q, want %q", i, g.ContentType, w.ContentType)
		}
	}
}

// The server is safe for concurrent requests.
func TestConcurrentPatches(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))
	c := newClient(t, s)

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "k" + string(rune('a'+i))
			_, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"metadata":{"labels":{"`+key+`":"v"}}}`))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent patch: %v", err)
		}
	}
	m := mustObject(t, s, kube.Instances, "ns", "i")
	labels, _ := field(m, "metadata", "labels")
	if lm, ok := labels.(map[string]any); !ok || len(lm) != n {
		t.Errorf("labels = %v, want %d entries", labels, n)
	}
	if got := len(s.Requests()); got != n {
		t.Errorf("Requests() has %d entries, want %d", got, n)
	}
}
