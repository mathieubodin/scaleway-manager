package main

// Story 3 (#42), CAP-7: the fake writes like the operator and never more than
// needed. Helpers come from lifecycle_test.go (prefix eng).

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// engStatusKeys is the whole status struct of each kind, from the companion's
// "Status fields".
var engStatusKeys = map[string][]string{
	"instances":       {"created_at", "error_message", "project_id", "public_ip", "scaleway_id", "state", "sync_state"},
	"loadbalancers":   {"created_at", "error_message", "project_id", "scaleway_id", "state", "sync_state", "vip_address"},
	"scalewaysecrets": {"current_version", "error_message", "last_synced_resource_version", "observed_generation", "scaleway_id", "sync_state"},
}

func engKeys(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// engRunToSynced runs the three passes that take a new object of each kind to Synced.
func engRunToSynced(e *engEnv) {
	for i := 0; i < 3; i++ {
		e.pass()
	}
}

// TestWritesPatchBodies: a finalizer patch is exactly
// {"metadata":{"finalizers":[<whole list>]}}, a status patch exactly
// {"status":{<whole struct>}}, both merge patches with no resourceVersion.
func TestWritesPatchBodies(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	engPutLB(e, "front")
	engPutSecret(e, "db-password")
	engRunToSynced(e)

	patches := e.patchesSince(0)
	if len(patches) == 0 {
		t.Fatal("no PATCH sent")
	}
	var finalizerPatches, statusPatches int
	for _, r := range patches {
		if r.ContentType != "application/merge-patch+json" {
			t.Errorf("%s: content type %q", r.RequestURI, r.ContentType)
		}
		if strings.Contains(string(r.Body), "resourceVersion") {
			t.Errorf("%s: body carries resourceVersion: %s", r.RequestURI, r.Body)
		}
		body := engDecode(t, r.Body)
		plural := ""
		for p := range engStatusKeys {
			if strings.Contains(r.RequestURI, "/"+p+"/") {
				plural = p
			}
		}
		if plural == "" {
			t.Errorf("PATCH to an unexpected resource: %s", r.RequestURI)
			continue
		}
		if engIsStatus(r) {
			statusPatches++
			if got := engKeys(body); !reflect.DeepEqual(got, []string{"status"}) {
				t.Errorf("%s: top-level keys %v, want [status]", r.RequestURI, got)
				continue
			}
			st, _ := body["status"].(map[string]any)
			if got := engKeys(st); !reflect.DeepEqual(got, engStatusKeys[plural]) {
				t.Errorf("%s: status keys %v, want the whole struct %v", r.RequestURI, got, engStatusKeys[plural])
			}
			continue
		}
		finalizerPatches++
		if got := engKeys(body); !reflect.DeepEqual(got, []string{"metadata"}) {
			t.Errorf("%s: top-level keys %v, want [metadata]", r.RequestURI, got)
			continue
		}
		md, _ := body["metadata"].(map[string]any)
		if got := engKeys(md); !reflect.DeepEqual(got, []string{"finalizers"}) {
			t.Errorf("%s: metadata keys %v, want [finalizers]", r.RequestURI, got)
		}
		if _, ok := md["finalizers"].([]any); !ok {
			t.Errorf("%s: finalizers is not a list: %s", r.RequestURI, r.Body)
		}
	}
	if finalizerPatches != 3 {
		t.Errorf("finalizer patches = %d, want 3 (one per object)", finalizerPatches)
	}
	// Instance 2, LoadBalancer 2, ScalewaySecret 2.
	if statusPatches != 6 {
		t.Errorf("status patches = %d, want 6", statusPatches)
	}
}

// TestWritesFinalizerWholeList: the finalizer patch carries the whole list,
// other finalizers kept.
func TestWritesFinalizerWholeList(t *testing.T) {
	e := engSetup(t)
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"web","namespace":"`+engNamespace+`","finalizers":["example.com/keep"]},"spec":{"name":"web","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))

	e.pass()
	patches := e.patchesSince(0)
	if len(patches) != 1 || engIsStatus(patches[0]) {
		t.Fatalf("want one finalizer PATCH, got %+v", patches)
	}
	md, _ := engDecode(t, patches[0].Body)["metadata"].(map[string]any)
	list, _ := md["finalizers"].([]any)
	got := map[any]bool{}
	for _, f := range list {
		got[f] = true
	}
	if len(list) != 2 || !got["example.com/keep"] || !got[engInstanceFinalizer] {
		t.Fatalf("finalizer patch list = %v, want example.com/keep and %s", list, engInstanceFinalizer)
	}
	stored := engFinalizers(e.object(kube.Instances, "web"))
	if len(stored) != 2 {
		t.Fatalf("stored finalizers = %v", stored)
	}
}

// TestWritesNoPatchWithoutChange: a pass whose computed object equals the one
// read sends no write request, and resourceVersion stays the same.
func TestWritesNoPatchWithoutChange(t *testing.T) {
	e := engSetup(t)
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"web","namespace":"`+engNamespace+`","finalizers":["`+engInstanceFinalizer+`"]},"spec":{"name":"web","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"},"status":{"scaleway_id":"11111111-2222-4333-8444-555555555555","public_ip":"192.0.2.10","state":"running","project_id":"`+engProjectID+`","created_at":"2026-01-01T00:00:00Z","sync_state":"Synced"}}`))
	e.srv.Put(kube.LoadBalancers, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"LoadBalancer","metadata":{"name":"front","namespace":"`+engNamespace+`","finalizers":["`+engLBFinalizer+`"]},"spec":{"name":"front","zone":"fr-par-1","lb_type":"LB-S"},"status":{"scaleway_id":"21111111-2222-4333-8444-555555555555","vip_address":"198.51.100.10","state":"ready","project_id":"`+engProjectID+`","sync_state":"Synced"}}`))
	e.srv.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"db-password","namespace":"`+engNamespace+`","generation":3,"finalizers":["`+engSecretFinalizer+`"]},"spec":{"name":"db-password","region":"fr-par","source":{"kubernetes_secret":{"name":"src","key":"value"}}},"status":{"scaleway_id":"31111111-2222-4333-8444-555555555555","current_version":4,"observed_generation":3,"sync_state":"Synced"}}`))

	before := map[string]json.RawMessage{}
	for r, name := range map[kube.Resource]string{kube.Instances: "web", kube.LoadBalancers: "front", kube.ScalewaySecrets: "db-password"} {
		raw, _ := e.srv.Object(r, engNamespace, name)
		before[name] = raw
	}
	e.pass()
	for _, r := range e.srv.Requests() {
		if r.Method != "GET" {
			t.Fatalf("write request on a pass with no change: %s %s %s", r.Method, r.RequestURI, r.Body)
		}
	}
	for r, name := range map[kube.Resource]string{kube.Instances: "web", kube.LoadBalancers: "front", kube.ScalewaySecrets: "db-password"} {
		raw, _ := e.srv.Object(r, engNamespace, name)
		if !reflect.DeepEqual(engDecode(t, raw), engDecode(t, before[name])) {
			t.Fatalf("%s changed:\n before %s\n after  %s", name, before[name], raw)
		}
	}
}

// TestWritesNamespaceRoleReadOnly: no request to namespaceroles other than GET.
func TestWritesNamespaceRoleReadOnly(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	engPutLB(e, "front")
	engPutSecret(e, "db-password")
	engRunToSynced(e)
	e.pass()
	for _, r := range e.srv.Requests() {
		if strings.Contains(r.RequestURI, "/namespaceroles") && r.Method != "GET" {
			t.Fatalf("%s %s: only GET is allowed on namespaceroles", r.Method, r.RequestURI)
		}
	}
	raw, ok := e.srv.Object(kube.NamespaceRoles, "", engNamespace)
	if !ok {
		t.Fatal("NamespaceRole gone")
	}
	if rv, _ := engMeta(engDecode(t, raw))["resourceVersion"].(string); rv != "2" {
		t.Fatalf("NamespaceRole resourceVersion = %q, want 2 (unchanged since Put)", rv)
	}
}

// TestWritesNoAnnotationOrLabel: no PATCH body touches metadata.annotations or
// metadata.labels, and the stored ones stay as they were.
func TestWritesNoAnnotationOrLabel(t *testing.T) {
	e := engSetup(t)
	meta := `"labels":{"app":"shop"},"annotations":{"example.com/note":"keep me"}`
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"web","namespace":"`+engNamespace+`",`+meta+`},"spec":{"name":"web","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))
	e.srv.Put(kube.LoadBalancers, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"LoadBalancer","metadata":{"name":"front","namespace":"`+engNamespace+`",`+meta+`},"spec":{"name":"front","zone":"fr-par-1","lb_type":"LB-S"}}`))
	e.srv.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"db-password","namespace":"`+engNamespace+`",`+meta+`},"spec":{"name":"db-password","region":"fr-par","source":{"kubernetes_secret":{"name":"src","key":"value"}}}}`))
	engRunToSynced(e)
	e.pass()

	for _, r := range e.patchesSince(0) {
		md, ok := engDecode(t, r.Body)["metadata"].(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"annotations", "labels"} {
			if _, ok := md[k]; ok {
				t.Fatalf("%s: PATCH body changes metadata.%s: %s", r.RequestURI, k, r.Body)
			}
		}
	}
	wantLabels := map[string]any{"app": "shop"}
	wantAnnotations := map[string]any{"example.com/note": "keep me"}
	for r, name := range map[kube.Resource]string{kube.Instances: "web", kube.LoadBalancers: "front", kube.ScalewaySecrets: "db-password"} {
		md := engMeta(e.object(r, name))
		if !reflect.DeepEqual(md["labels"], wantLabels) || !reflect.DeepEqual(md["annotations"], wantAnnotations) {
			t.Fatalf("%s: labels %v annotations %v changed", name, md["labels"], md["annotations"])
		}
	}
	raw, _ := e.srv.Object(kube.Namespaces, "", engNamespace)
	if rv, _ := engMeta(engDecode(t, raw))["resourceVersion"].(string); rv != "1" {
		t.Fatalf("Namespace resourceVersion = %q, want 1 (unchanged since Put)", rv)
	}
}
