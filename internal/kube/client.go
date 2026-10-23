package kube

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrUnknownResource     = errors.New("kube: unknown resource")
	ErrOperationNotAllowed = errors.New("kube: operation not allowed")
	ErrInvalidName         = errors.New("kube: invalid name")
	ErrEmptyToken          = errors.New("kube: empty token")
	ErrNoAPIServer         = errors.New("kube: KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT not set")
	ErrNoCA                = errors.New("kube: no certificate in CA PEM")
	ErrBadResponse         = errors.New("kube: bad response")
)

// APIError is a non-2xx answer of the API server.
type APIError struct {
	Method string
	Path   string
	Code   int
	Body   []byte
}

func (e *APIError) Error() string { return "" }

// Client calls the API server with the token each call receives.
type Client struct{}

func NewInCluster(getenv func(string) string, caPEM []byte) (*Client, error) {
	return nil, errNotImplemented
}

func (c *Client) List(ctx context.Context, tok Token, r Resource, namespace string) ([]json.RawMessage, error) {
	return nil, errNotImplemented
}

func (c *Client) Get(ctx context.Context, tok Token, r Resource, namespace, name string) (json.RawMessage, error) {
	return nil, errNotImplemented
}

func (c *Client) MergePatch(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error) {
	return nil, errNotImplemented
}

func (c *Client) PatchStatus(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error) {
	return nil, errNotImplemented
}
