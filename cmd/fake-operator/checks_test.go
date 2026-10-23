package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// Uses the s4 helpers of deletion_test.go.

const (
	s4NoRoleMessage = "Configuration error: No NamespaceRole found for namespace 'team-a'. " +
		"Create a NamespaceRole resource with name 'team-a' to assign a Scaleway role."
	s4NoAnnotationMessage = "Configuration error: Namespace 'team-a' must have annotation 'scaleway.mathieubodin.io/project-id'"
	s4NoSourceMessage     = "Secret source not configured: spec.source must have kubernetes_secret configured"
	s4NamespaceErrPrefix  = "Configuration error: Cannot access namespace team-a: "
)

func s4ReadOnlyMessage(k kind, role string) string {
	what := map[string]string{
		"Instance":       "create instances",
		"LoadBalancer":   "create load balancers",
		"ScalewaySecret": "sync secrets",
	}[k.name]
	return "Configuration error: Role '" + role + "' is read-only and cannot " + what + ". Use 'Editor' or 'Admin'."
}

func s4MustPath(t *testing.T, r kube.Resource, name string) string {
	t.Helper()
	p, err := kube.Path(r, "", name, false)
	if err != nil {
		t.Fatalf("kube.Path: %v", err)
	}
	return p
}

// s4WantNamespaceError asserts sync_state Error and an error_message that is
// the exact prefix plus non-empty text.
func s4WantNamespaceError(t *testing.T, obj map[string]any) {
	t.Helper()
	st := s4Status(obj)
	if st["sync_state"] != "Error" {
		t.Fatalf("sync_state = %v, want Error", st["sync_state"])
	}
	msg, _ := st["error_message"].(string)
	rest, ok := strings.CutPrefix(msg, s4NamespaceErrPrefix)
	if !ok || strings.TrimSpace(rest) == "" {
		t.Fatalf("error_message = %q, want prefix %q followed by non-empty text", msg, s4NamespaceErrPrefix)
	}
}

func TestChecksNoNamespaceRole(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.putNamespace(t, s4ProjectID)
			e.putObj(t, k, s4Obj{})

			e.pass(t)

			obj := e.get(t, k)
			if s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer added without a NamespaceRole")
			}
			s4WantStatus(t, obj, s4ErrorStatus(k, s4NoRoleMessage))
		})
	}
}

func TestChecksNamespaceRoleReadError(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			e.putObj(t, k, s4Obj{})
			e.srv.Fail("GET", s4MustPath(t, kube.NamespaceRoles, s4Namespace), 500)

			e.pass(t)

			obj := e.get(t, k)
			if s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer added on a NamespaceRole read error")
			}
			s4WantStatus(t, obj, s4ErrorStatus(k, "Kubernetes API error"))
		})
	}
}

func TestChecksReadOnlyRole(t *testing.T) {
	for _, role := range []string{"Viewer", "editor", "-"} {
		shown := role
		if role == "-" {
			shown = "" // no spec.scaleway_role counts as the role ''
		}
		for _, k := range []kind{instanceKind, loadBalancerKind} {
			t.Run(k.name+"/"+role, func(t *testing.T) {
				e := s4Setup(t)
				e.putNamespace(t, s4ProjectID)
				e.putRole(t, role)
				e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}})

				e.pass(t)

				obj := e.get(t, k)
				if !s4HasFinalizer(obj, k.finalizer) {
					t.Fatal("finalizer removed, want present")
				}
				s4WantStatus(t, obj, s4ErrorStatus(k, s4ReadOnlyMessage(k, shown)))
			})
		}
		t.Run("ScalewaySecret/"+role, func(t *testing.T) {
			k := scalewaySecretKind
			e := s4Setup(t)
			e.putNamespace(t, s4ProjectID)
			e.putRole(t, role)
			e.putObj(t, k, s4Obj{})

			e.pass(t)

			obj := e.get(t, k)
			if s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer added for a read-only role")
			}
			s4WantStatus(t, obj, s4ErrorStatus(k, s4ReadOnlyMessage(k, shown)))
		})
	}
}

