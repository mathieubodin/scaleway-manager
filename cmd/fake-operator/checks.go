package main

import "context"

func (o *operator) checkBefore(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	return false, errNotImplemented
}

func (o *operator) checkAfter(ctx context.Context, k kind, obj map[string]any) (failed bool, err error) {
	return false, errNotImplemented
}

func specListsValid(k kind, obj map[string]any) bool { return false }
