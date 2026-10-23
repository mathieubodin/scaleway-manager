package main

// Story 3 (#42), CAP-3: the engine moves each object one step per pass with the
// operator's status values. Every test runs pass with a fixed clock and a seeded
// rand against kubetest, with no sleep, and compares the whole status after each
// pass. The fixtures satisfy the checks of story 4 (NamespaceRole with an Editor
// role, project-id annotation, valid zone, type and region) and carry no cause
// annotation of story 5, so these tests stay valid once the leaves are merged.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
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
	engNamespace = "team-a"
	engProjectID = "5f0c6a2e-8b1d-4c3a-9e7f-1a2b3c4d5e6f"
	engToken     = "engine-test-token"

	engInstanceFinalizer = "scaleway.mathieubodin.io/instance-finalizer"
	engLBFinalizer       = "scaleway.mathieubodin.io/loadbalancer-finalizer"
	engSecretFinalizer   = "scaleway.mathieubodin.io/secret-finalizer"
)

// engSeed1 and engSeed2 seed both the operator's rand and the test's replica.
const (
	engSeed1 = 42
	engSeed2 = 7
)

// engNow is not UTC on purpose: created_at must be written in UTC.
var engNow = time.Date(2026, 10, 23, 8, 30, 15, 0, time.FixedZone("CEST", 2*60*60))

const engCreatedAt = "2026-10-23T06:30:15Z"

type engFixedClock struct{ t time.Time }

func (c engFixedClock) Now() time.Time { return c.t }

type engEnv struct {
	t    *testing.T
	srv  *kubetest.Server
	op   *operator
	logs *bytes.Buffer
}

// engSetup starts kubetest with a namespace carrying a UUID project-id
// annotation and its Editor NamespaceRole, and builds the operator.
func engSetup(t *testing.T) *engEnv {
	t.Helper()
	srv := kubetest.New(t)
	srv.ExpectToken(engToken)
	srv.Put(kube.Namespaces, json.RawMessage(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"`+engNamespace+`","annotations":{"scaleway.mathieubodin.io/project-id":"`+engProjectID+`"}}}`))
	srv.Put(kube.NamespaceRoles, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"NamespaceRole","metadata":{"name":"`+engNamespace+`"},"spec":{"namespace":"`+engNamespace+`","scaleway_role":"Editor"}}`))

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte(engToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		switch k {
		case "KUBERNETES_SERVICE_HOST":
			return srv.Host
		case "KUBERNETES_SERVICE_PORT":
			return srv.Port
		}
		return ""
	}
	c, err := kube.NewInCluster(getenv, srv.CAPEM)
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	logs := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	op := newOperator(c, engFixedClock{engNow}, rand.New(rand.NewPCG(engSeed1, engSeed2)), tokenPath, log)
	return &engEnv{t: t, srv: srv, op: op, logs: logs}
}

// engReplica returns a rand with the operator's seed, to compute the expected
// generated values in the order the contract draws them.
func engReplica() *rand.Rand { return rand.New(rand.NewPCG(engSeed1, engSeed2)) }

// engUUID computes a scaleway_id the way go-interfaces.md "Generated values" says.
func engUUID(r *rand.Rand) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[0:8], r.Uint64())
	binary.BigEndian.PutUint64(b[8:16], r.Uint64())
	b[6] = 0x40 | b[6]&0x0f
	b[8] = 0x80 | b[8]&0x3f
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func engOctet(r *rand.Rand) int { return r.IntN(254) + 1 }

func (e *engEnv) pass() {
	e.t.Helper()
	if err := e.op.pass(context.Background()); err != nil {
		e.t.Fatalf("pass: %v", err)
	}
}

func engDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return m
}

func (e *engEnv) object(r kube.Resource, name string) map[string]any {
	e.t.Helper()
	raw, ok := e.srv.Object(r, engNamespace, name)
	if !ok {
		e.t.Fatalf("%s %s/%s not found", r, engNamespace, name)
	}
	return engDecode(e.t, raw)
}

func engMeta(obj map[string]any) map[string]any {
	m, _ := obj["metadata"].(map[string]any)
	return m
}