func TestChecksReadOnlyRoleIgnoredOnceCreated(t *testing.T) {
	// For Instance and LoadBalancer the role is checked only while
	// status.scaleway_id is null.
	for _, k := range []kind{instanceKind, loadBalancerKind} {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.putNamespace(t, s4ProjectID)
			e.putRole(t, "Viewer")
			state := "creating"
			if k.resource == kube.LoadBalancers {
				state = "pending"
			}
			e.putObj(t, k, s4Obj{
				finalizers: []string{k.finalizer},
				status: map[string]any{
					"scaleway_id": s4ExistingID,
					"state":       state,
					"project_id":  s4ProjectID,
					"sync_state":  "Syncing",
				},
			})

			e.pass(t)

			if got := s4Status(e.get(t, k))["sync_state"]; got != "Synced" {
				t.Fatalf("sync_state = %v, want Synced", got)
			}
		})
	}
}

func TestChecksAllowedRoles(t *testing.T) {
	for _, role := range []string{"Editor", "Admin", "OrganizationOwner"} {
		for _, k := range kinds {
			t.Run(k.name+"/"+role, func(t *testing.T) {
				e := s4Setup(t)
				e.putNamespace(t, s4ProjectID)
				e.putRole(t, role)
				e.putObj(t, k, s4Obj{})

				e.pass(t)

				obj := e.get(t, k)
				if !s4HasFinalizer(obj, k.finalizer) {
					t.Fatal("finalizer not added for an allowed role")
				}
				if st := s4Status(obj); st != nil {
					t.Fatalf("status = %v, want none", st)
				}
			})
		}
	}
}

func TestChecksAnnotationMissing(t *testing.T) {
	for _, k := range []kind{instanceKind, loadBalancerKind} {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.putNamespace(t, "")
			e.putRole(t, "Editor")
			e.putObj(t, k, s4Obj{})

			e.pass(t)

			obj := e.get(t, k)
			if s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer added without the project-id annotation")
			}
			s4WantStatus(t, obj, s4ErrorStatus(k, s4NoAnnotationMessage))
		})
	}
}

func TestChecksAnnotationNotUUID(t *testing.T) {
	for _, pid := range []string{"not-a-uuid", "8c6c5a8e-3f0b-4a8e-9f3c-2b7d1e4a5c6", "8c6c5a8e3f0b4a8e9f3c2b7d1e4a5c6g"} {
		for _, k := range []kind{instanceKind, loadBalancerKind} {
			t.Run(k.name+"/"+pid, func(t *testing.T) {
				e := s4Setup(t)
				e.putNamespace(t, pid)
				e.putRole(t, "Editor")
				e.putObj(t, k, s4Obj{})

				e.pass(t)

				obj := e.get(t, k)
				if s4HasFinalizer(obj, k.finalizer) {
					t.Fatal("finalizer added with a non-UUID project-id")
				}
				want := "Configuration error: Annotation 'scaleway.mathieubodin.io/project-id' must be a valid UUID, got: '" + pid + "'"
				s4WantStatus(t, obj, s4ErrorStatus(k, want))
			})
		}
	}
}

func TestChecksAnnotationUUIDForms(t *testing.T) {
	// The forms Rust's Uuid::parse_str accepts, in any case.
	forms := []string{
		"8c6c5a8e-3f0b-4a8e-9f3c-2b7d1e4a5c6d",
		"8C6C5A8E-3F0B-4A8E-9F3C-2B7D1E4A5C6D",
		"8c6c5a8e3f0b4a8e9f3c2b7d1e4a5c6d",
		"{8c6c5a8e-3f0b-4a8e-9f3c-2b7d1e4a5c6d}",
		"urn:uuid:8c6c5a8e-3f0b-4a8e-9f3c-2b7d1e4a5c6d",
	}
	for _, pid := range forms {
		for _, k := range []kind{instanceKind, loadBalancerKind} {
			t.Run(k.name+"/"+pid, func(t *testing.T) {
				e := s4Setup(t)
				e.putNamespace(t, pid)
				e.putRole(t, "Editor")
				e.putObj(t, k, s4Obj{})

				e.pass(t)

				obj := e.get(t, k)
				if !s4HasFinalizer(obj, k.finalizer) {
					t.Fatalf("finalizer not added for UUID form %q (status %v)", pid, s4Status(obj))
				}
			})
		}
	}
}

