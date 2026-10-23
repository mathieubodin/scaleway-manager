package main

import (
	"context"
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

// Story 4 test helpers. Every package-level name starts with "s4" so it
// cannot clash with the test files of stories 3, 5 and 6 in this package.

const (
	s4Namespace     = "team-a"
	s4ProjectID     = "8c6c5a8e-3f0b-4a8e-9f3c-2b7d1e4a5c6d"
	s4ProjectIDKey  = "scaleway.mathieubodin.io/project-id"
	s4OutcomeKey    = "fake.scaleway-manager.test/outcome"
	s4MessageKey    = "fake.scaleway-manager.test/message"
	s4OtherFinal    = "example.com/keep"
	s4ExistingID    = "0f8fad5b-d9cb-469f-a165-70867728950e"
	s4LockedMessage = "Load balancer is locked — deletion blocked by Scaleway"
	s4RegionMessage = "Configuration error: Invalid region 'xx-yyy'. Use one of: fr-par, nl-ams, pl-waw."
)

type s4Clock struct{ t time.Time }

func (c s4Clock) Now() time.Time { return c.t }

type s4Env struct {
	srv *kubetest.Server
	op  *operator
}

// s4Setup starts kubetest and builds an operator against it, with a token
// file, a fixed clock and a seeded rand.
func s4Setup(t *testing.T) *s4Env {
	t.Helper()
	srv := kubetest.New(t)
	env := map[string]string{
		"KUBERNETES_SERVICE_HOST": srv.Host,
		"KUBERNETES_SERVICE_PORT": srv.Port,
	}
	c, err := kube.NewInCluster(func(k string) string { return env[k] }, srv.CAPEM)
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("s4-token\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	clock := s4Clock{time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}
	rnd := rand.New(rand.NewPCG(4, 4))
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return &s4Env{srv: srv, op: newOperator(c, clock, rnd, tokenPath, log)}
}

func (e *s4Env) pass(t *testing.T) {
	t.Helper()
	if err := e.op.pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
}

func s4Raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// s4PutNamespace seeds the namespace; an empty projectID means no annotation.
func (e *s4Env) putNamespace(t *testing.T, projectID string) {
	t.Helper()
	meta := map[string]any{"name": s4Namespace}
	if projectID != "" {
		meta["annotations"] = map[string]any{s4ProjectIDKey: projectID}
	}
	e.srv.Put(kube.Namespaces, s4Raw(t, map[string]any{
		"apiVersion": "v1", "kind": "Namespace", "metadata": meta,
	}))
}

// putRole seeds the NamespaceRole named after the namespace; role "-" means
// no spec.scaleway_role field.
func (e *s4Env) putRole(t *testing.T, role string) {
	t.Helper()
	spec := map[string]any{"namespace": s4Namespace}
	if role != "-" {
		spec["scaleway_role"] = role
	}
	e.srv.Put(kube.NamespaceRoles, s4Raw(t, map[string]any{
		"apiVersion": "scaleway.mathieubodin.io/v1",
		"kind":       "NamespaceRole",
		"metadata":   map[string]any{"name": s4Namespace},
		"spec":       spec,
	}))
}

// s4Valid seeds a namespace with a UUID project-id and an Editor role.
func (e *s4Env) valid(t *testing.T) {
	t.Helper()
	e.putNamespace(t, s4ProjectID)
	e.putRole(t, "Editor")
}

type s4Obj struct {
	finalizers  []string
	annotations map[string]string
	spec        map[string]any
	status      map[string]any
}

func s4Spec(k kind) map[string]any {
	switch k.resource {
	case kube.Instances:
		return map[string]any{"name": "web", "zone": "fr-par-1", "image": "ubuntu_jammy", "instance_type": "DEV1-S"}
	case kube.LoadBalancers:
		return map[string]any{"name": "lb", "zone": "fr-par-1", "lb_type": "LB-S"}
	default:
		return map[string]any{
			"name":   "db-password",
			"region": "fr-par",
			"source": map[string]any{"kubernetes_secret": map[string]any{"name": "src", "key": "password"}},
		}
	}
}

func s4ObjName(k kind) string { return strings.ToLower(k.name) + "-1" }

func (e *s4Env) putObj(t *testing.T, k kind, o s4Obj) {
	t.Helper()
	meta := map[string]any{"name": s4ObjName(k), "namespace": s4Namespace}
	if o.finalizers != nil {
		meta["finalizers"] = o.finalizers
	}
	if o.annotations != nil {
		meta["annotations"] = o.annotations
	}
	spec := o.spec
	if spec == nil {
		spec = s4Spec(k)
	}
	obj := map[string]any{
		"apiVersion": "scaleway.mathieubodin.io/v1",
		"kind":       k.name,
		"metadata":   meta,
		"spec":       spec,
	}
	if o.status != nil {
		obj["status"] = o.status
	}
	e.srv.Put(k.resource, s4Raw(t, obj))
}

// get returns the stored object, or nil when it is gone.
func (e *s4Env) get(t *testing.T, k kind) map[string]any {
	t.Helper()
	raw, ok := e.srv.Object(k.resource, s4Namespace, s4ObjName(k))
	if !ok {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode object: %v", err)
	}
	return m
}

func s4Finalizers(obj map[string]any) []string {
	meta, _ := obj["metadata"].(map[string]any)
	list, _ := meta["finalizers"].([]any)
	out := []string{}
	for _, f := range list {
		s, _ := f.(string)
		out = append(out, s)
	}
	return out
}

func s4HasFinalizer(obj map[string]any, f string) bool {
	for _, x := range s4Finalizers(obj) {
		if x == f {
			return true
		}
	}
	return false
}

func s4Status(obj map[string]any) map[string]any {
	st, _ := obj["status"].(map[string]any)
	return st
}

// patches returns the PATCH requests on the object k; status selects the
// /status ones (true) or the object ones (false).
func (e *s4Env) patches(k kind, status bool) []kubetest.Request {
	var out []kubetest.Request
	for _, r := range e.srv.Requests() {
		if r.Method != "PATCH" || !strings.Contains(r.RequestURI, "/"+k.resource.String()+"/") {
			continue
		}
		if strings.HasSuffix(strings.SplitN(r.RequestURI, "?", 2)[0], "/status") == status {
			out = append(out, r)
		}
	}
	return out
}

// s4ErrorStatus is the stored status after a first Error on an object with
// no status: the kind's Rust defaults, sync_state Error and the message.
// Null fields are removed by the merge patch, so they are absent here.
func s4ErrorStatus(k kind, msg string) map[string]any {
	st := map[string]any{"sync_state": "Error", "error_message": msg}
	switch k.resource {
	case kube.Instances:
		st["state"] = "unknown"
	case kube.LoadBalancers:
		st["state"] = ""
	}
	return st
}

func s4WantStatus(t *testing.T, obj map[string]any, want map[string]any) {
	t.Helper()
	if obj == nil {
		t.Fatalf("object gone, want status %v", want)
	}
	if got := s4Status(obj); !reflect.DeepEqual(got, want) {
		t.Fatalf("status = %#v, want %#v", got, want)
	}
}

func TestDeletionNoCauseRemovesFinalizer(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.putNamespace(t, "") // neither NamespaceRole nor annotation
			e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}})
			e.srv.Delete(k.resource, s4Namespace, s4ObjName(k))

			e.pass(t)

			if obj := e.get(t, k); obj != nil {
				t.Fatalf("object still present with finalizers %v", s4Finalizers(obj))
			}
			if p := e.patches(k, true); len(p) != 0 {
				t.Fatalf("status patches = %d, want 0", len(p))
			}
		})
	}
}

