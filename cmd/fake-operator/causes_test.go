package main

// Tests of story 5 (CAP-6): causes by annotation. Every helper and type here
// carries the "causes" prefix so this file does not clash with the test files
// of the other stories in package main.

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

const (
	causesNS        = "causes"
	causesProjectID = "0b9c6f2e-6d1a-4c55-9a43-2f0e8c1d7a10"
	causesToken     = "causes-token"
	causesID        = "11111111-2222-4333-8444-555555555555"

	causesOutcomeKey = "fake.scaleway-manager.test/outcome"
	causesMessageKey = "fake.scaleway-manager.test/message"
	causesSourceKey  = "fake.scaleway-manager.test/source-version"

	causesAPIErrorMsg    = "Scaleway API error: 500 Internal Server Error"
	causesRevokedMsg     = "Source Secret opt-in revoked — Scaleway version disabled. Re-add label/annotation to re-sync."
	causesUnavailableMsg = "Source Secret unavailable (see operator logs)"
)

var causesNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

type causesClock struct{}

func (causesClock) Now() time.Time { return causesNow }

type causesEnv struct {
	s *kubetest.Server
	o *operator
}

// causesSetup starts kubetest with a namespace and a NamespaceRole that pass
// every check of story 4, and builds an operator with a fixed clock and a
// seeded rand.
func causesSetup(t *testing.T) *causesEnv {
	t.Helper()
	s := kubetest.New(t)
	s.ExpectToken(causesToken)
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte(causesToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		switch k {
		case "KUBERNETES_SERVICE_HOST":
			return s.Host
		case "KUBERNETES_SERVICE_PORT":
			return s.Port
		}
		return ""
	}
	c, err := kube.NewInCluster(getenv, s.CAPEM)
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	s.Put(kube.Namespaces, causesJSON(t, map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name":        causesNS,
			"annotations": map[string]any{"scaleway.mathieubodin.io/project-id": causesProjectID},
		},
	}))
	s.Put(kube.NamespaceRoles, causesJSON(t, map[string]any{
		"apiVersion": "scaleway.mathieubodin.io/v1",
		"kind":       "NamespaceRole",
		"metadata":   map[string]any{"name": causesNS},
		"spec":       map[string]any{"namespace": causesNS, "scaleway_role": "Editor"},
	}))
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	o := newOperator(c, causesClock{}, rand.New(rand.NewPCG(5, 42)), tokenPath, log)
	return &causesEnv{s: s, o: o}
}

func causesJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func causesSpec(k kind) map[string]any {
	switch k.resource {
	case kube.Instances:
		return map[string]any{"name": "vm", "zone": "fr-par-1", "image": "ubuntu_jammy", "instance_type": "DEV1-S"}
	case kube.LoadBalancers:
		return map[string]any{"name": "lb", "zone": "fr-par-1", "lb_type": "LB-S"}
	default:
		return map[string]any{
			"name":   "sec",
			"region": "fr-par",
			"source": map[string]any{"kubernetes_secret": map[string]any{"name": "src", "key": "value"}},
		}
	}
}

// causesObject builds an object of kind k named name. A nil status means no
// status; withFinalizer adds the kind's finalizer.
func causesObject(k kind, name string, generation int, annotations map[string]string, withFinalizer bool, status map[string]any) map[string]any {
	md := map[string]any{"name": name, "namespace": causesNS, "generation": generation}
	if len(annotations) > 0 {
		a := map[string]any{}
		for key, v := range annotations {
			a[key] = v
		}
		md["annotations"] = a
	}
	if withFinalizer {
		md["finalizers"] = []any{k.finalizer}
	}
	obj := map[string]any{
		"apiVersion": "scaleway.mathieubodin.io/v1",
		"kind":       k.name,
		"metadata":   md,
		"spec":       causesSpec(k),
	}
	if status != nil {
		obj["status"] = status
	}
	return obj
}

func (e *causesEnv) put(t *testing.T, k kind, obj map[string]any) {
	t.Helper()
	e.s.Put(k.resource, causesJSON(t, obj))
}