func TestChecksAnnotationNamespaceReadError(t *testing.T) {
	for _, k := range []kind{instanceKind, loadBalancerKind} {
		t.Run(k.name, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			e.putObj(t, k, s4Obj{})
			e.srv.Fail("GET", s4MustPath(t, kube.Namespaces, s4Namespace), 500)

			e.pass(t)

			obj := e.get(t, k)
			if s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer added on a namespace read error")
			}
			s4WantNamespaceError(t, obj)
		})
	}
}

func TestChecksSecretAnnotationMissingAtCreation(t *testing.T) {
	k := scalewaySecretKind
	e := s4Setup(t)
	e.putNamespace(t, "")
	e.putRole(t, "Editor")
	e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}})

	e.pass(t)

	obj := e.get(t, k)
	if !s4HasFinalizer(obj, k.finalizer) {
		t.Fatal("finalizer removed, want present")
	}
	s4WantStatus(t, obj, s4ErrorStatus(k, s4NoAnnotationMessage))
}

func TestChecksSecretAnnotationNamespaceReadError(t *testing.T) {
	k := scalewaySecretKind
	e := s4Setup(t)
	e.valid(t)
	e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}})
	e.srv.Fail("GET", s4MustPath(t, kube.Namespaces, s4Namespace), 500)

	e.pass(t)

	obj := e.get(t, k)
	if !s4HasFinalizer(obj, k.finalizer) {
		t.Fatal("finalizer removed, want present")
	}
	s4WantNamespaceError(t, obj)
}

func TestChecksSecretAnnotationNoUUIDCheck(t *testing.T) {
	k := scalewaySecretKind
	e := s4Setup(t)
	e.putNamespace(t, "not-a-uuid")
	e.putRole(t, "Editor")
	e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}})

	e.pass(t)

	if got := s4Status(e.get(t, k))["sync_state"]; got != "Synced" {
		t.Fatalf("sync_state = %v, want Synced (no UUID check for ScalewaySecret)", got)
	}
}

func TestChecksSecretAnnotationOnlyAtCreation(t *testing.T) {
	k := scalewaySecretKind
	e := s4Setup(t)
	e.putNamespace(t, "")
	e.putRole(t, "Editor")
	e.putObj(t, k, s4Obj{
		finalizers: []string{k.finalizer},
		status: map[string]any{
			"scaleway_id":         s4ExistingID,
			"current_version":     1,
			"observed_generation": 1,
			"sync_state":          "Synced",
		},
	})

	e.pass(t)

	if got := s4Status(e.get(t, k))["sync_state"]; got != "Synced" {
		t.Fatalf("sync_state = %v, want Synced", got)
	}
	if p := e.patches(k, true); len(p) != 0 {
		t.Fatalf("status patches = %d, want 0", len(p))
	}
}

func TestChecksSecretNoSource(t *testing.T) {
	sources := map[string]any{
		"no source":              nil,
		"empty source":           map[string]any{},
		"null kubernetes_secret": map[string]any{"kubernetes_secret": nil},
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			k := scalewaySecretKind
			e := s4Setup(t)
			e.valid(t)
			spec := s4Spec(k)
			if src == nil {
				delete(spec, "source")
			} else {
				spec["source"] = src
			}
			e.putObj(t, k, s4Obj{finalizers: []string{k.finalizer}, spec: spec})

			e.pass(t)

			obj := e.get(t, k)
			if !s4HasFinalizer(obj, k.finalizer) {
				t.Fatal("finalizer removed, want present")
			}
			s4WantStatus(t, obj, s4ErrorStatus(k, s4NoSourceMessage))
		})
	}
}

