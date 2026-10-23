package kubetest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

// Behavior 4: merge patch on the object follows RFC 7386; status is ignored on a kind
// with a status subresource; metadata.generation is incremented when spec changes.
// Reproduces: https://www.rfc-editor.org/rfc/rfc7386
// Reproduces: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
func TestMergePatchObject(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"keep":"k","drop":"d","list":[1,2,3],"nested":{"a":1,"b":2}},"status":{"phase":"Ready"}`))
	c := newClient(t, s)

	out, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i",
		[]byte(`{"spec":{"drop":null,"list":[4],"nested":{"b":3}},"status":{"phase":"Error"}}`))
	if err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	for label, raw := range map[string][]byte{"response": out, "stored": mustRaw(t, s, kube.Instances, "ns", "i")} {
		m := decode(t, raw)
		spec, _ := field(m, "spec")
		want := map[string]any{"keep": "k", "list": []any{float64(4)}, "nested": map[string]any{"a": float64(1), "b": float64(3)}}
		if !reflect.DeepEqual(spec, want) {
			t.Errorf("%s: spec = %v, want %v", label, spec, want)
		}
		if got := stringField(t, m, "status", "phase"); got != "Ready" {
			t.Errorf("%s: status.phase = %q, want Ready (status ignored on object patch)", label, got)
		}
		if got := numberField(t, m, "metadata", "generation"); got != 2 {
			t.Errorf("%s: generation = %v, want 2", label, got)
		}
	}

	// A metadata-only change does not bump generation.
	if _, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"metadata":{"labels":{"l":"v"}}}`)); err != nil {
		t.Fatalf("MergePatch labels: %v", err)
	}
	m := mustObject(t, s, kube.Instances, "ns", "i")
	if got := numberField(t, m, "metadata", "generation"); got != 2 {
		t.Errorf("generation after labels patch = %v, want 2", got)
	}
	if got := stringField(t, m, "metadata", "labels", "l"); got != "v" {
		t.Errorf("label l = %q, want v", got)
	}
}

// Behavior 5: merge patch on /status applies only status.
// Reproduces: https://www.rfc-editor.org/rfc/rfc7386
// Reproduces: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
func TestMergePatchStatus(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.LoadBalancers, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"LoadBalancer","metadata":{"namespace":"ns","name":"lb"},"spec":{"x":1}}`))
	c := newClient(t, s)

	out, err := c.PatchStatus(ctx, kube.Token(testToken), kube.LoadBalancers, "ns", "lb",
		[]byte(`{"spec":{"x":2},"metadata":{"labels":{"l":"v"}},"status":{"a":1}}`))
	if err != nil {
		t.Fatalf("PatchStatus: %v", err)
	}
	for label, raw := range map[string][]byte{"response": out, "stored": mustRaw(t, s, kube.LoadBalancers, "ns", "lb")} {
		m := decode(t, raw)
		if got := numberField(t, m, "status", "a"); got != 1 {
			t.Errorf("%s: status.a = %v, want 1", label, got)
		}
		if got := numberField(t, m, "spec", "x"); got != 1 {
			t.Errorf("%s: spec.x = %v, want 1 (spec ignored on /status)", label, got)
		}
		if _, ok := field(m, "metadata", "labels"); ok {
			t.Errorf("%s: labels set by a /status patch", label)
		}
		if got := numberField(t, m, "metadata", "generation"); got != 1 {
			t.Errorf("%s: generation = %v, want 1", label, got)
		}
	}
}

// Matrix rows: status isolation, status only, null removal.
// Reproduces: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
func TestPatchMatrix(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"namespace":"ns","name":"sec"},"spec":{"x":0},"status":{"a":0,"b":"kept"}}`))
	c := newClient(t, s)
	tok := kube.Token(testToken)

	// Status isolation.
	if _, err := c.MergePatch(ctx, tok, kube.ScalewaySecrets, "ns", "sec", []byte(`{"spec":{"x":1},"status":{"a":1}}`)); err != nil {
		t.Fatalf("status isolation: %v", err)
	}
	m := mustObject(t, s, kube.ScalewaySecrets, "ns", "sec")
	if got := numberField(t, m, "spec", "x"); got != 1 {
		t.Errorf("status isolation: spec.x = %v, want 1", got)
	}
	if got := numberField(t, m, "status", "a"); got != 0 {
		t.Errorf("status isolation: status.a = %v, want 0", got)
	}
	if got := numberField(t, m, "metadata", "generation"); got != 2 {
		t.Errorf("status isolation: generation = %v, want 2", got)
	}

	// Status only.
	if _, err := c.PatchStatus(ctx, tok, kube.ScalewaySecrets, "ns", "sec", []byte(`{"metadata":{"labels":{"l":"v"}},"status":{"a":1}}`)); err != nil {
		t.Fatalf("status only: %v", err)
	}
	m = mustObject(t, s, kube.ScalewaySecrets, "ns", "sec")
	if got := numberField(t, m, "status", "a"); got != 1 {
		t.Errorf("status only: status.a = %v, want 1", got)
	}
	if _, ok := field(m, "metadata", "labels"); ok {
		t.Errorf("status only: labels changed")
	}
	if got := numberField(t, m, "metadata", "generation"); got != 2 {
		t.Errorf("status only: generation = %v, want 2", got)
	}

	// Null removal.
	if _, err := c.PatchStatus(ctx, tok, kube.ScalewaySecrets, "ns", "sec", []byte(`{"status":{"a":null}}`)); err != nil {
		t.Fatalf("null removal: %v", err)
	}
	m = mustObject(t, s, kube.ScalewaySecrets, "ns", "sec")
	if _, ok := field(m, "status", "a"); ok {
		t.Errorf("null removal: status.a still present")
	}
	if got := stringField(t, m, "status", "b"); got != "kept" {
		t.Errorf("null removal: status.b = %q, want kept", got)
	}
}

