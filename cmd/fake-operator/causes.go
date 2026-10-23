package main

import "context"

const (
	outcomeAnnotation       = "fake.scaleway-manager.test/outcome"
	messageAnnotation       = "fake.scaleway-manager.test/message"
	sourceVersionAnnotation = "fake.scaleway-manager.test/source-version"
)

// revokedBeforeFinalizer reports a Revoked cause checked before the finalizer.
// Story 3 stub: never.
func revokedBeforeFinalizer(k kind, obj map[string]any) bool { return false }

// applyCause applies the outcome annotation. Story 3 stub: no cause.
func (o *operator) applyCause(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	return false, nil
}
