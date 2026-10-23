package kube_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const scwGroup = "/apis/scaleway.mathieubodin.io/v1"

// The closed list: eight constants, in order, with their plural, scope and kind.
func TestResourcesClosedList(t *testing.T) {
	want := []kube.Resource{
		kube.Instances, kube.LoadBalancers, kube.ScalewaySecrets, kube.NamespaceRoles,
		kube.Namespaces, kube.Secrets, kube.CRDs, kube.Deployments,
	}
	if got := kube.Resources(); !slices.Equal(got, want) {
		t.Fatalf("Resources() = %v, want %v", got, want)
	}
	if kube.Instances != 1 {
		t.Errorf("Instances = %d, want 1 (iota + 1)", int(kube.Instances))
	}

	cases := []struct {
		r          kube.Resource
		plural     string
		namespaced bool
		kind       string
	}{
		{kube.Instances, "instances", true, "Instance"},
		{kube.LoadBalancers, "loadbalancers", true, "LoadBalancer"},
		{kube.ScalewaySecrets, "scalewaysecrets", true, "ScalewaySecret"},
		{kube.NamespaceRoles, "namespaceroles", false, "NamespaceRole"},
		{kube.Namespaces, "namespaces", false, "Namespace"},
		{kube.Secrets, "secrets", true, "Secret"},
		{kube.CRDs, "customresourcedefinitions", false, "CustomResourceDefinition"},
		{kube.Deployments, "deployments", true, "Deployment"},
	}
	for _, tc := range cases {
		if got := tc.r.String(); got != tc.plural {
			t.Errorf("%d.String() = %q, want %q", int(tc.r), got, tc.plural)
		}
		if got := tc.r.Namespaced(); got != tc.namespaced {
			t.Errorf("%s.Namespaced() = %v, want %v", tc.plural, got, tc.namespaced)
		}
		if got := tc.r.Kind(); got != tc.kind {
			t.Errorf("%s.Kind() = %q, want %q", tc.plural, got, tc.kind)
		}
	}
}