// get returns the stored object decoded with UseNumber, as the engine decodes it.
func (e *causesEnv) get(t *testing.T, k kind, name string) map[string]any {
	t.Helper()
	raw, ok := e.s.Object(k.resource, causesNS, name)
	if !ok {
		t.Fatalf("%s %s/%s not found", k.name, causesNS, name)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var obj map[string]any
	if err := d.Decode(&obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

// setAnnotations replaces the annotations of the stored object, as a user would.
func (e *causesEnv) setAnnotations(t *testing.T, k kind, name string, annotations map[string]string) {
	t.Helper()
	obj := e.get(t, k, name)
	md := obj["metadata"].(map[string]any)
	delete(md, "resourceVersion")
	if len(annotations) == 0 {
		delete(md, "annotations")
	} else {
		a := map[string]any{}
		for key, v := range annotations {
			a[key] = v
		}
		md["annotations"] = a
	}
	e.put(t, k, obj)
}

func (e *causesEnv) pass(t *testing.T) {
	t.Helper()
	if err := e.o.pass(t.Context()); err != nil {
		t.Fatalf("pass: %v", err)
	}
}

// passPatches runs one pass and returns the PATCH requests it sent.
func (e *causesEnv) passPatches(t *testing.T) []kubetest.Request {
	t.Helper()
	before := len(e.s.Requests())
	e.pass(t)
	var out []kubetest.Request
	for _, r := range e.s.Requests()[before:] {
		if r.Method == "PATCH" {
			out = append(out, r)
		}
	}
	return out
}

func (e *causesEnv) status(t *testing.T, k kind, name string) map[string]any {
	t.Helper()
	st, _ := e.get(t, k, name)["status"].(map[string]any)
	return causesNorm(t, st)
}

func (e *causesEnv) finalizers(t *testing.T, k kind, name string) []any {
	t.Helper()
	md := e.get(t, k, name)["metadata"].(map[string]any)
	f, _ := md["finalizers"].([]any)
	return f
}

// causesNorm round-trips v through JSON (numbers become float64) and drops
// null-valued keys, since a merge patch deletes them.
func causesNorm(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	out := map[string]any{}
	if v == nil {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(causesJSON(t, v), &m); err != nil {
		t.Fatal(err)
	}
	for key, val := range m {
		if val != nil {
			out[key] = val
		}
	}
	return out
}

func causesWant(t *testing.T, got, want map[string]any) {
	t.Helper()
	w := causesNorm(t, want)
	if !reflect.DeepEqual(got, w) {
		t.Errorf("status\n got: %s\nwant: %s", causesJSON(t, got), causesJSON(t, w))
	}
}

func causesNoPatch(t *testing.T, patches []kubetest.Request) {
	t.Helper()
	if len(patches) != 0 {
		var b strings.Builder
		for _, p := range patches {
			b.WriteString(p.RequestURI + " " + string(p.Body) + "\n")
		}
		t.Errorf("want no PATCH, got %d:\n%s", len(patches), b.String())
	}
}

func causesSyncedInstance() map[string]any {
	return map[string]any{
		"scaleway_id": causesID, "public_ip": "192.0.2.7", "state": "running",
		"project_id": causesProjectID, "created_at": "2026-10-01T00:00:00Z",
		"error_message": nil, "sync_state": "Synced",
	}
}

func causesSyncedLB() map[string]any {
	return map[string]any{
		"scaleway_id": causesID, "vip_address": "198.51.100.7", "state": "ready",
		"project_id": causesProjectID, "created_at": nil,
		"error_message": nil, "sync_state": "Synced",
	}
}

func causesSyncedSecret() map[string]any {
	return map[string]any{
		"scaleway_id": causesID, "current_version": 2, "last_synced_resource_version": nil,
		"observed_generation": 1, "sync_state": "Synced", "error_message": nil,
	}
}

func causesSynced(k kind) map[string]any {
	switch k.resource {
	case kube.Instances:
		return causesSyncedInstance()
	case kube.LoadBalancers:
		return causesSyncedLB()
	default:
		return causesSyncedSecret()
	}
}

func causesWith(base map[string]any, kv ...any) map[string]any {
	out := map[string]any{}
	for key, v := range base {
		out[key] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

func causesOutcome(v string) map[string]string { return map[string]string{causesOutcomeKey: v} }

// --- Annotation keys ---

func TestCausesAnnotationKeys(t *testing.T) {
	if outcomeAnnotation != causesOutcomeKey {
		t.Errorf("outcomeAnnotation = %q", outcomeAnnotation)
	}
	if messageAnnotation != causesMessageKey {
		t.Errorf("messageAnnotation = %q", messageAnnotation)
	}
	if sourceVersionAnnotation != causesSourceKey {
		t.Errorf("sourceVersionAnnotation = %q", sourceVersionAnnotation)
	}
}

// --- NotFound ---

func TestCausesNotFoundInstanceKept(t *testing.T) {
	e := causesSetup(t)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, causesOutcome("NotFound"), true, causesSyncedInstance()))

	patches := e.passPatches(t)
	if len(patches) != 1 {
		t.Fatalf("pass 1: want 1 PATCH, got %d", len(patches))
	}
	if !strings.HasSuffix(patches[0].RequestURI, "/status") {
		t.Errorf("pass 1: PATCH on %s, want /status", patches[0].RequestURI)
	}
	var body struct {
		Status map[string]any `json:"status"`
	}
	if err := json.Unmarshal(patches[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"scaleway_id", "public_ip", "project_id", "created_at", "error_message"} {
		v, ok := body.Status[f]
		if !ok || v != nil {
			t.Errorf("patch body: %s = %v (present %v), want null", f, v, ok)
		}
	}
	causesWant(t, e.status(t, instanceKind, "vm"), map[string]any{"state": "unknown", "sync_state": "Syncing"})

	causesNoPatch(t, e.passPatches(t))
	causesWant(t, e.status(t, instanceKind, "vm"), map[string]any{"state": "unknown", "sync_state": "Syncing"})
}

func TestCausesNotFoundLoadBalancerKept(t *testing.T) {
	e := causesSetup(t)
	e.put(t, loadBalancerKind, causesObject(loadBalancerKind, "lb", 1, causesOutcome("NotFound"), true, causesSyncedLB()))

	e.pass(t)
	causesWant(t, e.status(t, loadBalancerKind, "lb"), map[string]any{"state": "", "sync_state": "Pending"})

	causesNoPatch(t, e.passPatches(t))
}

func TestCausesNotFoundRemovedInstanceRecreated(t *testing.T) {
	e := causesSetup(t)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, causesOutcome("NotFound"), true, causesSyncedInstance()))
	e.pass(t)
	e.setAnnotations(t, instanceKind, "vm", nil)

	e.pass(t)
	st := e.status(t, instanceKind, "vm")
	if st["sync_state"] != "Syncing" || st["state"] != "creating" {
		t.Errorf("pass after removal: sync_state %v, state %v; want Syncing, creating", st["sync_state"], st["state"])
	}
	id, _ := st["scaleway_id"].(string)
	if id == "" || id == causesID {
		t.Errorf("scaleway_id = %q, want a new generated id", id)
	}
	if st["project_id"] != causesProjectID {
		t.Errorf("project_id = %v", st["project_id"])
	}
	if st["created_at"] != causesNow.Format(time.RFC3339) {
		t.Errorf("created_at = %v", st["created_at"])
	}

	e.pass(t)
	st = e.status(t, instanceKind, "vm")
	if st["sync_state"] != "Synced" || st["state"] != "running" {
		t.Errorf("next pass: sync_state %v, state %v; want Synced, running", st["sync_state"], st["state"])
	}
	if ip, _ := st["public_ip"].(string); !strings.HasPrefix(ip, "192.0.2.") {
		t.Errorf("public_ip = %v", st["public_ip"])
	}
	if st["scaleway_id"] != id {
		t.Errorf("scaleway_id changed: %v, want %v", st["scaleway_id"], id)
	}
}

func TestCausesNotFoundRemovedLoadBalancerRecreated(t *testing.T) {
	e := causesSetup(t)
	e.put(t, loadBalancerKind, causesObject(loadBalancerKind, "lb", 1, causesOutcome("NotFound"), true, causesSyncedLB()))
	e.pass(t)
	e.setAnnotations(t, loadBalancerKind, "lb", nil)

	e.pass(t)
	st := e.status(t, loadBalancerKind, "lb")
	if st["sync_state"] != "Syncing" || st["state"] != "pending" {
		t.Errorf("pass after removal: sync_state %v, state %v; want Syncing, pending", st["sync_state"], st["state"])
	}
	if id, _ := st["scaleway_id"].(string); id == "" || id == causesID {
		t.Errorf("scaleway_id = %q, want a new generated id", id)
	}

	e.pass(t)
	st = e.status(t, loadBalancerKind, "lb")
	if st["sync_state"] != "Synced" || st["state"] != "ready" {
		t.Errorf("next pass: sync_state %v, state %v; want Synced, ready", st["sync_state"], st["state"])
	}
	if ip, _ := st["vip_address"].(string); !strings.HasPrefix(ip, "198.51.100.") {
		t.Errorf("vip_address = %v", st["vip_address"])
	}
}

// --- Error ---

func TestCausesErrorInstanceAndLoadBalancer(t *testing.T) {
	for _, k := range []kind{instanceKind, loadBalancerKind} {
		t.Run(k.name, func(t *testing.T) {
			e := causesSetup(t)
			e.put(t, k, causesObject(k, "obj", 1, causesOutcome("Error"), true, causesSynced(k)))

			e.pass(t)
			causesWant(t, e.status(t, k, "obj"),
				causesWith(causesSynced(k), "sync_state", "Error", "error_message", causesAPIErrorMsg))

			causesNoPatch(t, e.passPatches(t))
		})
	}
}

func TestCausesErrorSecretAtCreation(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, causesOutcome("Error"), true, nil))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"),
		map[string]any{"sync_state": "Error", "error_message": causesAPIErrorMsg})

	causesNoPatch(t, e.passPatches(t))
}

