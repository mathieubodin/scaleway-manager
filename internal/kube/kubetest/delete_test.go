package kubetest_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

// Behavior 6: Delete with finalizers sets deletionTimestamp and keeps the object; a patch
// that empties finalizers then removes it; without finalizers Delete removes at once.
// Reproduces: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/
func TestDeleteWithFinalizers(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, json.RawMessage(`{"metadata":{"namespace":"ns","name":"i","finalizers":["scaleway.mathieubodin.io/finalizer"]},"spec":{"x":1}}`))
	before := resourceVersion(t, mustRaw(t, s, kube.Instances, "ns", "i"))

	start := time.Now().UTC().Add(-time.Second)
	s.Delete(kube.Instances, "ns", "i")

	raw, ok := s.Object(kube.Instances, "ns", "i")
	if !ok {
		t.Fatalf("object with finalizers removed by Delete")
	}
	m := decode(t, raw)
	ts := stringField(t, m, "metadata", "deletionTimestamp")
	when, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("deletionTimestamp %q is not RFC 3339: %v", ts, err)
	}
	if when.Before(start.Truncate(time.Second)) || when.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("deletionTimestamp %v not near now", when)
	}
	if rv := resourceVersion(t, raw); rv <= before {
		t.Errorf("Delete: resourceVersion %d not above %d", rv, before)
	}
	if got := numberField(t, m, "metadata", "generation"); got != 1 {
		t.Errorf("Delete changed generation to %v", got)
	}

	// Delete on an object already deleting does nothing.
	s.Delete(kube.Instances, "ns", "i")
	if again := mustRaw(t, s, kube.Instances, "ns", "i"); string(again) != string(raw) {
		t.Errorf("second Delete changed the object: %s, was %s", again, raw)
	}

	// Emptying finalizers removes the object; the response carries its last state.
	c := newClient(t, s)
	out, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"metadata":{"finalizers":[]}}`))
	if err != nil {
		t.Fatalf("MergePatch finalizers: %v", err)
	}
	if _, ok := s.Object(kube.Instances, "ns", "i"); ok {
		t.Errorf("object still present after finalizers emptied")
	}
	last := decode(t, out)
	if got := stringField(t, last, "metadata", "name"); got != "i" {
		t.Errorf("patch response name = %q, want i", got)
	}
	stringField(t, last, "metadata", "deletionTimestamp")
	if _, err := c.Get(ctx, kube.Token(testToken), kube.Instances, "ns", "i"); apiCode(t, err) != 404 {
		t.Errorf("Get after removal: want 404")
	}
}

// Matrix row "finalizers emptied while deleting", via the null form of RFC 7386.
// Reproduces: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/
func TestFinalizersNullWhileDeletingRemoves(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.LoadBalancers, json.RawMessage(`{"metadata":{"namespace":"ns","name":"lb","finalizers":["f"]}}`))
	s.Delete(kube.LoadBalancers, "ns", "lb")
	c := newClient(t, s)
	if _, err := c.MergePatch(ctx, kube.Token(testToken), kube.LoadBalancers, "ns", "lb", []byte(`{"metadata":{"finalizers":null}}`)); err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	if _, ok := s.Object(kube.LoadBalancers, "ns", "lb"); ok {
		t.Errorf("object still present")
	}
}

// Emptying finalizers on an object that is not deleting keeps it.
// Reproduces: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/
func TestFinalizersEmptiedWithoutDeletionKeeps(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, json.RawMessage(`{"metadata":{"namespace":"ns","name":"i","finalizers":["f"]}}`))
	c := newClient(t, s)
	if _, err := c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "i", []byte(`{"metadata":{"finalizers":[]}}`)); err != nil {
		t.Fatalf("MergePatch: %v", err)
	}
	if _, ok := s.Object(kube.Instances, "ns", "i"); !ok {
		t.Errorf("object without deletionTimestamp removed")
	}
}

// Without finalizers, Delete removes the object at once; on an absent object it does nothing.
// Reproduces: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/
func TestDeleteWithoutFinalizers(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns", "i", ""))
	s.Put(kube.Instances, json.RawMessage(`{"metadata":{"namespace":"ns","name":"empty","finalizers":[]}}`))
	for _, name := range []string{"i", "empty"} {
		if _, ok := s.Object(kube.Instances, "ns", name); !ok {
			t.Fatalf("object %s absent after Put", name)
		}
	}
	s.Delete(kube.Instances, "ns", "i")
	s.Delete(kube.Instances, "ns", "empty")
	if _, ok := s.Object(kube.Instances, "ns", "i"); ok {
		t.Errorf("object without finalizers still present")
	}
	if _, ok := s.Object(kube.Instances, "ns", "empty"); ok {
		t.Errorf("object with empty finalizers still present")
	}
	s.Delete(kube.Instances, "ns", "absent")
	s.Delete(kube.NamespaceRoles, "", "absent")
}
