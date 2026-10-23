package main

import "testing"

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