func TestCausesErrorSyncedSecretNoPatch(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, causesOutcome("Error"), true, causesSyncedSecret()))

	causesNoPatch(t, e.passPatches(t))
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesSyncedSecret())
}

// With a scaleway_id, Error has no effect whatever the sync_state, and it is
// handled: the lifecycle does not recover the secret to Synced.
func TestCausesErrorSecretWithIDNotSyncedNoPatch(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "sync_state", "Error", "error_message", "earlier")
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, causesOutcome("Error"), true, st))

	causesNoPatch(t, e.passPatches(t))
}

// --- Locked ---

func TestCausesLockedLoadBalancerOutsideDeletion(t *testing.T) {
	e := causesSetup(t)
	e.put(t, loadBalancerKind, causesObject(loadBalancerKind, "lb", 1, causesOutcome("Locked"), true, causesSyncedLB()))

	causesNoPatch(t, e.passPatches(t))
	causesWant(t, e.status(t, loadBalancerKind, "lb"), causesSyncedLB())
}

// Locked outside deletion is not handled: the normal lifecycle runs.
func TestCausesLockedLoadBalancerLifecycleRuns(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedLB(), "sync_state", "Syncing", "state", "pending", "vip_address", nil)
	e.put(t, loadBalancerKind, causesObject(loadBalancerKind, "lb", 1, causesOutcome("Locked"), true, st))

	e.pass(t)
	got := e.status(t, loadBalancerKind, "lb")
	if got["sync_state"] != "Synced" || got["state"] != "ready" {
		t.Errorf("sync_state %v, state %v; want Synced, ready", got["sync_state"], got["state"])
	}
}