func TestChecksSpecListsOutOfListPass(t *testing.T) {
	cases := []struct {
		k     kind
		field string
		value string
	}{
		{instanceKind, "zone", "fr-par-9"},
		{loadBalancerKind, "zone", "fr-par-9"},
		{instanceKind, "instance_type", "DEV1-XXL"},
		{loadBalancerKind, "lb_type", "LB-XL"},
	}
	for _, c := range cases {
		t.Run(c.k.name+"/"+c.field, func(t *testing.T) {
			e := s4Setup(t)
			e.valid(t)
			spec := s4Spec(c.k)
			spec[c.field] = c.value
			e.putObj(t, c.k, s4Obj{finalizers: []string{c.k.finalizer}, spec: spec})

			e.pass(t)

			obj := e.get(t, c.k)
			if !s4HasFinalizer(obj, c.k.finalizer) {
				t.Fatal("finalizer removed, want present")
			}
			if st := s4Status(obj); st != nil {
				t.Fatalf("status = %v, want none", st)
			}
			if p := e.patches(c.k, true); len(p) != 0 {
				t.Fatalf("status patches = %d, want 0", len(p))
			}
		})
	}
}

func s4DecodeObj(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(s4Raw(t, map[string]any{"spec": spec})))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}

func TestChecksSpecListsValid(t *testing.T) {
	zones := []string{"fr-par-1", "fr-par-2", "nl-ams-1", "pl-waw-1", "sg-sin-1", "it-mil-1"}
	types := []string{
		"DEV1-S", "DEV1-M", "DEV1-L", "DEV1-XL",
		"GP1-XS", "GP1-S", "GP1-M", "GP1-L", "GP1-XL",
		"CPU1-XS", "CPU1-S", "CPU1-M", "CPU1-L",
		"GPU-3090", "GPU-4090",
	}
	lbTypes := []string{"LB-S", "LB-GP"}

	inst := func(zone, typ string) map[string]any {
		s := s4Spec(instanceKind)
		s["zone"], s["instance_type"] = zone, typ
		return s4DecodeObj(t, s)
	}
	lb := func(zone, typ string) map[string]any {
		s := s4Spec(loadBalancerKind)
		s["zone"], s["lb_type"] = zone, typ
		return s4DecodeObj(t, s)
	}

	for _, z := range zones {
		if !specListsValid(instanceKind, inst(z, "DEV1-S")) {
			t.Errorf("Instance zone %s rejected", z)
		}
		if !specListsValid(loadBalancerKind, lb(z, "LB-S")) {
			t.Errorf("LoadBalancer zone %s rejected", z)
		}
	}
	for _, ty := range types {
		if !specListsValid(instanceKind, inst("fr-par-1", ty)) {
			t.Errorf("instance_type %s rejected", ty)
		}
	}
	for _, ty := range lbTypes {
		if !specListsValid(loadBalancerKind, lb("fr-par-1", ty)) {
			t.Errorf("lb_type %s rejected", ty)
		}
	}

	for _, z := range []string{"fr-par-3", "FR-PAR-1", "fr-par", "nl-ams-2", "pl-waw-2", ""} {
		if specListsValid(instanceKind, inst(z, "DEV1-S")) {
			t.Errorf("Instance zone %q accepted", z)
		}
		if specListsValid(loadBalancerKind, lb(z, "LB-S")) {
			t.Errorf("LoadBalancer zone %q accepted", z)
		}
	}
	for _, ty := range []string{"DEV1-XXL", "dev1-s", "GP1-XXS", "PLAY2-NANO", "GPU-3070", ""} {
		if specListsValid(instanceKind, inst("fr-par-1", ty)) {
			t.Errorf("instance_type %q accepted", ty)
		}
	}
	for _, ty := range []string{"LB-M", "lb-s", "LB-GP-M", "LB-XL", ""} {
		if specListsValid(loadBalancerKind, lb("fr-par-1", ty)) {
			t.Errorf("lb_type %q accepted", ty)
		}
	}
}
