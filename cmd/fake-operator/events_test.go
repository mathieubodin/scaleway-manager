package main

// CAP-7: the fake creates no Event. Creation, cause, check and deletion passes
// send only GET and PATCH, and never touch an events endpoint. Helpers come
// from lifecycle_test.go (prefix eng).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// eventsAssertNone fails on any request that is neither GET nor PATCH, or that
// targets core or events.k8s.io Events.
func eventsAssertNone(t *testing.T, e *engEnv, step string) {
	t.Helper()
	for _, r := range e.srv.Requests() {
		path := strings.SplitN(r.RequestURI, "?", 2)[0]
		if r.Method != "GET" && r.Method != "PATCH" {
			t.Errorf("%s: %s %s: only GET and PATCH are allowed", step, r.Method, r.RequestURI)
		}
		if strings.Contains(path, "/events") || strings.HasPrefix(path, "/apis/events.k8s.io") {
			t.Errorf("%s: %s %s: request to an events endpoint", step, r.Method, r.RequestURI)
		}
	}
}

func TestWritesNoEvent(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	engPutLB(e, "front")
	engPutSecret(e, "db-password")
	// Cause: a supported one and a refused one.
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"broken","namespace":"`+engNamespace+`","annotations":{"`+outcomeAnnotation+`":"`+causeError+`"}},"spec":{"name":"broken","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"refused","namespace":"`+engNamespace+`","annotations":{"`+outcomeAnnotation+`":"`+causeLocked+`"}},"spec":{"name":"refused","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))
	// Check: a ScalewaySecret with no source fails its precondition.
	e.srv.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"no-source","namespace":"`+engNamespace+`"},"spec":{"name":"no-source","region":"fr-par"}}`))

	engRunToSynced(e)
	if got := e.status(kube.Instances, "web")["sync_state"]; got != "Synced" {
		t.Fatalf("web sync_state = %v, want Synced", got)
	}
	if len(e.patchesSince(0)) == 0 {
		t.Fatal("no PATCH sent: the passes did nothing")
	}
	eventsAssertNone(t, e, "creation, cause and check")

	for r, name := range map[kube.Resource]string{kube.Instances: "web", kube.LoadBalancers: "front", kube.ScalewaySecrets: "db-password"} {
		e.srv.Delete(r, engNamespace, name)
	}
	before := len(e.srv.Requests())
	e.pass()
	e.pass()
	if len(e.patchesSince(before)) == 0 {
		t.Fatal("no PATCH sent by the deletion passes")
	}
	if _, ok := e.srv.Object(kube.Instances, engNamespace, "web"); ok {
		t.Fatal("web not deleted")
	}
	eventsAssertNone(t, e, "deletion")
}