// --- Revoked ---

func TestCausesRevokedWithID(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, causesOutcome("Revoked"), true, causesSyncedSecret()))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), map[string]any{
		"scaleway_id": causesID, "current_version": 2, "observed_generation": 1,
		"sync_state": "Revoked", "error_message": causesRevokedMsg,
	})

	causesNoPatch(t, e.passPatches(t))
}

func TestCausesRevokedWithoutIDNoFinalizer(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, causesOutcome("Revoked"), false, nil))

	patches := e.passPatches(t)
	for _, p := range patches {
		if !strings.HasSuffix(p.RequestURI, "/status") {
			t.Errorf("pass 1: unexpected object PATCH %s %s", p.RequestURI, p.Body)
		}
	}
	causesWant(t, e.status(t, scalewaySecretKind, "sec"),
		map[string]any{"sync_state": "Error", "error_message": causesUnavailableMsg})
	if f := e.finalizers(t, scalewaySecretKind, "sec"); len(f) != 0 {
		t.Errorf("finalizers = %v, want none", f)
	}

	causesNoPatch(t, e.passPatches(t))
	if f := e.finalizers(t, scalewaySecretKind, "sec"); len(f) != 0 {
		t.Errorf("pass 2: finalizers = %v, want none", f)
	}
}

func TestCausesRevokedBeforeFinalizer(t *testing.T) {
	withID := map[string]any{"scaleway_id": causesID, "sync_state": "Synced"}
	nullID := map[string]any{"scaleway_id": nil, "sync_state": "Syncing"}
	cases := []struct {
		name   string
		k      kind
		ann    map[string]string
		status map[string]any
		want   bool
	}{
		{"secret no status", scalewaySecretKind, causesOutcome("Revoked"), nil, true},
		{"secret null id", scalewaySecretKind, causesOutcome("Revoked"), nullID, true},
		{"secret with id", scalewaySecretKind, causesOutcome("Revoked"), withID, false},
		{"secret no cause", scalewaySecretKind, nil, nil, false},
		{"secret empty outcome", scalewaySecretKind, causesOutcome(""), nil, false},
		{"secret other cause", scalewaySecretKind, causesOutcome("Error"), nil, false},
		{"instance revoked", instanceKind, causesOutcome("Revoked"), nil, false},
		{"loadbalancer revoked", loadBalancerKind, causesOutcome("Revoked"), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := causesJSON(t, causesObject(c.k, "obj", 1, c.ann, false, c.status))
			d := json.NewDecoder(bytes.NewReader(raw))
			d.UseNumber()
			var obj map[string]any
			if err := d.Decode(&obj); err != nil {
				t.Fatal(err)
			}
			if got := revokedBeforeFinalizer(c.k, obj); got != c.want {
				t.Errorf("revokedBeforeFinalizer = %v, want %v", got, c.want)
			}
		})
	}
}