// Matrix row Paths: exact strings for each constant.
func TestPathExactStrings(t *testing.T) {
	cases := []struct {
		r      kube.Resource
		ns     string
		name   string
		status bool
		want   string
	}{
		{kube.Instances, "ns", "n", false, scwGroup + "/namespaces/ns/instances/n"},
		{kube.Instances, "ns", "n", true, scwGroup + "/namespaces/ns/instances/n/status"},
		{kube.Instances, "ns", "", false, scwGroup + "/namespaces/ns/instances"},
		{kube.Instances, "", "", false, scwGroup + "/instances"},
		{kube.LoadBalancers, "ns", "n", false, scwGroup + "/namespaces/ns/loadbalancers/n"},
		{kube.LoadBalancers, "ns", "n", true, scwGroup + "/namespaces/ns/loadbalancers/n/status"},
		{kube.LoadBalancers, "ns", "", false, scwGroup + "/namespaces/ns/loadbalancers"},
		{kube.LoadBalancers, "", "", false, scwGroup + "/loadbalancers"},
		{kube.ScalewaySecrets, "ns", "n", false, scwGroup + "/namespaces/ns/scalewaysecrets/n"},
		{kube.ScalewaySecrets, "ns", "n", true, scwGroup + "/namespaces/ns/scalewaysecrets/n/status"},
		{kube.ScalewaySecrets, "ns", "", false, scwGroup + "/namespaces/ns/scalewaysecrets"},
		{kube.ScalewaySecrets, "", "", false, scwGroup + "/scalewaysecrets"},
		{kube.NamespaceRoles, "", "n", false, scwGroup + "/namespaceroles/n"},
		{kube.NamespaceRoles, "", "", false, scwGroup + "/namespaceroles"},
		{kube.Namespaces, "", "n", false, "/api/v1/namespaces/n"},
		{kube.Namespaces, "", "", false, "/api/v1/namespaces"},
		{kube.Secrets, "ns", "n", false, "/api/v1/namespaces/ns/secrets/n"},
		{kube.Secrets, "ns", "", false, "/api/v1/namespaces/ns/secrets"},
		{kube.Secrets, "", "", false, "/api/v1/secrets"},
		{kube.CRDs, "", "instances.scaleway.mathieubodin.io", false,
			"/apis/apiextensions.k8s.io/v1/customresourcedefinitions/instances.scaleway.mathieubodin.io"},
		{kube.CRDs, "", "", false, "/apis/apiextensions.k8s.io/v1/customresourcedefinitions"},
		{kube.Deployments, "ns", "", false, "/apis/apps/v1/namespaces/ns/deployments"},
		{kube.Deployments, "ns", "n", false, "/apis/apps/v1/namespaces/ns/deployments/n"},
	}
	for _, tc := range cases {
		got, err := kube.Path(tc.r, tc.ns, tc.name, tc.status)
		if err != nil {
			t.Errorf("Path(%s, %q, %q, %v): error %v, want %q", tc.r, tc.ns, tc.name, tc.status, err, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("Path(%s, %q, %q, %v) = %q, want %q", tc.r, tc.ns, tc.name, tc.status, got, tc.want)
		}
	}
}

// Scope and status rules of Path.
func TestPathScopeAndStatusErrors(t *testing.T) {
	cases := []struct {
		label  string
		r      kube.Resource
		ns     string
		name   string
		status bool
		want   error
	}{
		{"namespace on cluster kind (Namespaces)", kube.Namespaces, "ns", "n", false, kube.ErrInvalidName},
		{"namespace on cluster kind (NamespaceRoles)", kube.NamespaceRoles, "ns", "", false, kube.ErrInvalidName},
		{"namespace on cluster kind (CRDs)", kube.CRDs, "ns", "n", false, kube.ErrInvalidName},
		{"namespaced kind, name without namespace", kube.Instances, "", "n", false, kube.ErrInvalidName},
		{"namespaced kind, name without namespace (Secrets)", kube.Secrets, "", "n", false, kube.ErrInvalidName},
		{"status without name", kube.Instances, "ns", "", true, kube.ErrInvalidName},
		{"status on Namespaces", kube.Namespaces, "", "n", true, kube.ErrOperationNotAllowed},
		{"status on NamespaceRoles", kube.NamespaceRoles, "", "n", true, kube.ErrOperationNotAllowed},
		{"status on Secrets", kube.Secrets, "ns", "n", true, kube.ErrOperationNotAllowed},
		{"status on CRDs", kube.CRDs, "", "n", true, kube.ErrOperationNotAllowed},
		{"status on Deployments", kube.Deployments, "ns", "n", true, kube.ErrOperationNotAllowed},
		{"unknown resource 0", kube.Resource(0), "ns", "n", false, kube.ErrUnknownResource},
		{"unknown resource 9", kube.Resource(9), "ns", "n", false, kube.ErrUnknownResource},
		{"unknown resource -1", kube.Resource(-1), "", "", false, kube.ErrUnknownResource},
		{"unknown before invalid name", kube.Resource(42), "Bad", "a/b", true, kube.ErrUnknownResource},
	}
	for _, tc := range cases {
		got, err := kube.Path(tc.r, tc.ns, tc.name, tc.status)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Path = %q, %v; want error %v", tc.label, got, err, tc.want)
		}
	}
}

