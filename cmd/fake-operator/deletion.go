package main

import "context"

// deleting handles an object whose deletionTimestamp is set. Story 3 stub:
// it removes the kind's finalizer and reports handled.
func (o *operator) deleting(ctx context.Context, k kind, obj map[string]any) (handled bool, err error) {
	if metadata(obj)["deletionTimestamp"] == nil {
		return false, nil
	}
	return true, o.setFinalizer(ctx, k, obj, false)
}
