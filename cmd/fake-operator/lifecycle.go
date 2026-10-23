package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const projectIDAnnotation = "scaleway.mathieubodin.io/project-id"

// statusFields is the whole status struct of a kind (companion "Status fields").
func statusFields(k kind) []string {
	switch k.resource {
	case kube.Instances:
		return []string{"scaleway_id", "public_ip", "state", "project_id", "created_at", "error_message", "sync_state"}
	case kube.LoadBalancers:
		return []string{"scaleway_id", "vip_address", "state", "project_id", "created_at", "error_message", "sync_state"}
	default:
		return []string{"scaleway_id", "current_version", "last_synced_resource_version", "observed_generation", "sync_state", "error_message"}
	}
}

// defaultStatus is the Rust default of the kind's status, every field present.
func defaultStatus(k kind) map[string]any {
	s := map[string]any{}
	for _, f := range statusFields(k) {
		s[f] = nil
	}
	switch k.resource {
	case kube.Instances:
		s["state"] = "unknown"
		s["sync_state"] = "Syncing"
	case kube.LoadBalancers:
		s["state"] = ""
		s["sync_state"] = "Pending"
	default:
		s["sync_state"] = "Syncing"
	}
	return s
}

// currentStatus returns a copy of obj's status with every field of the kind
// (absent ones as nil), or defaultStatus(k) when obj has no status.
func currentStatus(k kind, obj map[string]any) map[string]any {
	st, ok := obj["status"].(map[string]any)
	if !ok {
		return defaultStatus(k)
	}
	return wholeStatus(k, st)
}

// wholeStatus keeps exactly the kind's fields, absent ones as nil.
func wholeStatus(k kind, st map[string]any) map[string]any {
	s := map[string]any{}
	for _, f := range statusFields(k) {
		s[f] = st[f]
	}
	return s
}

// writeStatus sends {"status":{...}} with every field of the kind, unless obj
// has a status equal to it, and updates obj in place from the response. A
// missing status always differs, even from defaultStatus.
func (o *operator) writeStatus(ctx context.Context, k kind, obj map[string]any, status map[string]any) error {
	next := wholeStatus(k, status)
	if _, ok := obj["status"].(map[string]any); ok {
		same, err := sameJSON(next, currentStatus(k, obj))
		if err != nil || same {
			return err
		}
	}
	body, err := json.Marshal(map[string]any{"status": next})
	if err != nil {
		return err
	}
	resp, err := o.kube.PatchStatus(ctx, o.token, k.resource, metaString(obj, "namespace"), metaString(obj, "name"), body)
	if err != nil {
		return err
	}
	return replaceObject(obj, resp)
}

// setFinalizer adds (present) or removes the kind's finalizer, keeping the
// others, and sends the whole list; nothing is sent when nothing changes.
func (o *operator) setFinalizer(ctx context.Context, k kind, obj map[string]any, present bool) error {
	if hasFinalizer(k, obj) == present {
		return nil
	}
	list := []any{}
	cur, _ := metadata(obj)["finalizers"].([]any)
	for _, f := range cur {
		if f != k.finalizer {
			list = append(list, f)
		}
	}
	if present {
		list = append(list, k.finalizer)
	}
	body, err := json.Marshal(map[string]any{"metadata": map[string]any{"finalizers": list}})
	if err != nil {
		return err
	}
	resp, err := o.kube.MergePatch(ctx, o.token, k.resource, metaString(obj, "namespace"), metaString(obj, "name"), body)
	if err != nil {
		return err
	}
	return replaceObject(obj, resp)
}

func hasFinalizer(k kind, obj map[string]any) bool {
	cur, _ := metadata(obj)["finalizers"].([]any)
	for _, f := range cur {
		if f == k.finalizer {
			return true
		}
	}
	return false
}

