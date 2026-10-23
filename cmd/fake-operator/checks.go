package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// Messages of the operator's checks (scaleway-operator v0.1.15).
const (
	checkMsgNoNamespaceRole = "Configuration error: No NamespaceRole found for namespace '%s'. " +
		"Create a NamespaceRole resource with name '%s' to assign a Scaleway role."
	checkMsgAPIError        = "Kubernetes API error"
	checkMsgReadOnly        = "Configuration error: Role '%s' is read-only and cannot %s. Use 'Editor' or 'Admin'."
	checkMsgNoAnnotation    = "Configuration error: Namespace '%s' must have annotation 'scaleway.mathieubodin.io/project-id'"
	checkMsgNamespaceAccess = "Configuration error: Cannot access namespace %s: %s"
	checkMsgNotUUID         = "Configuration error: Annotation 'scaleway.mathieubodin.io/project-id' must be a valid UUID, got: '%s'"
	checkMsgNoSource        = "Secret source not configured: spec.source must have kubernetes_secret configured"
)

// Lists of the operator's src/scaleway.rs and allowed roles.
var (
	checkAllowedRoles  = []string{"Editor", "Admin", "OrganizationOwner"}
	checkZones         = []string{"fr-par-1", "fr-par-2", "nl-ams-1", "pl-waw-1", "sg-sin-1", "it-mil-1"}
	checkInstanceTypes = []string{
		"DEV1-S", "DEV1-M", "DEV1-L", "DEV1-XL",
		"GP1-XS", "GP1-S", "GP1-M", "GP1-L", "GP1-XL",
		"CPU1-XS", "CPU1-S", "CPU1-M", "CPU1-L",
		"GPU-3090", "GPU-4090",
	}
	checkLBTypes = []string{"LB-S", "LB-GP"}
)

// checkBefore runs the checks before the finalizer: the NamespaceRole, then
// the read-only role (ScalewaySecret) or the project-id annotation (Instance,
// LoadBalancer). A failure writes Error and reports failed.
func (o *operator) checkBefore(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	ns := metaString(obj, "namespace")
	role, msg := o.checkRole(ctx, ns)
	if msg != "" {
		return true, o.checkFail(ctx, k, obj, msg)
	}
	if k.resource == kube.ScalewaySecrets {
		if !slices.Contains(checkAllowedRoles, role) {
			return true, o.checkFail(ctx, k, obj, fmt.Sprintf(checkMsgReadOnly, role, "sync secrets"))
		}
		return false, nil
	}
	pid, msg := o.checkProjectID(ctx, ns)
	if msg == "" && !checkIsUUID(pid) {
		msg = fmt.Sprintf(checkMsgNotUUID, pid)
	}
	if msg != "" {
		return true, o.checkFail(ctx, k, obj, msg)
	}
	return false, nil
}

// checkAfter runs the checks after the finalizer: the read-only role while
// scaleway_id is null or empty (Instance, LoadBalancer); the source, then the
// project-id annotation at creation (ScalewaySecret).
func (o *operator) checkAfter(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	ns := metaString(obj, "namespace")
	created := hasScalewayID(currentStatus(k, obj))
	if k.resource == kube.ScalewaySecrets {
		spec, _ := obj["spec"].(map[string]any)
		src, _ := spec["source"].(map[string]any)
		if src["kubernetes_secret"] == nil {
			return true, o.checkFail(ctx, k, obj, checkMsgNoSource)
		}
		if created {
			return false, nil
		}
		if _, msg := o.checkProjectID(ctx, ns); msg != "" {
			return true, o.checkFail(ctx, k, obj, msg)
		}
		return false, nil
	}
	if created {
		return false, nil
	}
	role, msg := o.checkRole(ctx, ns)
	if msg == "" && !slices.Contains(checkAllowedRoles, role) {
		what := "create instances"
		if k.resource == kube.LoadBalancers {
			what = "create load balancers"
		}
		msg = fmt.Sprintf(checkMsgReadOnly, role, what)
	}
	if msg != "" {
		return true, o.checkFail(ctx, k, obj, msg)
	}
	return false, nil
}

// specListsValid checks spec.zone, spec.instance_type and spec.lb_type
// against the lists of the operator's src/scaleway.rs, then that spec.name is
// not empty (validate_spec, validate_lb_spec).
func specListsValid(k kind, obj map[string]any) bool {
	spec, _ := obj["spec"].(map[string]any)
	str := func(key string) string {
		s, _ := spec[key].(string)
		return s
	}
	switch k.resource {
	case kube.Instances:
		return slices.Contains(checkZones, str("zone")) && slices.Contains(checkInstanceTypes, str("instance_type")) && str("name") != ""
	case kube.LoadBalancers:
		return slices.Contains(checkZones, str("zone")) && slices.Contains(checkLBTypes, str("lb_type")) && str("name") != ""
	}
	return true
}

// checkFail writes sync_state Error with msg, keeping the other fields.
func (o *operator) checkFail(ctx context.Context, k kind, obj map[string]any, msg string) error {
	s := currentStatus(k, obj)
	s["sync_state"] = "Error"
	s["error_message"] = msg
	return o.writeStatus(ctx, k, obj, s)
}

// checkRole reads spec.scaleway_role of the NamespaceRole named ns; an absent
// field is the empty role. A non-empty msg is the failure message.
func (o *operator) checkRole(ctx context.Context, ns string) (role, msg string) {
	raw, err := o.kube.Get(ctx, o.token, kube.NamespaceRoles, "", ns)
	if err != nil {
		var apiErr *kube.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			return "", fmt.Sprintf(checkMsgNoNamespaceRole, ns, ns)
		}
		return "", checkMsgAPIError
	}
	nr, err := decodeObject(raw)
	if err != nil {
		return "", checkMsgAPIError
	}
	spec, _ := nr["spec"].(map[string]any)
	role, _ = spec["scaleway_role"].(string)
	return role, ""
}

// checkProjectID reads the namespace's project-id annotation; a present
// annotation counts even when empty, as in the operator. A non-empty msg is
// the failure message.
func (o *operator) checkProjectID(ctx context.Context, ns string) (pid, msg string) {
	raw, err := o.kube.Get(ctx, o.token, kube.Namespaces, "", ns)
	if err == nil {
		var nsObj map[string]any
		if nsObj, err = decodeObject(raw); err == nil {
			ann, _ := metadata(nsObj)["annotations"].(map[string]any)
			p, ok := ann[projectIDAnnotation].(string)
			if !ok {
				return "", fmt.Sprintf(checkMsgNoAnnotation, ns)
			}
			return p, ""
		}
	}
	return "", fmt.Sprintf(checkMsgNamespaceAccess, ns, err.Error())
}

// checkIsUUID accepts the forms of Rust's Uuid::parse_str: hyphenated,
// 32 hex digits, braced, urn:uuid: prefixed; hex digits in any case.
func checkIsUUID(s string) bool {
	switch len(s) {
	case 32:
		return checkIsHex(s)
	case 36:
	case 38:
		if s[0] != '{' || s[37] != '}' {
			return false
		}
		s = s[1:37]
	case 45:
		var ok bool
		if s, ok = strings.CutPrefix(s, "urn:uuid:"); !ok {
			return false
		}
	default:
		return false
	}
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if s[i] != '-' {
				return false
			}
		} else if !checkIsHex(s[i : i+1]) {
			return false
		}
	}
	return true
}

func checkIsHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}
