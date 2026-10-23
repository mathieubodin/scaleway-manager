package main

import "context"

const (
	outcomeAnnotation       = "fake.scaleway-manager.test/outcome"
	messageAnnotation       = "fake.scaleway-manager.test/message"
	sourceVersionAnnotation = "fake.scaleway-manager.test/source-version"
)

func revokedBeforeFinalizer(k kind, obj map[string]any) bool { return false }

func (o *operator) applyCause(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	return false, errNotImplemented
}
