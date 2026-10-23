package kubetest_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

// Behavior 10: a request without a bearer token gets 401.
// Reproduces: https://kubernetes.io/docs/reference/access-authn-authz/authentication/
func TestNoBearerTokenIs401(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))

	code, body := rawDo(t, s, http.MethodGet, mustPath(t, kube.Instances, "ns", "i", false), "", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("code %d, want 401", code)
	}
	assertStatusBody(t, body, http.StatusUnauthorized, "Unauthorized")

	// The token check comes before 404 and 415.
	code, _ = rawDo(t, s, http.MethodGet, mustPath(t, kube.Instances, "ns", "absent", false), "", "", nil)
	if code != http.StatusUnauthorized {
		t.Errorf("absent object without token: code %d, want 401", code)
	}
	code, _ = rawDo(t, s, http.MethodPatch, mustPath(t, kube.Instances, "ns", "i", false), "", "application/json", []byte(`{}`))
	if code != http.StatusUnauthorized {
		t.Errorf("wrong content type without token: code %d, want 401", code)
	}

	if got := s.Tokens(); !reflect.DeepEqual(got, []string{"", "", ""}) {
		t.Errorf("Tokens() = %q, want three empty entries", got)
	}
}

// Behavior 11: every received token is recorded; once ExpectToken is set, another token gets 401.
// Reproduces: https://kubernetes.io/docs/reference/access-authn-authz/authentication/
func TestTokenRecordingAndExpectToken(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))
	c := newClient(t, s)

	// Before ExpectToken, any token is accepted.
	if _, err := c.Get(ctx, kube.Token("first"), kube.Instances, "ns", "i"); err != nil {
		t.Fatalf("Get with any token before ExpectToken: %v", err)
	}

	s.ExpectToken("good")
	_, err := c.Get(ctx, kube.Token("bad"), kube.Instances, "ns", "i")
	ae := apiErr(t, err)
	if ae.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: code %d, want 401", ae.Code)
	}
	assertStatusBody(t, ae.Body, http.StatusUnauthorized, "Unauthorized")

	if _, err := c.Get(ctx, kube.Token("good"), kube.Instances, "ns", "i"); err != nil {
		t.Errorf("expected token: %v", err)
	}

	// A rotated token: the old one is now refused.
	s.ExpectToken("rotated")
	if _, err := c.Get(ctx, kube.Token("good"), kube.Instances, "ns", "i"); apiCode(t, err) != http.StatusUnauthorized {
		t.Errorf("old token after rotation: want 401")
	}
	if _, err := c.Get(ctx, kube.Token("rotated"), kube.Instances, "ns", "i"); err != nil {
		t.Errorf("rotated token: %v", err)
	}

	want := []string{"first", "bad", "good", "good", "rotated"}
	if got := s.Tokens(); !reflect.DeepEqual(got, want) {
		t.Errorf("Tokens() = %q, want %q", got, want)
	}
}

// Behavior 12: 415 on a PATCH whose Content-Type is not application/merge-patch+json.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/#patch-and-apply
func TestPatchContentTypeIs415(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"x":0}`))
	before := mustRaw(t, s, kube.Instances, "ns", "i")
	path := mustPath(t, kube.Instances, "ns", "i", false)
	statusPath := mustPath(t, kube.Instances, "ns", "i", true)

	for _, ct := range []string{"application/json", "application/json-patch+json", "application/strategic-merge-patch+json", "application/apply-patch+yaml", ""} {
		for _, p := range []string{path, statusPath} {
			code, body := rawDo(t, s, http.MethodPatch, p, testToken, ct, []byte(`{"spec":{"x":1},"status":{"x":1}}`))
			if code != http.StatusUnsupportedMediaType {
				t.Errorf("PATCH %s with %q: code %d, want 415", p, ct, code)
				continue
			}
			assertStatusBody(t, body, http.StatusUnsupportedMediaType, "UnsupportedMediaType")
		}
	}
	if after := mustRaw(t, s, kube.Instances, "ns", "i"); string(after) != string(before) {
		t.Errorf("object changed by a 415 patch: %s, was %s", after, before)
	}

	// The 415 check comes before 404.
	code, _ := rawDo(t, s, http.MethodPatch, mustPath(t, kube.Instances, "ns", "absent", false), testToken, "application/json", []byte(`{}`))
	if code != http.StatusUnsupportedMediaType {
		t.Errorf("absent object with wrong content type: code %d, want 415", code)
	}

	// The right content type is accepted.
	code, body := rawDo(t, s, http.MethodPatch, path, testToken, mergePatchType, []byte(`{"spec":{"x":2}}`))
	if code != http.StatusOK {
		t.Errorf("merge-patch content type: code %d, body %s", code, body)
	}
}
