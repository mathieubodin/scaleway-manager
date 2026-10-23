package main

import "testing"

// Tests of the second round of review fixes of #42. They reuse the helpers of
// causes_test.go.

// F3: a source-version lower than current_version is not applied: the
// lifecycle runs and leaves current_version unchanged.
func TestReviewFix2SourceVersionLowerLeavesVersion(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 1, map[string]string{causesSourceKey: "1"}, true, causesSyncedSecret()))

	causesNoPatch(t, e.passPatches(t))
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesSyncedSecret())
}

// F3: after a spec change, source-version "2" does not take current_version
// back from 3 to 2 on the next pass.
func TestReviewFix2SpecChangeThenNoFlip(t *testing.T) {
	e := causesSetup(t)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 2, map[string]string{causesSourceKey: "2"}, true, causesSyncedSecret()))
	want := causesWith(causesSyncedSecret(), "current_version", 3, "observed_generation", 2)

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), want)

	causesNoPatch(t, e.passPatches(t))
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), want)
}

// observed_generation null with a scaleway_id: the generation is recorded
// without a version push (operator decide_next_action_secret step 7bis).
func TestReviewFix2NullObservedGenerationRecordedWithoutPush(t *testing.T) {
	e := causesSetup(t)
	st := causesWith(causesSyncedSecret(), "observed_generation", nil)
	e.put(t, scalewaySecretKind, causesObject(scalewaySecretKind, "sec", 4, nil, true, st))

	e.pass(t)
	causesWant(t, e.status(t, scalewaySecretKind, "sec"), causesWith(causesSyncedSecret(), "observed_generation", 4))

	causesNoPatch(t, e.passPatches(t))
}