// --- Refused causes ---

func TestCausesRefused(t *testing.T) {
	cases := []struct {
		k     kind
		cause string
	}{
		{instanceKind, "Foo"},
		{loadBalancerKind, "Foo"},
		{scalewaySecretKind, "Foo"},
		{scalewaySecretKind, "NotFound"},
		{instanceKind, "Locked"},
		{scalewaySecretKind, "Locked"},
		{instanceKind, "Revoked"},
		{loadBalancerKind, "Revoked"},
	}
	for _, c := range cases {
		t.Run(c.k.name+"/"+c.cause, func(t *testing.T) {
			e := causesSetup(t)
			e.put(t, c.k, causesObject(c.k, "obj", 1, causesOutcome(c.cause), true, causesSynced(c.k)))

			e.pass(t)
			got := e.status(t, c.k, "obj")
			want := "fake-operator: cause '" + c.cause + "' is not supported for " + c.k.name
			if got["sync_state"] != "Error" || got["error_message"] != want {
				t.Errorf("sync_state %v, error_message %v; want Error, %q", got["sync_state"], got["error_message"], want)
			}
			if got["scaleway_id"] != causesID {
				t.Errorf("scaleway_id = %v, want kept", got["scaleway_id"])
			}

			causesNoPatch(t, e.passPatches(t))
		})
	}
}

func TestCausesEmptyOutcomeIsAbsent(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedInstance(), "sync_state", "Syncing", "state", "creating", "public_ip", nil)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, causesOutcome(""), true, st))

	e.pass(t)
	got := e.status(t, instanceKind, "vm")
	if got["sync_state"] != "Synced" || got["state"] != "running" {
		t.Errorf("sync_state %v, state %v; want Synced, running", got["sync_state"], got["state"])
	}
	if _, ok := got["error_message"]; ok {
		t.Errorf("error_message = %v, want null", got["error_message"])
	}
}

// --- message annotation ---

func TestCausesMessageReplacesError(t *testing.T) {
	cases := []struct {
		k      kind
		status map[string]any
	}{
		{instanceKind, causesSyncedInstance()},
		{loadBalancerKind, causesSyncedLB()},
		{scalewaySecretKind, nil}, // Error on a secret applies at creation only
	}
	for _, c := range cases {
		t.Run(c.k.name, func(t *testing.T) {
			e := causesSetup(t)
			ann := map[string]string{causesOutcomeKey: "Error", causesMessageKey: "boom"}
			e.put(t, c.k, causesObject(c.k, "obj", 1, ann, true, c.status))

			e.pass(t)
			got := e.status(t, c.k, "obj")
			if got["sync_state"] != "Error" || got["error_message"] != "boom" {
				t.Errorf("sync_state %v, error_message %v; want Error, boom", got["sync_state"], got["error_message"])
			}
		})
	}
}