func engFinalizers(obj map[string]any) []any {
	l, _ := engMeta(obj)["finalizers"].([]any)
	return l
}

func (e *engEnv) status(r kube.Resource, name string) map[string]any {
	e.t.Helper()
	s, _ := e.object(r, name)["status"].(map[string]any)
	return s
}

func (e *engEnv) resourceVersion(r kube.Resource, name string) string {
	e.t.Helper()
	rv, _ := engMeta(e.object(r, name))["resourceVersion"].(string)
	return rv
}

// patchesSince returns the PATCH requests recorded from index from on.
func (e *engEnv) patchesSince(from int) []kubetest.Request {
	var out []kubetest.Request
	for _, r := range e.srv.Requests()[from:] {
		if r.Method == "PATCH" {
			out = append(out, r)
		}
	}
	return out
}

func engIsStatus(r kubetest.Request) bool {
	return strings.HasSuffix(strings.SplitN(r.RequestURI, "?", 2)[0], "/status")
}

func engAssertStatus(t *testing.T, step string, got, want map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(want)
		t.Fatalf("%s: status\n got  %s\n want %s", step, gj, wj)
	}
}

func engAssertBody(t *testing.T, step string, r kubetest.Request, want map[string]any) {
	t.Helper()
	got := engDecode(t, r.Body)
	if !reflect.DeepEqual(got, want) {
		wj, _ := json.Marshal(want)
		t.Fatalf("%s: patch body\n got  %s\n want %s", step, r.Body, wj)
	}
}

func engPutInstance(e *engEnv, name string) {
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"`+name+`","namespace":"`+engNamespace+`"},"spec":{"name":"`+name+`","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))
}

func engPutLB(e *engEnv, name string) {
	e.srv.Put(kube.LoadBalancers, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"LoadBalancer","metadata":{"name":"`+name+`","namespace":"`+engNamespace+`"},"spec":{"name":"`+name+`","zone":"fr-par-1","lb_type":"LB-S"}}`))
}

func engPutSecret(e *engEnv, name string) {
	e.srv.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"`+name+`","namespace":"`+engNamespace+`"},"spec":{"name":"`+name+`","region":"fr-par","source":{"kubernetes_secret":{"name":"src","key":"value"}}}}`))
}

// engFinalizerPass checks pass 1: one finalizer patch, no status.
func engFinalizerPass(t *testing.T, e *engEnv, r kube.Resource, name, finalizer string) {
	t.Helper()
	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 1 || engIsStatus(patches[0]) {
		t.Fatalf("pass 1: want one finalizer PATCH, got %d PATCH requests: %+v", len(patches), patches)
	}
	engAssertBody(t, "pass 1", patches[0], map[string]any{"metadata": map[string]any{"finalizers": []any{finalizer}}})
	obj := e.object(r, name)
	if got := engFinalizers(obj); !reflect.DeepEqual(got, []any{finalizer}) {
		t.Fatalf("pass 1: finalizers = %v, want [%s]", got, finalizer)
	}
	if s, ok := obj["status"]; ok {
		t.Fatalf("pass 1: status written: %v", s)
	}
}

