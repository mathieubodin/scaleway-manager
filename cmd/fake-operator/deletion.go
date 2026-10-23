package main

import "context"

func (o *operator) deleting(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	return false, errNotImplemented
}