func TestCausesMessageReplacesRevoked(t *testing.T) {
	e := causesSetup(t)
	ann := map[string]string{causesOutcomeKey: "Revoked", causesMessageKey: "boom"}
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, ann, true, causesSyncedSecret()))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), map[string]any{
		"scaleway_id": causesID, "current_version": 2, "observed_generation": 1,
		"sync_state": "Revoked", "error_message": "boom",
	})
}

func TestCausesMessageReplacesRevokedWithoutID(t *testing.T) {
	e := causesSetup(t)
	ann := map[string]string{causesOutcomeKey: "Revoked", causesMessageKey: "boom"}
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, ann, false, nil))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), map[string]any{"sync_state": "Error", "error_message": "boom"})
	if f := e.finalizers(t, scalewaySecretKind, "sec"); len(f) != 0 {
		t.Errorf("finalizers = %v, want none", f)
	}
}

func TestCausesMessageIgnoredWithoutCause(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedInstance(), "sync_state", "Syncing", "state", "creating", "public_ip", nil)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, map[string]string{causesMessageKey: "boom"}, true, st))

	e.pass(t)
	got := e.status(t, instanceKind, "vm")
	if got["sync_state"] != "Synced" {
		t.Errorf("sync_state = %v, want Synced", got["sync_state"])
	}
	if _, ok := got["error_message"]; ok {
		t.Errorf("error_message = %v, want null", got["error_message"])
	}
}

func TestCausesMessageIgnoredWithNotFound(t *testing.T) {
	e := causesSetup(t)
	ann := map[string]string{causesOutcomeKey: "NotFound", causesMessageKey: "boom"}
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, ann, true, causesSyncedInstance()))

	e.pass(t)
	causesWant(t, e.status(t, instanceKind, "vm"), map[string]any{"state": "unknown", "sync_state": "Syncing"})
}

func TestCausesMessageIgnoredWithRefusedCause(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := causesSetup(t)
			ann := map[string]string{causesOutcomeKey: "Foo", causesMessageKey: "boom"}
			e.put(t, k, causesObject(k, "obj", 1, ann, true, causesSynced(k)))

			e.pass(t)
			got := e.status(t, k, "obj")
			want := "fake-operator: cause 'Foo' is not supported for " + k.name
			if got["error_message"] != want {
				t.Errorf("error_message = %v, want %q", got["error_message"], want)
			}
		})
	}
}

// --- source-version ---

func TestCausesSourceVersionSyncedSecret(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "3"}, true, causesSyncedSecret()))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "current_version", 3))

	causesNoPatch(t, e.passPatches(t))
}

func TestCausesSourceVersionNewSecret(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "3"}, true, nil))

	e.pass(t)
	st := e.status(t, scalewaySecretKind, "sec")
	if st["sync_state"] != "Synced" || st["current_version"] != float64(1) {
		t.Errorf("pass 1: sync_state %v, current_version %v; want Synced, 1", st["sync_state"], st["current_version"])
	}
	id := st["scaleway_id"]

	e.pass(t)
	st = e.status(t, scalewaySecretKind, "sec")
	if st["sync_state"] != "Synced" || st["current_version"] != float64(3) {
		t.Errorf("pass 2: sync_state %v, current_version %v; want Synced, 3", st["sync_state"], st["current_version"])
	}
	if st["scaleway_id"] != id {
		t.Errorf("scaleway_id changed: %v, want %v", st["scaleway_id"], id)
	}
}

// source-version equal to current_version is not handled: the lifecycle runs.
func TestCausesSourceVersionEqualLetsLifecycleRun(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 2, map[string]string{causesSourceKey: "2"}, true, causesSyncedSecret()))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"),
		causesWith(causesSyncedSecret(), "current_version", 3, "observed_generation", 2))
}