func TestDeletionNoCauseKeepsOtherFinalizers(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.putNamespace(t, "")
			e.putObj(t, k, s4Obj{finalizers: []string{s4OtherFinal, k.finalizer}})
			e.srv.Delete(k.resource, s4Namespace, s4ObjName(k))

			e.pass(t)

			obj := e.get(t, k)
			if obj == nil {
				t.Fatal("object gone, want it kept by the other finalizer")
			}
			if got := s4Finalizers(obj); !reflect.DeepEqual(got, []string{s4OtherFinal}) {
				t.Fatalf("finalizers = %v, want [%s]", got, s4OtherFinal)
			}
			ps := e.patches(k, false)
			if len(ps) != 1 {
				t.Fatalf("object patches = %d, want 1", len(ps))
			}
			var body map[string]any
			if err := json.Unmarshal(ps[0].Body, &body); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			want := map[string]any{"metadata": map[string]any{"finalizers": []any{s4OtherFinal}}}
			if !reflect.DeepEqual(body, want) {
				t.Fatalf("finalizer patch = %s, want whole list %v", ps[0].Body, want)
			}
			if p := e.patches(k, true); len(p) != 0 {
				t.Fatalf("status patches = %d, want 0", len(p))
			}
		})
	}
}

func TestDeletionErrorKeepsFinalizer(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			e.putObj(t, k, s4Obj{
				finalizers:  []string{k.finalizer},
				annotations: map[string]string{s4OutcomeKey: "Error"},
			})
			e.srv.Delete(k.resource, s4Namespace, s4ObjName(k))

			e.pass(t)

			obj := e.get(t, k)
			if obj == nil || !s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer removed under outcome Error, want kept")
			}
			if p := e.patches(k, true); len(p) != 0 {
				t.Fatalf("status patches = %d, want 0", len(p))
			}
			if p := e.patches(k, false); len(p) != 0 {
				t.Fatalf("object patches = %d, want 0", len(p))
			}
		})
	}
}

