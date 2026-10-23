package main

import (
	"testing"
	"time"
)

// Tests of the third round of review fixes of #42. They reuse the helpers of
// causes_test.go.

// F8: NotFound on an object with no status writes the NotFound status, even
// though it equals defaultStatus.
func TestReviewFix3NotFoundWithoutStatus(t *testing.T) {
	for _, tc := range []struct {
		k    kind
		want map[string]any
	}{
		{instanceKind, map[string]any{"state": "unknown", "sync_state": "Syncing"}},
		{loadBalancerKind, map[string]any{"state": "", "sync_state": "Pending"}},
	} {
		t.Run(tc.k.name, func(t *testing.T) {
			e := causesSetup(t)
			e.put(t, tc.k, causesObject(tc.k, "obj", 1, causesOutcome("NotFound"), true, nil))

			e.pass(t)
			if _, ok := e.get(t, tc.k, "obj")["status"]; !ok {
				t.Fatal("no status written")
			}
			causesWant(t, e.status(t, tc.k, "obj"), tc.want)

			causesNoPatch(t, e.passPatches(t))
		})
	}
}

// F1: a ScalewaySecret leaving Revoked pushes a version, as the operator does:
// its Revoked status clears last_synced_resource_version (decide_next_action_secret
// step 7).
func TestReviewFix3LeavingRevokedPushes(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "sync_state", "Revoked", "error_message", "revoked")
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, nil, true, st))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "current_version", 3))

	causesNoPatch(t, e.passPatches(t))
}

// F2: an empty spec.name is refused with no status write, as validate_spec and
// validate_lb_spec do ("name cannot be empty").
func TestReviewFix3EmptyNameRefused(t *testing.T) {
	for _, k := range []kind{instanceKind, loadBalancerKind} {
		t.Run(k.name, func(t *testing.T) {
			e := causesSetup(t)
			obj := causesObject(k, "obj", 1, nil, true, nil)
			obj["spec"].(map[string]any)["name"] = ""
			e.put(t, k, obj)

			causesNoPatch(t, e.passPatches(t))
			if _, ok := e.get(t, k, "obj")["status"]; ok {
				t.Error("status written for an empty spec.name")
			}
		})
	}
}

// F3: created_at carries the fraction of a second as chrono's RFC 3339 with
// SecondsFormat::AutoSi: none, 3, 6 or 9 digits.
func TestReviewFix3CreatedAtFraction(t *testing.T) {
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

// F12: source-version is bounded by the operator's u32 current_version.
func TestReviewFix3SourceVersionU32(t *testing.T) {
	t.Run("max", func(t *testing.T) {
		e := causesSetup(t)
		e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "4294967295"}, true, causesSyncedSecret()))

		e.pass(t)
		causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "current_version", 4294967295))
	})
	t.Run("above", func(t *testing.T) {
		e := causesSetup(t)
		e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "4294967296"}, true, causesSyncedSecret()))

		e.pass(t)
		got := e.status(t, scalewaySecretKind, "sec")
		want := "fake-operator: source-version '4294967296' is not a positive integer"
		if got["sync_state"] != "Error" || got["error_message"] != want {
			t.Errorf("sync_state %v, error_message %v; want Error, %q", got["sync_state"], got["error_message"], want)
		}
	})
}