// TestLifecycleInstance covers matrix rows "Pass 1", "Pass 2" and "Pass 3".
func TestLifecycleInstance(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	r := engReplica()
	id := engUUID(r)
	ip := fmt.Sprintf("192.0.2.%d", engOctet(r))

	engFinalizerPass(t, e, kube.Instances, "web", engInstanceFinalizer)

	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 1 || !engIsStatus(patches[0]) {
		t.Fatalf("pass 2: want one /status PATCH, got %+v", patches)
	}
	engAssertBody(t, "pass 2", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":   id,
		"public_ip":     nil,
		"state":         "creating",
		"project_id":    engProjectID,
		"created_at":    engCreatedAt,
		"error_message": nil,
		"sync_state":    "Syncing",
	}})
	engAssertStatus(t, "pass 2", e.status(kube.Instances, "web"), map[string]any{
		"scaleway_id": id,
		"state":       "creating",
		"project_id":  engProjectID,
		"created_at":  engCreatedAt,
		"sync_state":  "Syncing",
	})

	from = len(e.srv.Requests())
	e.pass()
	patches = e.patchesSince(from)
	if len(patches) != 1 || !engIsStatus(patches[0]) {
		t.Fatalf("pass 3: want one /status PATCH, got %+v", patches)
	}
	engAssertBody(t, "pass 3", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":   id,
		"public_ip":     ip,
		"state":         "running",
		"project_id":    engProjectID,
		"created_at":    engCreatedAt,
		"error_message": nil,
		"sync_state":    "Synced",
	}})
	engAssertStatus(t, "pass 3", e.status(kube.Instances, "web"), map[string]any{
		"scaleway_id": id,
		"public_ip":   ip,
		"state":       "running",
		"project_id":  engProjectID,
		"created_at":  engCreatedAt,
		"sync_state":  "Synced",
	})
	if got := engFinalizers(e.object(kube.Instances, "web")); !reflect.DeepEqual(got, []any{engInstanceFinalizer}) {
		t.Fatalf("pass 3: finalizers = %v", got)
	}
}

// TestLifecycleLoadBalancer covers matrix row "LoadBalancer".
func TestLifecycleLoadBalancer(t *testing.T) {
	e := engSetup(t)
	engPutLB(e, "front")
	r := engReplica()
	id := engUUID(r)
	vip := fmt.Sprintf("198.51.100.%d", engOctet(r))

	engFinalizerPass(t, e, kube.LoadBalancers, "front", engLBFinalizer)

	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 1 || !engIsStatus(patches[0]) {
		t.Fatalf("pass 2: want one /status PATCH, got %+v", patches)
	}
	engAssertBody(t, "pass 2", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":   id,
		"vip_address":   nil,
		"state":         "pending",
		"project_id":    engProjectID,
		"created_at":    nil,
		"error_message": nil,
		"sync_state":    "Syncing",
	}})
	engAssertStatus(t, "pass 2", e.status(kube.LoadBalancers, "front"), map[string]any{
		"scaleway_id": id,
		"state":       "pending",
		"project_id":  engProjectID,
		"sync_state":  "Syncing",
	})

	from = len(e.srv.Requests())
	e.pass()
	patches = e.patchesSince(from)
	if len(patches) != 1 || !engIsStatus(patches[0]) {
		t.Fatalf("pass 3: want one /status PATCH, got %+v", patches)
	}
	engAssertBody(t, "pass 3", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":   id,
		"vip_address":   vip,
		"state":         "ready",
		"project_id":    engProjectID,
		"created_at":    nil,
		"error_message": nil,
		"sync_state":    "Synced",
	}})
	engAssertStatus(t, "pass 3", e.status(kube.LoadBalancers, "front"), map[string]any{
		"scaleway_id": id,
		"vip_address": vip,
		"state":       "ready",
		"project_id":  engProjectID,
		"sync_state":  "Synced",
	})
}

// TestLifecycleScalewaySecret covers matrix row "ScalewaySecret": two /status
// PATCHes in order in one pass, Syncing then Synced.
func TestLifecycleScalewaySecret(t *testing.T) {
	e := engSetup(t)
	engPutSecret(e, "db-password")
	id := engUUID(engReplica())

	engFinalizerPass(t, e, kube.ScalewaySecrets, "db-password", engSecretFinalizer)

	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 2 || !engIsStatus(patches[0]) || !engIsStatus(patches[1]) {
		t.Fatalf("pass 2: want two /status PATCH requests, got %+v", patches)
	}
	engAssertBody(t, "pass 2, first patch", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":                  id,
		"current_version":              nil,
		"last_synced_resource_version": nil,
		"observed_generation":          nil,
		"sync_state":                   "Syncing",
		"error_message":                nil,
	}})
	engAssertBody(t, "pass 2, second patch", patches[1], map[string]any{"status": map[string]any{
		"scaleway_id":                  id,
		"current_version":              json.Number("1"),
		"last_synced_resource_version": nil,
		"observed_generation":          json.Number("1"),
		"sync_state":                   "Synced",
		"error_message":                nil,
	}})
	engAssertStatus(t, "pass 2", e.status(kube.ScalewaySecrets, "db-password"), map[string]any{
		"scaleway_id":         id,
		"current_version":     json.Number("1"),
		"observed_generation": json.Number("1"),
		"sync_state":          "Synced",
	})
}