// lifecycle moves an object that passed every check one step.
func (o *operator) lifecycle(ctx context.Context, k kind, obj map[string]any) error {
	s := currentStatus(k, obj)
	if id, _ := s["scaleway_id"].(string); id == "" {
		return o.create(ctx, k, obj, s)
	}
	gen := metadata(obj)["generation"]
	switch {
	case k.resource == kube.ScalewaySecrets && s["sync_state"] == "Synced" && s["observed_generation"] == nil:
		// A generation never observed is recorded without a push, as the
		// operator's AlreadySynced does (decide_next_action_secret step 7bis).
		s["observed_generation"] = gen
		return o.writeStatus(ctx, k, obj, s)
	case k.resource == kube.ScalewaySecrets && s["sync_state"] != "Revoked" &&
		s["observed_generation"] != nil && numString(gen) != numString(s["observed_generation"]):
		// A spec change pushes a version whatever sync_state, except Revoked
		// (the operator stops there).
		v, err := intValue(s["current_version"])
		if err != nil {
			return err
		}
		s["current_version"] = v + 1
		s["observed_generation"] = gen
		s["last_synced_resource_version"] = nil
	case s["sync_state"] != "Synced":
		switch k.resource {
		case kube.Instances:
			s["state"] = "running"
			if s["public_ip"] == nil {
				s["public_ip"] = fmt.Sprintf("192.0.2.%d", o.octet())
			}
		case kube.LoadBalancers:
			s["state"] = "ready"
			if s["vip_address"] == nil {
				s["vip_address"] = fmt.Sprintf("198.51.100.%d", o.octet())
			}
		default:
			if s["current_version"] == nil {
				s["current_version"] = 1
			}
			s["observed_generation"] = gen
			s["last_synced_resource_version"] = nil
		}
	default:
		return nil
	}
	s["error_message"] = nil
	s["sync_state"] = "Synced"
	return o.writeStatus(ctx, k, obj, s)
}

// create is the creation step: Syncing, and for a ScalewaySecret a second
// patch to Synced in the same pass.
func (o *operator) create(ctx context.Context, k kind, obj map[string]any, s map[string]any) error {
	s["scaleway_id"] = o.uuid()
	s["error_message"] = nil
	s["sync_state"] = "Syncing"
	switch k.resource {
	case kube.Instances, kube.LoadBalancers:
		if k.resource == kube.Instances {
			s["state"] = "creating"
			s["created_at"] = o.clock.Now().UTC().Format(time.RFC3339)
		} else {
			s["state"] = "pending"
		}
		s["project_id"] = o.projectID(ctx, metaString(obj, "namespace"))
		return o.writeStatus(ctx, k, obj, s)
	}
	s["current_version"] = nil
	s["last_synced_resource_version"] = nil
	s["observed_generation"] = nil
	if err := o.writeStatus(ctx, k, obj, s); err != nil {
		return err
	}
	s = currentStatus(k, obj)
	s["current_version"] = 1
	s["observed_generation"] = metadata(obj)["generation"]
	s["last_synced_resource_version"] = nil
	s["error_message"] = nil
	s["sync_state"] = "Synced"
	return o.writeStatus(ctx, k, obj, s)
}

// projectID is the namespace's project-id annotation, or nil when the
// namespace or the annotation cannot be read (checking it is story 4's job).
func (o *operator) projectID(ctx context.Context, namespace string) any {
	raw, err := o.kube.Get(ctx, o.token, kube.Namespaces, "", namespace)
	if err != nil {
		return nil
	}
	ns, err := decodeObject(raw)
	if err != nil {
		return nil
	}
	ann, _ := metadata(ns)["annotations"].(map[string]any)
	if pid, ok := ann[projectIDAnnotation].(string); ok && pid != "" {
		return pid
	}
	return nil
}

// uuid draws a random UUID v4 from the injected rand.
func (o *operator) uuid() string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[0:8], o.rand.Uint64())
	binary.BigEndian.PutUint64(b[8:16], o.rand.Uint64())
	b[6] = 0x40 | b[6]&0x0f
	b[8] = 0x80 | b[8]&0x3f
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// octet draws an IP last octet in 1..254.
func (o *operator) octet() int { return o.rand.IntN(254) + 1 }

// numString is the decimal form of a JSON number, "" when absent.
func numString(v any) string {
	switch n := v.(type) {
	case nil:
		return ""
	case json.Number:
		return n.String()
	default:
		return fmt.Sprint(n)
	}
}

// intValue reads an integer status field; nil counts as 0.
func intValue(v any) (int64, error) {
	switch n := v.(type) {
	case nil:
		return 0, nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("fake-operator: status number %q is not an integer", n)
		}
		return i, nil
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	}
	return 0, fmt.Errorf("fake-operator: status number of type %T", v)
}

func sameJSON(a, b any) (bool, error) {
	ja, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	jb, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ja, jb), nil
}

// replaceObject replaces obj's content by the decoded API response.
func replaceObject(obj map[string]any, resp []byte) error {
	m, err := decodeObject(resp)
	if err != nil {
		return err
	}
	for key := range obj {
		delete(obj, key)
	}
	for key, v := range m {
		obj[key] = v
	}
	return nil
}
