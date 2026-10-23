package kubetest_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
	"github.com/mathieubodin/scaleway-manager/internal/kube/kubetest"
)

// Behavior 1: list across namespaces and in one namespace.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestListAcrossNamespacesAndInOne(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Instances, instance("ns-b", "i2", ""))
	s.Put(kube.Instances, instance("ns-a", "i3", ""))
	s.Put(kube.Instances, instance("ns-a", "i1", ""))
	c := newClient(t, s)

	all, err := c.List(ctx, kube.Token(testToken), kube.Instances, "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	wantAll := [][2]string{{"ns-a", "i1"}, {"ns-a", "i3"}, {"ns-b", "i2"}}
	assertItems(t, all, wantAll)

	one, err := c.List(ctx, kube.Token(testToken), kube.Instances, "ns-a")
	if err != nil {
		t.Fatalf("List ns-a: %v", err)
	}
	assertItems(t, one, wantAll[:2])

	empty, err := c.List(ctx, kube.Token(testToken), kube.Instances, "ns-empty")
	if err != nil {
		t.Fatalf("List of a namespace with no objects: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty namespace: got %d items, want 0", len(empty))
	}

	// Body shape: {"apiVersion":"v1","kind":"InstanceList","metadata":{"resourceVersion":...},"items":[...]}.
	code, body := rawDo(t, s, http.MethodGet, mustPath(t, kube.Instances, "", "", false), testToken, "", nil)
	if code != http.StatusOK {
		t.Fatalf("raw list: code %d, body %s", code, body)
	}
	m := decode(t, body)
	if got := stringField(t, m, "kind"); got != "InstanceList" {
		t.Errorf("kind = %q, want InstanceList", got)
	}
	if got := stringField(t, m, "apiVersion"); got != "v1" {
		t.Errorf("apiVersion = %q, want v1", got)
	}
	stringField(t, m, "metadata", "resourceVersion")
	items, ok := m["items"].([]any)
	if !ok || len(items) != 3 {
		t.Errorf("items = %v, want an array of 3", m["items"])
	}
}

func assertItems(t *testing.T, items []json.RawMessage, want [][2]string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, raw := range items {
		m := decode(t, raw)
		ns := stringField(t, m, "metadata", "namespace")
		name := stringField(t, m, "metadata", "name")
		if ns != want[i][0] || name != want[i][1] {
			t.Errorf("item %d = %s/%s, want %s/%s", i, ns, name, want[i][0], want[i][1])
		}
	}
}

// Behavior 2: get of a namespace and of a cluster-scoped NamespaceRole; 404 when absent.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestGetNamespaceAndNamespaceRole(t *testing.T) {
	s := kubetest.New(t)
	s.Put(kube.Namespaces, json.RawMessage(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"team-a","annotations":{"scaleway.mathieubodin.io/project-id":"p"}}}`))
	s.Put(kube.NamespaceRoles, json.RawMessage(`{"apiVersion":"scaleway.mathieubodin.io/v1","kind":"NamespaceRole","metadata":{"name":"team-a"},"spec":{"role":"Editor"}}`))
	c := newClient(t, s)

	ns, err := c.Get(ctx, kube.Token(testToken), kube.Namespaces, "", "team-a")
	if err != nil {
		t.Fatalf("Get namespace: %v", err)
	}
	if got := stringField(t, decode(t, ns), "metadata", "annotations", "scaleway.mathieubodin.io/project-id"); got != "p" {
		t.Errorf("project-id = %q, want p", got)
	}

	role, err := c.Get(ctx, kube.Token(testToken), kube.NamespaceRoles, "", "team-a")
	if err != nil {
		t.Fatalf("Get NamespaceRole: %v", err)
	}
	if got := stringField(t, decode(t, role), "spec", "role"); got != "Editor" {
		t.Errorf("spec.role = %q, want Editor", got)
	}

	_, err = c.Get(ctx, kube.Token(testToken), kube.Namespaces, "", "absent")
	if code := apiCode(t, err); code != http.StatusNotFound {
		t.Errorf("absent namespace: code %d, want 404", code)
	}
	_, err = c.Get(ctx, kube.Token(testToken), kube.NamespaceRoles, "", "absent")
	if code := apiCode(t, err); code != http.StatusNotFound {
		t.Errorf("absent NamespaceRole: code %d, want 404", code)
	}
}

// Behavior 3: list of deployments in a namespace.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestListDeploymentsInNamespace(t *testing.T) {
	s := kubetest.New(t)
	dep := func(ns, name string) json.RawMessage {
		return json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"namespace":"` + ns + `","name":"` + name + `"},"spec":{"replicas":1}}`)
	}
	s.Put(kube.Deployments, dep("scaleway-system", "scaleway-operator"))
	s.Put(kube.Deployments, dep("other", "web"))
	c := newClient(t, s)

	items, err := c.List(ctx, kube.Token(testToken), kube.Deployments, "scaleway-system")
	if err != nil {
		t.Fatalf("List deployments: %v", err)
	}
	assertItems(t, items, [][2]string{{"scaleway-system", "scaleway-operator"}})

	code, body := rawDo(t, s, http.MethodGet, mustPath(t, kube.Deployments, "scaleway-system", "", false), testToken, "", nil)
	if code != http.StatusOK {
		t.Fatalf("raw list: code %d, body %s", code, body)
	}
	if got := stringField(t, decode(t, body), "kind"); got != "DeploymentList" {
		t.Errorf("kind = %q, want DeploymentList", got)
	}
}

// Behavior 13: 404 on an unknown object, with a Kubernetes Status body.
// Reproduces: https://kubernetes.io/docs/reference/using-api/api-concepts/
func TestUnknownObjectIs404(t *testing.T) {
	s := kubetest.New(t)
	c := newClient(t, s)

	_, err := c.Get(ctx, kube.Token(testToken), kube.Instances, "ns", "missing")
	ae := apiErr(t, err)
	if ae.Code != http.StatusNotFound {
		t.Fatalf("Get: code %d, want 404", ae.Code)
	}
	assertStatusBody(t, ae.Body, http.StatusNotFound, "NotFound")

	_, err = c.MergePatch(ctx, kube.Token(testToken), kube.Instances, "ns", "missing", []byte(`{"spec":{"x":1}}`))
	if code := apiCode(t, err); code != http.StatusNotFound {
		t.Errorf("MergePatch: code %d, want 404", code)
	}
	_, err = c.PatchStatus(ctx, kube.Token(testToken), kube.Instances, "ns", "missing", []byte(`{"status":{"x":1}}`))
	if code := apiCode(t, err); code != http.StatusNotFound {
		t.Errorf("PatchStatus: code %d, want 404", code)
	}
}