// Behavior 7: resourceVersion is a decimal string, bumped on each change.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestResourceVersionBumps(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"x":0}`))
	c := newClient(t, s)

	prev := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i"))
	for i := 1; i <= 3; i++ {
		out, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"spec":{"x":`+strconv.Itoa(i)+`}}`))
		if err != nil {
			t.Fatalf("MergePatch %d: %v", i, err)
		}
		rv := resourceVersion(t, out)
		if rv <= prev {
			t.Errorf("patch %d: resourceVersion %d not above %d", i, rv, prev)
		}
		if stored := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i")); stored != rv {
			t.Errorf("patch %d: stored resourceVersion %d, response %d", i, stored, rv)
		}
		prev = rv
	}

	out, err := c.PatchStatus(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"status":{"y":1}}`))
	if err != nil {
		t.Fatalf("PatchStatus: %v", err)
	}
	if rv := resourceVersion(t, out); rv <= prev {
		t.Errorf("status patch: resourceVersion %d not above %d", rv, prev)
	}

	// Put of another object also takes the next value of the global counter.
	before := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i"))
	s.Put(kube.Instances, instance("ns", "j", ""))
	if rv := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "j")); rv <= before {
		t.Errorf("Put: resourceVersion %d not above %d", rv, before)
	}
}

// Behavior 8: a patch that changes nothing keeps resourceVersion.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestNoOpPatchKeepsResourceVersion(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"x":1},"status":{"a":"b"}`))
	c := newClient(t, s)
	before := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i"))

	out, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"spec":{"x":1}}`))
	if err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	if rv := resourceVersion(t, out); rv != before {
		t.Errorf("no-op object patch: resourceVersion %d, want %d", rv, before)
	}
	out, err = c.PatchStatus(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"status":{"a":"b"}}`))
	if err != nil {
		t.Fatalf("PatchStatus: %v", err)
	}
	if rv := resourceVersion(t, out); rv != before {
		t.Errorf("no-op status patch: resourceVersion %d, want %d", rv, before)
	}
	m := mustObject(t, s, kube.Instances, "ns", "i")
	if got := numberField(t, m, "metadata", "generation"); got != 1 {
		t.Errorf("generation = %v, want 1", got)
	}
}

