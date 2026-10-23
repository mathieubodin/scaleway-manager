package main

import (
	"context"
	"strconv"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const (
	outcomeAnnotation       = "fake.scaleway-manager.test/outcome"
	messageAnnotation       = "fake.scaleway-manager.test/message"
	sourceVersionAnnotation = "fake.scaleway-manager.test/source-version"
)

// Causes (companion "Causes").
const (
	causeNotFound = "NotFound"
	causeError    = "Error"
	causeLocked   = "Locked"
	causeRevoked  = "Revoked"
)

// Messages the operator writes for a cause.
const (
	apiErrorMessage          = "Scaleway API error: 500 Internal Server Error"
	revokedMessage           = "Source Secret opt-in revoked — Scaleway version disabled. Re-add label/annotation to re-sync."
	sourceUnavailableMessage = "Source Secret unavailable (see operator logs)"
)

// causesByKind lists the causes each kind supports; any other value is refused.
var causesByKind = map[kube.Resource]map[string]bool{
	kube.Instances:       {causeNotFound: true, causeError: true},
	kube.LoadBalancers:   {causeNotFound: true, causeError: true, causeLocked: true},
	kube.ScalewaySecrets: {causeError: true, causeRevoked: true},
}

func annotation(obj map[string]any, key string) string {
	a, _ := metadata(obj)["annotations"].(map[string]any)
	v, _ := a[key].(string)
	return v
}

// causeMessage is the message annotation when non-empty, else def.
func causeMessage(obj map[string]any, def string) string {
	if m := annotation(obj, messageAnnotation); m != "" {
		return m
	}
	return def
}

func hasScalewayID(s map[string]any) bool {
	id, _ := s["scaleway_id"].(string)
	return id != ""
}

// revokedBeforeFinalizer reports a Revoked cause on a ScalewaySecret with no
// scaleway_id: the operator fails it before adding the finalizer.
func revokedBeforeFinalizer(k kind, obj map[string]any) bool {
	return k.resource == kube.ScalewaySecrets &&
		annotation(obj, outcomeAnnotation) == causeRevoked &&
		!hasScalewayID(currentStatus(k, obj))
}

// applyCause applies the outcome annotation, then the source-version one. It
// reports handled when it wrote the pass's write or decided there is none.
func (o *operator) applyCause(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	cause := annotation(obj, outcomeAnnotation)
	if cause == "" {
		return o.applySourceVersion(ctx, k, obj)
	}
	s := currentStatus(k, obj)
	if !causesByKind[k.resource][cause] {
		s["sync_state"] = "Error"
		s["error_message"] = "fake-operator: cause '" + cause + "' is not supported for " + k.name
		return true, o.writeStatus(ctx, k, obj, s)
	}
	switch cause {
	case causeNotFound:
		s["scaleway_id"] = nil
		s["project_id"] = nil
		s["error_message"] = nil
		if k.resource == kube.Instances {
			s["public_ip"] = nil
			s["created_at"] = nil
			s["state"] = "unknown"
			s["sync_state"] = "Syncing"
		} else {
			s["vip_address"] = nil
			s["state"] = ""
			s["sync_state"] = "Pending"
		}
	case causeError:
		if k.resource == kube.ScalewaySecrets && hasScalewayID(s) {
			return true, nil
		}
		s["sync_state"] = "Error"
		s["error_message"] = causeMessage(obj, apiErrorMessage)
	case causeLocked:
		// Outside deletion a lock has no effect (deletion is story 4).
		return false, nil
	case causeRevoked:
		if !hasScalewayID(s) {
			s["sync_state"] = "Error"
			s["error_message"] = causeMessage(obj, sourceUnavailableMessage)
			break
		}
		s["sync_state"] = "Revoked"
		s["error_message"] = causeMessage(obj, revokedMessage)
		s["last_synced_resource_version"] = nil
	}
	return true, o.writeStatus(ctx, k, obj, s)
}

// applySourceVersion handles the source-version annotation of a ScalewaySecret
// with no cause: an invalid value is an Error whatever the state; a valid one
// different from current_version is pushed once the secret is Synced.
func (o *operator) applySourceVersion(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	if k.resource != kube.ScalewaySecrets {
		return false, nil
	}
	a, _ := metadata(obj)["annotations"].(map[string]any)
	v, present := a[sourceVersionAnnotation].(string)
	if !present {
		return false, nil
	}
	s := currentStatus(k, obj)
	n, perr := strconv.ParseUint(v, 10, 63)
	if perr != nil || n == 0 {
		s["sync_state"] = "Error"
		s["error_message"] = "fake-operator: source-version '" + v + "' is not a positive integer"
		return true, o.writeStatus(ctx, k, obj, s)
	}
	if s["sync_state"] != "Synced" || numString(s["current_version"]) == strconv.FormatUint(n, 10) {
		return false, nil
	}
	s["current_version"] = int64(n)
	s["observed_generation"] = metadata(obj)["generation"]
	s["last_synced_resource_version"] = nil
	s["error_message"] = nil
	s["sync_state"] = "Synced"
	return true, o.writeStatus(ctx, k, obj, s)
}