// TestLifecycleSteady covers matrix row "Steady": a Synced object of each kind,
// three more passes, no PATCH, resourceVersion and generated values unchanged.
func TestLifecycleSteady(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	engPutLB(e, "front")
	engPutSecret(e, "db-password")
	for i := 0; i < 3; i++ {
		e.pass()
	}
	type ref struct {
		r    kube.Resource
		name string
	}
	refs := []ref{{kube.Instances, "web"}, {kube.LoadBalancers, "front"}, {kube.ScalewaySecrets, "db-password"}}
	statuses := map[string]map[string]any{}
	versions := map[string]string{}
	for _, x := range refs {
		s := e.status(x.r, x.name)
		if s["sync_state"] != "Synced" {
			t.Fatalf("%s %s not Synced after three passes: %v", x.r, x.name, s)
		}
		statuses[x.name] = s
		versions[x.name] = e.resourceVersion(x.r, x.name)
	}
	for _, k := range []string{"scaleway_id", "public_ip", "created_at"} {
		if _, ok := statuses["web"][k]; !ok {
			t.Fatalf("Instance has no %s: %v", k, statuses["web"])
		}
	}
	if _, ok := statuses["front"]["vip_address"]; !ok {
		t.Fatalf("LoadBalancer has no vip_address: %v", statuses["front"])
	}

	from := len(e.srv.Requests())
	for i := 0; i < 3; i++ {
		e.pass()
	}
	if p := e.patchesSince(from); len(p) != 0 {
		t.Fatalf("steady passes sent %d PATCH requests: %+v", len(p), p)
	}
	for _, x := range refs {
		engAssertStatus(t, "steady "+x.name, e.status(x.r, x.name), statuses[x.name])
		if got := e.resourceVersion(x.r, x.name); got != versions[x.name] {
			t.Fatalf("%s resourceVersion %s → %s", x.name, versions[x.name], got)
		}
	}
}