// Matrix row Bad names: namespace and name rules, before any request.
func TestPathNameRules(t *testing.T) {
	badNamespaces := []string{
		"Ns", "a.b", strings.Repeat("a", 64), "-a", "a-", "a_b", "a/b", "a%2Fb", ".", "..", "a b", "é",
	}
	for _, ns := range badNamespaces {
		if _, err := kube.Path(kube.Instances, ns, "n", false); !errors.Is(err, kube.ErrInvalidName) {
			t.Errorf("namespace %q: error %v, want ErrInvalidName", ns, err)
		}
		if _, err := kube.Path(kube.Instances, ns, "", false); !errors.Is(err, kube.ErrInvalidName) {
			t.Errorf("namespace %q (collection): error %v, want ErrInvalidName", ns, err)
		}
	}

	badNames := []string{
		"a/b", "a%2Fb", "..", ".", strings.Repeat("a", 254), "A", "-a", "a-", ".a", "a.", "a b",
		"a?b", "a#b", "a_b", "a\x00b",
	}
	for _, name := range badNames {
		if _, err := kube.Path(kube.Instances, "ns", name, false); !errors.Is(err, kube.ErrInvalidName) {
			t.Errorf("name %q: error %v, want ErrInvalidName", name, err)
		}
	}

	// Names of Namespaces objects follow the label rule.
	for _, name := range []string{"a.b", strings.Repeat("a", 64), "Ns"} {
		if _, err := kube.Path(kube.Namespaces, "", name, false); !errors.Is(err, kube.ErrInvalidName) {
			t.Errorf("Namespaces name %q: error %v, want ErrInvalidName", name, err)
		}
	}

	// Limits that are valid.
	ns63 := strings.Repeat("a", 63)
	if p, err := kube.Path(kube.Instances, ns63, "n", false); err != nil || p != scwGroup+"/namespaces/"+ns63+"/instances/n" {
		t.Errorf("63-char namespace: %q, %v", p, err)
	}
	name253 := strings.Repeat("a", 253)
	if p, err := kube.Path(kube.Instances, "ns", name253, false); err != nil || p != scwGroup+"/namespaces/ns/instances/"+name253 {
		t.Errorf("253-char name: %q, %v", p, err)
	}
	for _, name := range []string{"a", "0", "a-b", "a.b", "a1.b-2.c3"} {
		if _, err := kube.Path(kube.Instances, "ns", name, false); err != nil {
			t.Errorf("valid name %q: %v", name, err)
		}
	}
	if p, err := kube.Path(kube.Namespaces, "", ns63, false); err != nil || p != "/api/v1/namespaces/"+ns63 {
		t.Errorf("63-char Namespaces name: %q, %v", p, err)
	}
}

// ParsePath is the inverse of Path and refuses any other path.
func TestParsePathRoundTrip(t *testing.T) {
	type args struct {
		r      kube.Resource
		ns     string
		name   string
		status bool
	}
	var inputs []args
	for _, r := range kube.Resources() {
		if r.Namespaced() {
			inputs = append(inputs, args{r, "", "", false}, args{r, "ns", "", false}, args{r, "ns", "a.b", false})
		} else {
			inputs = append(inputs, args{r, "", "", false}, args{r, "", "n", false})
		}
	}
	for _, r := range []kube.Resource{kube.Instances, kube.LoadBalancers, kube.ScalewaySecrets} {
		inputs = append(inputs, args{r, "ns", "n", true})
	}
	for _, in := range inputs {
		p, err := kube.Path(in.r, in.ns, in.name, in.status)
		if err != nil {
			t.Fatalf("Path(%s, %q, %q, %v): %v", in.r, in.ns, in.name, in.status, err)
		}
		r, ns, name, status, err := kube.ParsePath(p)
		if err != nil {
			t.Errorf("ParsePath(%q): %v", p, err)
			continue
		}
		if r != in.r || ns != in.ns || name != in.name || status != in.status {
			t.Errorf("ParsePath(%q) = %s, %q, %q, %v; want %s, %q, %q, %v",
				p, r, ns, name, status, in.r, in.ns, in.name, in.status)
		}
	}

	bad := []string{
		"",
		"/",
		"/api/v1",
		"/api/v1/pods",
		"/api/v1/namespaces/ns/pods/p",
		"/api/v1/namespaces/n/status",
		"/api/v1/namespaces/ns/secrets/n/status",
		"/apis/apps/v1/namespaces/ns/deployments/n/status",
		scwGroup + "/namespaces/ns/instances/n/extra",
		scwGroup + "/namespaces/ns/instances/n/status/x",
		scwGroup + "/namespaces/ns/instances/",
		scwGroup + "/namespaces/Ns/instances/n",
		scwGroup + "/namespaces/ns/instances/a%2Fb",
		scwGroup + "/namespaces/ns/instances/..",
		scwGroup + "/namespaces/ns/instances?x=1",
		scwGroup + "/namespaces/ns/namespaceroles/n",
		scwGroup + "/namespaces/ns/namespaceroles",
		scwGroup + "/namespaces/ns/instances/n/status/",
		"/apis/scaleway.mathieubodin.io/v2/instances",
		"/api/v1/deployments",
		"instances",
	}
	for _, p := range bad {
		if r, ns, name, status, err := kube.ParsePath(p); !errors.Is(err, kube.ErrUnknownResource) {
			t.Errorf("ParsePath(%q) = %s, %q, %q, %v, %v; want ErrUnknownResource", p, r, ns, name, status, err)
		}
	}
}