func TestDeletionLockedLoadBalancer(t *testing.T) {
	synced := func() map[string]any {
		return map[string]any{
			"scaleway_id": s4ExistingID,
			"vip_address": "198.51.100.7",
			"state":       "ready",
			"project_id":  s4ProjectID,
			"sync_state":  "Synced",
		}
	}
	cases := []struct {
		name        string
		annotations map[string]string
		wantMsg     string
	}{
		{"default message", map[string]string{s4OutcomeKey: "Locked"}, s4LockedMessage},
		{"message annotation", map[string]string{s4OutcomeKey: "Locked", s4MessageKey: "custom lock"}, "custom lock"},
		{"empty message annotation", map[string]string{s4OutcomeKey: "Locked", s4MessageKey: ""}, s4LockedMessage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			e.putObj(t, loadBalancerKind, s4Obj{
				finalizers:  []string{loadBalancerKind.finalizer},
				annotations: c.annotations,
				status:      synced(),
			})
			e.srv.Delete(kube.LoadBalancers, s4Namespace, s4ObjName(loadBalancerKind))

			e.pass(t)

			obj := e.get(t, loadBalancerKind)
			if obj == nil || !s4HasFinalizer(obj, loadBalancerKind.finalizer) {
				t.Fatal("finalizer removed under Locked, want kept")
			}
			want := synced()
			want["sync_state"] = "TerminationBlocked"
			want["error_message"] = c.wantMsg
			s4WantStatus(t, obj, want)
		})
	}
}

func TestDeletionRefusedCauseRemovesFinalizer(t *testing.T) {
	cases := []struct {
		k     kind
		cause string
	}{
		{instanceKind, "Locked"},
		{scalewaySecretKind, "NotFound"},
		{loadBalancerKind, "Revoked"},
	}
	for _, c := range cases {
		t.Run(c.k.name+"/"+c.cause, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			e.putObj(t, c.k, s4Obj{
				finalizers:  []string{c.k.finalizer},
				annotations: map[string]string{s4OutcomeKey: c.cause},
			})
			e.srv.Delete(c.k.resource, s4Namespace, s4ObjName(c.k))

			e.pass(t)

			if obj := e.get(t, c.k); obj != nil {
				t.Fatalf("object still present with finalizers %v, want finalizer removed", s4Finalizers(obj))
			}
			if p := e.patches(c.k, true); len(p) != 0 {
				t.Fatalf("status patches = %d, want 0", len(p))
			}
		})
	}
}

func TestDeletionBadRegionKeepsFinalizer(t *testing.T) {
	e := s4Setup(t)
	e.valid(t)
	spec := s4Spec(scalewaySecretKind)
	spec["region"] = "xx-yyy"
	e.putObj(t, scalewaySecretKind, s4Obj{finalizers: []string{scalewaySecretKind.finalizer}, spec: spec})
	e.srv.Delete(kube.ScalewaySecrets, s4Namespace, s4ObjName(scalewaySecretKind))

	e.pass(t)

	obj := e.get(t, scalewaySecretKind)
	if obj == nil || !s4HasFinalizer(obj, scalewaySecretKind.finalizer) {
		t.Fatal("finalizer removed with an invalid region, want kept")
	}
	s4WantStatus(t, obj, s4ErrorStatus(scalewaySecretKind, s4RegionMessage))
}

func TestDeletionRegionCheckedBeforeNamespaceRole(t *testing.T) {
	// The region check runs first for every ScalewaySecret, deleted or not:
	// a new secret with an invalid region gets the region Error, not the
	// NamespaceRole one, and no finalizer.
	e := s4Setup(t)
	e.putNamespace(t, s4ProjectID) // no NamespaceRole
	spec := s4Spec(scalewaySecretKind)
	spec["region"] = "xx-yyy"
	e.putObj(t, scalewaySecretKind, s4Obj{spec: spec})

	e.pass(t)

	obj := e.get(t, scalewaySecretKind)
	if s4HasFinalizer(obj, scalewaySecretKind.finalizer) {
		t.Fatal("finalizer added with an invalid region")
	}
	s4WantStatus(t, obj, s4ErrorStatus(scalewaySecretKind, s4RegionMessage))
}

func TestDeletionValidRegions(t *testing.T) {
	for _, region := range []string{"fr-par", "nl-ams", "pl-waw"} {
		t.Run(region, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			spec := s4Spec(scalewaySecretKind)
			spec["region"] = region
			e.putObj(t, scalewaySecretKind, s4Obj{spec: spec})

			e.pass(t)

			obj := e.get(t, scalewaySecretKind)
			if !s4HasFinalizer(obj, scalewaySecretKind.finalizer) {
				t.Fatalf("region %s: finalizer not added", region)
			}
			if st := s4Status(obj); st != nil {
				t.Fatalf("region %s: status = %v, want none", region, st)
			}
		})
	}
}
