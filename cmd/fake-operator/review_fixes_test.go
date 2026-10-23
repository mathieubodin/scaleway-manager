package main

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests of the behavior changes made by the review of #42. They reuse the
// helpers of causes_test.go.

// stopClock closes stop the first time Now is called: Instance creation reads
// the clock while its object is in flight.
type stopClock struct {
	once sync.Once
	stop chan struct{}
}

func (c *stopClock) Now() time.Time {
	c.once.Do(func() { close(c.stop) })
	return causesNow
}

// F9: a spec change pushes a version whenever scaleway_id is set, whatever
// sync_state; leaving Revoked pushes one too (F1 of the second review).
func TestReviewFixSpecChangePushesWhateverSyncState(t *testing.T) {
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

// F4: once the signal arrives, the pass finishes the in-flight object and
// starts no other.
func TestReviewFixSignalStopsPassBetweenObjects(t *testing.T) {
	e := causesSetup(t)
	clock := &stopClock{stop: make(chan struct{})}
	e.o.clock = clock
	e.o.stop = clock.stop
	e.put(t, instanceKind, causesObject(instanceKind, "vm-a", 1, nil, true, nil))
	e.put(t, instanceKind, causesObject(instanceKind, "vm-b", 1, nil, true, nil))
	before := len(e.s.Requests())

	e.pass(t)

	if st := e.status(t, instanceKind, "vm-a"); st["sync_state"] != "Syncing" || st["scaleway_id"] == nil {
		t.Errorf("vm-a (in flight): status %v, want its creation written", st)
	}
	if st := e.status(t, instanceKind, "vm-b"); len(st) != 0 {
		t.Errorf("vm-b (next object): status %v, want none", st)
	}
	for _, r := range e.s.Requests()[before:] {
		if strings.Contains(r.RequestURI, "/vm-b") || strings.Contains(r.RequestURI, "/loadbalancers") ||
			strings.Contains(r.RequestURI, "/scalewaysecrets") {
			t.Errorf("request after the signal: %s %s", r.Method, r.RequestURI)
		}
	}
}