// Behavior 9: a patch carrying a stale metadata.resourceVersion gets 409, on both endpoints.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestStaleResourceVersionIs409(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"x":0}`))
	c := newClient(t, s)
	stale := strconv.FormatInt(resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i")), 10)

	if _, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"spec":{"x":1}}`)); err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	current := mustRaw(t, s, kube.Instances, "ns", "i")

	_, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i",
		[]byte(`{"metadata":{"resourceVersion":"`+stale+`"},"spec":{"x":2}}`))
	ae := apiErr(t, err)
	if ae.Code != http.StatusConflict {
		t.Fatalf("object patch: code %d, want 409", ae.Code)
	}
	assertStatusBody(t, ae.Body, http.StatusConflict, "Conflict")

	_, err = c.PatchStatus(ctx, kube.Token(testToken), kube.Instances, "ns", "i",
		[]byte(`{"metadata":{"resourceVersion":"`+stale+`"},"status":{"a":1}}`))
	if code := apiCode(t, err); code != http.StatusConflict {
		t.Errorf("status patch: code %d, want 409", code)
	}

	if got := mustRaw(t, s, kube.Instances, "ns", "i"); !reflect.DeepEqual(decode(t, got), decode(t, current)) {
		t.Errorf("object changed by a rejected patch: %s, want %s", got, current)
	}

	// The current resourceVersion is accepted, and never stored from the patch.
	cur := strconv.FormatInt(resourceVersion(t, current), 10)
	out, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i",
		[]byte(`{"metadata":{"resourceVersion":"`+cur+`"},"spec":{"x":3}}`))
	if err != nil {
		t.Fatalf("patch with current resourceVersion: %v", err)
	}
	if rv := resourceVersion(t, out); strconv.FormatInt(rv, 10) == cur {
		t.Errorf("resourceVersion not bumped after a change")
	}
}

// Put sets generation to 1 when absent and keeps a given one; patch ignores metadata.name and namespace.
func TestPutGenerationAndPatchIdentity(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "a", ""))
	s.Put(kube.Instances, json.RawMessage(`{"metadata":{"namespace":"ns","name":"b","generation":5}}`))
	if got := numberField(t, mustObject(t, s, kube.Instances, "ns", "a"), "metadata", "generation"); got != 1 {
		t.Errorf("a: generation = %v, want 1", got)
	}
	if got := numberField(t, mustObject(t, s, kube.Instances, "ns", "b"), "metadata", "generation"); got != 5 {
		t.Errorf("b: generation = %v, want 5", got)
	}

	c := newClient(t, s)
	if _, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "a",
		[]byte(`{"metadata":{"name":"renamed","namespace":"other"}}`)); err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	m := mustObject(t, s, kube.Instances, "ns", "a")
	if got := stringField(t, m, "metadata", "name"); got != "a" {
		t.Errorf("name = %q, want a", got)
	}
	if got := stringField(t, m, "metadata", "namespace"); got != "ns" {
		t.Errorf("namespace = %q, want ns", got)
	}
	if _, ok := s.Object(kube.Instances, "other", "renamed"); ok {
		t.Errorf("patch created other/renamed")
	}

	// Put replaces.
	s.Put(kube.Instances, instance("ns", "a", `"spec":{"z":true}`))
	m = mustObject(t, s, kube.Instances, "ns", "a")
	if v, _ := field(m, "spec", "z"); v != true {
		t.Errorf("after replace: spec.z = %v, want true", v)
	}
}

// Large integers survive the store untouched (decoded with UseNumber, re-encoded from the map).
func TestNumbersKeptExactly(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", `"spec":{"big":9007199254740993}`))
	raw := mustRaw(t, s, kube.Instances, "ns", "i")
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	v, _ := field(m, "spec", "big")
	if n, ok := v.(json.Number); !ok || n.String() != "9007199254740993" {
		t.Errorf("spec.big = %v, want 9007199254740993", v)
	}
}

func mustRaw(t *testing.T, s *kubetest.Server, r kube.Resource, namespace, name string) []byte {
	t.Helper()
	raw, ok := s.Object(r, namespace, name)
	if !ok {
		t.Fatalf("Object(%v, %q, %q): absent", r, namespace, name)
	}
	return raw
}
