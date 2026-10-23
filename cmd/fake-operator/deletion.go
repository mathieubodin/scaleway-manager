package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// checkLockedMessage is the TerminationBlocked message of a locked LoadBalancer.
const checkLockedMessage = "Load balancer is locked — deletion blocked by Scaleway"

// deleting runs the ScalewaySecret region check, deletion included, then the
// deletion rules when deletionTimestamp is set. It reports handled when the
// object's pass ends here.
func (o *operator) deleting(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	const (
		outcomeKey = "fake.scaleway-manager.test/outcome"
		messageKey = "fake.scaleway-manager.test/message"
	)
	if k.resource == kube.ScalewaySecrets {
		spec, _ := obj["spec"].(map[string]any)
		region, _ := spec["region"].(string)
		if !slices.Contains([]string{"fr-par", "nl-ams", "pl-waw"}, region) {
			msg := fmt.Sprintf("Configuration error: Invalid region '%s'. Use one of: fr-par, nl-ams, pl-waw.", region)
			return true, o.checkFail(ctx, k, obj, msg)
		}
	}
	if metadata(obj)["deletionTimestamp"] == nil {
		return false, nil
	}
	ann, _ := metadata(obj)["annotations"].(map[string]any)
	outcome, _ := ann[outcomeKey].(string)
	// The operator calls Scaleway on delete only when it has a scaleway_id;
	// without one, a cause cannot block the deletion.
	s := currentStatus(k, obj)
	switch {
	case !hasScalewayID(s):
	case outcome == "Error":
		return true, nil
	case outcome == "Locked" && k.resource == kube.LoadBalancers:
		msg := checkLockedMessage
		if m, _ := ann[messageKey].(string); m != "" {
			msg = m
		}
		s["sync_state"] = "TerminationBlocked"
		s["error_message"] = msg
		return true, o.writeStatus(ctx, k, obj, s)
	}
	return true, o.setFinalizer(ctx, k, obj, false)
}
