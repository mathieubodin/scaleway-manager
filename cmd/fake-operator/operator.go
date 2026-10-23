// Command fake-operator mimics scaleway-operator v0.1.15 for automated tests (AD-9).
//
// Contract stub (#42): the bodies are written by stories 3 to 6.
package main

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

var errNotImplemented = errors.New("fake-operator: not implemented")

// Clock gives the current time; tests inject a fixed one.
type Clock interface{ Now() time.Time }

type kind struct {
	resource  kube.Resource
	name      string
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
	token     kube.Token
}

func newOperator(c *kube.Client, clock Clock, rnd *rand.Rand, tokenPath string, log *slog.Logger) *operator {
	return &operator{kube: c, clock: clock, rand: rnd, tokenPath: tokenPath, log: log}
}

func readToken(path string) (kube.Token, error) { return "", errNotImplemented }

func (o *operator) pass(ctx context.Context) error { return errNotImplemented }