// TestLifecycleSecretVersionPush covers matrix row "Spec change".
func TestLifecycleSecretVersionPush(t *testing.T) {
	e := engSetup(t)
	const id = "0b7d2c1e-3f4a-4b5c-8d6e-7f8091a2b3c4"
	e.srv.Put(kube.ScalewaySecrets, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"ScalewaySecret","metadata":{"name":"db-password","namespace":"`+engNamespace+`","generation":2,"finalizers":["`+engSecretFinalizer+`"]},"spec":{"name":"db-password","region":"fr-par","source":{"kubernetes_secret":{"name":"src","key":"value"}}},"status":{"scaleway_id":"`+id+`","current_version":1,"observed_generation":1,"sync_state":"Synced"}}`))

	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 1 || !engIsStatus(patches[0]) {
		t.Fatalf("want one /status PATCH, got %+v", patches)
	}
	engAssertBody(t, "version push", patches[0], map[string]any{"status": map[string]any{
		"scaleway_id":                  id,
		"current_version":              json.Number("2"),
		"last_synced_resource_version": nil,
		"observed_generation":          json.Number("2"),
		"sync_state":                   "Synced",
		"error_message":                nil,
	}})
	engAssertStatus(t, "version push", e.status(kube.ScalewaySecrets, "db-password"), map[string]any{
		"scaleway_id":         id,
		"current_version":     json.Number("2"),
		"observed_generation": json.Number("2"),
		"sync_state":          "Synced",
	})

	from = len(e.srv.Requests())
	e.pass()
	if p := e.patchesSince(from); len(p) != 0 {
		t.Fatalf("pass after the push sent PATCH requests: %+v", p)
	}
}

// TestLifecycleFailedList covers matrix row "Failed list": Instances skipped
// and logged, the two other kinds processed in the same pass.
func TestLifecycleFailedList(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	engPutLB(e, "front")
	engPutSecret(e, "db-password")
	e.srv.Fail("GET", "/apis/scaleway.mathieubodin.io/v1/instances", 500)

	_ = e.op.pass(context.Background())

	if f := engFinalizers(e.object(kube.Instances, "web")); len(f) != 0 {
		t.Fatalf("Instance processed despite the failed list: finalizers %v", f)
	}
	if f := engFinalizers(e.object(kube.LoadBalancers, "front")); !reflect.DeepEqual(f, []any{engLBFinalizer}) {
		t.Fatalf("LoadBalancer finalizers = %v", f)
	}
	if f := engFinalizers(e.object(kube.ScalewaySecrets, "db-password")); !reflect.DeepEqual(f, []any{engSecretFinalizer}) {
		t.Fatalf("ScalewaySecret finalizers = %v", f)
	}
	for _, r := range e.srv.Requests() {
		if r.Method == "PATCH" && strings.Contains(r.RequestURI, "/instances/") {
			t.Fatalf("PATCH sent to an Instance: %s", r.RequestURI)
		}
	}

	found := false
	for _, line := range engLogLines(t, e.logs) {
		if line["msg"] == "list failed" {
			if _, ok := line["kind"]; !ok {
				t.Fatalf("list failed log has no kind: %v", line)
			}
			if _, ok := line["error"]; !ok {
				t.Fatalf("list failed log has no error: %v", line)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("no \"list failed\" log line:\n%s", e.logs)
	}
	if strings.Contains(e.logs.String(), engToken) {
		t.Fatalf("token in logs:\n%s", e.logs)
	}
}

// TestLifecycleObjectErrorDoesNotStopPass: an error on one object is logged
// and the other objects of the pass are processed.
func TestLifecycleObjectErrorDoesNotStopPass(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "broken")
	engPutInstance(e, "web")
	p, err := kube.Path(kube.Instances, engNamespace, "broken", false)
	if err != nil {
		t.Fatal(err)
	}
	e.srv.Fail("PATCH", p, 500)

	if err := e.op.pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if f := engFinalizers(e.object(kube.Instances, "web")); !reflect.DeepEqual(f, []any{engInstanceFinalizer}) {
		t.Fatalf("web finalizers = %v", f)
	}
	found := false
	for _, line := range engLogLines(t, e.logs) {
		if line["msg"] == "object failed" && line["name"] == "broken" && line["namespace"] == engNamespace {
			if _, ok := line["kind"]; !ok {
				t.Fatalf("object failed log has no kind: %v", line)
			}
			if _, ok := line["error"]; !ok {
				t.Fatalf("object failed log has no error: %v", line)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("no \"object failed\" log line for broken:\n%s", e.logs)
	}
	if strings.Contains(e.logs.String(), engToken) {
		t.Fatalf("token in logs:\n%s", e.logs)
	}
}

// TestLifecycleToken: pass reads the token file (trimmed) and sends it on
// every request; an unreadable or empty file fails the pass before any request.
func TestLifecycleToken(t *testing.T) {
	e := engSetup(t)
	engPutInstance(e, "web")
	e.pass()
	toks := e.srv.Tokens()
	if len(toks) == 0 {
		t.Fatal("no request sent")
	}
	for i, tok := range toks {
		if tok != engToken {
			t.Fatalf("request %d token %q, want %q", i, tok, engToken)
		}
	}

	for name, setup := range map[string]func(path string){
		"missing": func(path string) { _ = os.Remove(path) },
		"empty": func(path string) {
			if err := os.WriteFile(path, []byte(" \n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := engSetup(t)
			engPutInstance(e, "web")
			setup(e.op.tokenPath)
			if err := e.op.pass(context.Background()); err == nil {
				t.Fatal("pass returned nil with an unusable token file")
			}
			if n := len(e.srv.Requests()); n != 0 {
				t.Fatalf("%d requests sent with an unusable token file", n)
			}
		})
	}
}

func engLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", l)
		}
		out = append(out, m)
	}
	return out
}

// TestLifecycleDeletionStub: an object with deletionTimestamp has its finalizer
// removed (whole list, other finalizers kept) and gets no status write.
func TestLifecycleDeletionStub(t *testing.T) {
	e := engSetup(t)
	e.srv.Put(kube.Instances, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"Instance","metadata":{"name":"web","namespace":"`+engNamespace+`","finalizers":["example.com/keep","`+engInstanceFinalizer+`"]},"spec":{"name":"web","zone":"fr-par-1","image":"ubuntu_jammy","instance_type":"DEV1-S"}}`))
	e.srv.Delete(kube.Instances, engNamespace, "web")

	from := len(e.srv.Requests())
	e.pass()
	patches := e.patchesSince(from)
	if len(patches) != 1 || engIsStatus(patches[0]) {
		t.Fatalf("want one finalizer PATCH, got %+v", patches)
	}
	engAssertBody(t, "deletion", patches[0], map[string]any{"metadata": map[string]any{"finalizers": []any{"example.com/keep"}}})
	if f := engFinalizers(e.object(kube.Instances, "web")); !reflect.DeepEqual(f, []any{"example.com/keep"}) {
		t.Fatalf("finalizers = %v, want [example.com/keep]", f)
	}

	from = len(e.srv.Requests())
	e.pass()
	if p := e.patchesSince(from); len(p) != 0 {
		t.Fatalf("pass after finalizer removal sent PATCH requests: %+v", p)
	}
}

