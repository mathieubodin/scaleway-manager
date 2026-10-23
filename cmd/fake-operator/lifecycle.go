package main

import "context"

func (o *operator) writeStatus(ctx context.Context, k kind, obj map[string]any, status map[string]any) error {
	return errNotImplemented
}

func (o *operator) setFinalizer(ctx context.Context, k kind, obj map[string]any, present bool) error {
	return errNotImplemented
}

func defaultStatus(k kind) map[string]any { return nil }

func currentStatus(k kind, obj map[string]any) map[string]any { return nil }
