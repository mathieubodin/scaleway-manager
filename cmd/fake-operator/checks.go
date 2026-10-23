package main

import "context"

// checkBefore runs the checks before the finalizer. Story 3 stub: no check.
func (o *operator) checkBefore(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	return false, nil
}

// checkAfter runs the checks after the finalizer. Story 3 stub: no check.
func (o *operator) checkAfter(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	return false, nil
}

// specListsValid checks the spec lists. Story 3 stub: always valid.
func specListsValid(k kind, obj map[string]any) bool { return true }