// The tests below reuse the helpers of causes_test.go.

// A spec change pushes a version whenever scaleway_id is set, whatever
// sync_state; leaving Revoked pushes one too.
func TestLifecycleSecretSpecChangePushesWhateverSyncState(t *testing.T) {
	cases := []struct {
		syncState string
		wantVer   int
	}{
		{"Error", 3},
		{"Syncing", 3},
		{"Revoked", 3},
	}
	for _, c := range cases {
		t.Run(c.syncState, func(t *testing.T) {
			e := causesSetup(t)
			st := causesWith(causesSyncedSecret(), "sync_state", c.syncState, "error_message", "previous")
			e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 2, nil, true, st))

			e.pass(t)

			got := e.status(t, scalewaySecretKind, "sec")
			if got["current_version"] != float64(c.wantVer) {
				t.Errorf("current_version = %v, want %d", got["current_version"], c.wantVer)
			}
			if got["observed_generation"] != float64(2) {
				t.Errorf("observed_generation = %v, want 2", got["observed_generation"])
			}
		})
	}
}

// observed_generation null with a scaleway_id: the generation is recorded
// without a version push (operator decide_next_action_secret step 7bis).
func TestLifecycleSecretNullObservedGenerationRecordedWithoutPush(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "observed_generation", nil)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 4, nil, true, st))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "observed_generation", 4))

	causesNoPatch(t, e.passPatches(t))
}

// A ScalewaySecret leaving Revoked pushes a version, as the operator does:
// its Revoked status clears last_synced_resource_version (decide_next_action_secret
// step 7).
func TestLifecycleSecretLeavingRevokedPushes(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "sync_state", "Revoked", "error_message", "revoked")
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, nil, true, st))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "current_version", 3))

	causesNoPatch(t, e.passPatches(t))
}

// created_at carries the fraction of a second as chrono's RFC 3339 with
// SecondsFormat::AutoSi: none, 3, 6 or 9 digits.
func TestLifecycleCreatedAtFraction(t *testing.T) {
	for _, tc := range []struct {
		nsec int
		want string
	}{
		{0, "2026-10-23T12:00:00Z"},
		{500_000_000, "2026-10-23T12:00:00.500Z"},
		{1_500_000, "2026-10-23T12:00:00.001500Z"},
		{1, "2026-10-23T12:00:00.000000001Z"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			e := causesSetup(t)
			e.o.clock = engFixedClock{time.Date(2026, 10, 23, 14, 0, 0, tc.nsec, time.FixedZone("CEST", 2*60*60))}
			e.put(t, instanceKind, causesObject(instanceKind, "vm", 1, nil, true, nil))

			e.pass(t)
			if got := e.status(t, instanceKind, "vm")["created_at"]; got != tc.want {
				t.Errorf("created_at = %v, want %s", got, tc.want)
			}
		})
	}
}