// source-version takes precedence over a generation change in the same pass,
// and its write sets observed_generation to metadata.generation.
func TestCausesSourceVersionBeforeGenerationPush(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 2, map[string]string{causesSourceKey: "5"}, true, causesSyncedSecret()))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"),
		causesWith(causesSyncedSecret(), "current_version", 5, "observed_generation", 2))

	causesNoPatch(t, e.passPatches(t))
}

// A valid source-version waits for Synced.
func TestCausesSourceVersionWaitsForSynced(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "sync_state", "Error", "error_message", "earlier")
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "3"}, true, st))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesSyncedSecret())

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "current_version", 3))
}

func TestCausesSourceVersionInvalid(t *testing.T) {
	for _, v := range []string{"0", "x", "-1"} {
		t.Run("synced/"+v, func(t *testing.T) {
			e := causesSetup(t)
			e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: v}, true, causesSyncedSecret()))

			e.pass(t)
			got := e.status(t, scalewaySecretKind, "sec")
			want := "fake-operator: source-version '" + v + "' is not a positive integer"
			if got["sync_state"] != "Error" || got["error_message"] != want {
				t.Errorf("sync_state %v, error_message %v; want Error, %q", got["sync_state"], got["error_message"], want)
			}
		})
	}
	t.Run("new/x", func(t *testing.T) {
		e := causesSetup(t)
		e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "x"}, true, nil))

		e.pass(t)
		got := e.status(t, scalewaySecretKind, "sec")
		want := "fake-operator: source-version 'x' is not a positive integer"
		if got["sync_state"] != "Error" || got["error_message"] != want {
			t.Errorf("sync_state %v, error_message %v; want Error, %q", got["sync_state"], got["error_message"], want)
		}
	})
}

func TestCausesSourceVersionIgnoredOnOtherKinds(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedInstance(), "sync_state", "Syncing", "state", "creating", "public_ip", nil)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, map[string]string{causesSourceKey: "x"}, true, st))

	e.pass(t)
	got := e.status(t, instanceKind, "vm")
	if got["sync_state"] != "Synced" || got["state"] != "running" {
		t.Errorf("sync_state %v, state %v; want Synced, running", got["sync_state"], got["state"])
	}
}

// A cause takes precedence over source-version.
func TestCausesCauseBeforeSourceVersion(t *testing.T) {
	e := causesSetup(t)
	ann := map[string]string{causesOutcomeKey: "Revoked", causesSourceKey: "9"}
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, ann, true, causesSyncedSecret()))

	e.pass(t)
	got := e.status(t, scalewaySecretKind, "sec")
	if got["sync_state"] != "Revoked" || got["current_version"] != float64(2) {
		t.Errorf("sync_state %v, current_version %v; want Revoked, 2", got["sync_state"], got["current_version"])
	}
}

// --- No memory: removing the cause lets the lifecycle resume ---

func TestCausesErrorRemovedLifecycleResumes(t *testing.T) {
	e := causesSetup(t)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, causesOutcome("Error"), true, causesSyncedInstance()))
	e.pass(t)
	e.setAnnotations(t, instanceKind, "vm", nil)

	e.pass(t)
	got := e.status(t, instanceKind, "vm")
	if got["sync_state"] != "Synced" || got["scaleway_id"] != causesID {
		t.Errorf("sync_state %v, scaleway_id %v; want Synced, %s", got["sync_state"], got["scaleway_id"], causesID)
	}
	if _, ok := got["error_message"]; ok {
		t.Errorf("error_message = %v, want null", got["error_message"])
	}
}

// The fake never writes annotations: the outcome annotation stays.
func TestCausesAnnotationNeverRemoved(t *testing.T) {
	e := causesSetup(t)
	e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, causesOutcome("NotFound"), true, causesSyncedInstance()))
	patches := e.passPatches(t)
	patches = append(patches, e.passPatches(t)...)
	for _, p := range patches {
		if bytes.Contains(p.Body, []byte("annotations")) {
			t.Errorf("PATCH %s touches annotations: %s", p.RequestURI, p.Body)
		}
	}
	md := e.get(t, instanceKind, "vm")["metadata"].(map[string]any)
	a, _ := md["annotations"].(map[string]any)
	if a[causesOutcomeKey] != "NotFound" {
		t.Errorf("annotations = %v, want outcome kept", a)
	}
}
