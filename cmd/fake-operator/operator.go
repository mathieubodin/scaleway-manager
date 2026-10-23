// Command fake-operator mimics scaleway-operator v0.1.15 for automated tests (AD-9).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// Clock gives the current time; tests inject a fixed one.
type Clock interface{ Now() time.Time }

type kind struct {
	resource  kube.Resource
	name      string // the <Kind> of messages
	finalizer string
}

var (
	instanceKind       = kind{kube.Instances, "Instance", "scaleway.mathieubodin.io/instance-finalizer"}
	loadBalancerKind   = kind{kube.LoadBalancers, "LoadBalancer", "scaleway.mathieubodin.io/loadbalancer-finalizer"}
	scalewaySecretKind = kind{kube.ScalewaySecrets, "ScalewaySecret", "scaleway.mathieubodin.io/secret-finalizer"}
	kinds              = []kind{instanceKind, loadBalancerKind, scalewaySecretKind}
)

type operator struct {
	kube      *kube.Client
	clock     Clock
	rand      *rand.Rand
	tokenPath string
	log       *slog.Logger
	token     kube.Token      // read at the start of each pass
	stop      <-chan struct{} // closed on a signal: the pass ends before the next object; nil never stops
}

func newOperator(c *kube.Client, clock Clock, rnd *rand.Rand, tokenPath string, log *slog.Logger) *operator {
	return &operator{kube: c, clock: clock, rand: rnd, tokenPath: tokenPath, log: log}
}

var errEmptyToken = errors.New("fake-operator: empty token file")

// readToken reads the service account token file, trimmed. An empty result is an error.
func readToken(path string) (kube.Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("fake-operator: read token: %w", err)
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", errEmptyToken
	}
	return kube.Token(t), nil
}

// stopped reports whether a signal asked the pass to end.
func (o *operator) stopped() bool {
	select {
	case <-o.stop:
		return true
	default:
		return false
	}
}

// pass moves every object of the three kinds at most one step. It returns an
// error only when the token cannot be read; list and object errors are logged.
// Once o.stop is closed it starts no other request: the in-flight object ends,
// the next one is not started.
func (o *operator) pass(ctx context.Context) error {
	tok, err := readToken(o.tokenPath)
	if err != nil {
		return err
	}
	o.token = tok
	for _, k := range kinds {
		if o.stopped() {
			return nil
		}
		items, err := o.kube.List(ctx, o.token, k.resource, "")
		if err != nil {
			o.log.Error("list failed", "kind", k.name, "error", err.Error())
			continue
		}
		for _, raw := range items {
			if o.stopped() {
				return nil
			}
			obj, err := decodeObject(raw)
			if err == nil {
				err = o.reconcile(ctx, k, obj)
			}
			if err != nil {
				o.log.Error("object failed", "kind", k.name, "namespace", metaString(obj, "namespace"),
					"name", metaString(obj, "name"), "error", err.Error())
			}
		}
	}
	return nil
}

// reconcile applies the pass order of go-interfaces.md to one object. A step
// that reports handled or failed ends the object's pass.
func (o *operator) reconcile(ctx context.Context, k kind, obj map[string]any) error {
	if handled, err := o.deleting(ctx, k, obj); err != nil || handled {
		return err
	}
	if failed, err := o.checkBefore(ctx, k, obj); err != nil || failed {
		return err
	}
	if revokedBeforeFinalizer(k, obj) {
		_, err := o.applyCause(ctx, k, obj)
		return err
	}
	if !hasFinalizer(k, obj) {
		return o.setFinalizer(ctx, k, obj, true)
	}
	if failed, err := o.checkAfter(ctx, k, obj); err != nil || failed {
		return err
	}
	if !specListsValid(k, obj) {
		return nil
	}
	if handled, err := o.applyCause(ctx, k, obj); err != nil || handled {
		return err
	}
	return o.lifecycle(ctx, k, obj)
}

// decodeObject decodes an object with UseNumber (AD-4).
func decodeObject(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("fake-operator: decode object: %w", err)
	}
	if m == nil {
		return nil, errors.New("fake-operator: object is not a JSON object")
	}
	return m, nil
}

func metadata(obj map[string]any) map[string]any {
	m, _ := obj["metadata"].(map[string]any)
	return m
}

func metaString(obj map[string]any, key string) string {
	s, _ := metadata(obj)[key].(string)
	return s
}
