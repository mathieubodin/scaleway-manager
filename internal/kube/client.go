package kube

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
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

const (
	requestTimeout  = 30 * time.Second
	maxResponseSize = 16 << 20
	mergePatchType  = "application/merge-patch+json"
)

// APIError is a non-2xx answer of the API server.
type APIError struct {
	Method string
	Path   string
	Code   int
	Body   []byte
}

// Error names the method, the path and the code only; never the body.
func (e *APIError) Error() string {
	return "kube: " + e.Method + " " + e.Path + ": " + strconv.Itoa(e.Code)
}

// Client calls the API server with the token each call receives.
type Client struct {
	host string
	http *http.Client
}

// NewInCluster builds a client for the API server named by KUBERNETES_SERVICE_HOST and
// KUBERNETES_SERVICE_PORT, trusting only the certificates in caPEM.
func NewInCluster(getenv func(string) string, caPEM []byte) (*Client, error) {
	host, port := getenv("KUBERNETES_SERVICE_HOST"), getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, ErrNoAPIServer
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, ErrNoCA
	}
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Client{
		host: net.JoinHostPort(host, port),
		http: &http.Client{Transport: transport, Timeout: requestTimeout},
	}, nil
}

// check applies the error precedence: unknown resource, operation not allowed,
// invalid name, empty token. It returns the request path.
func check(tok Token, r Resource, op operation, namespace, name string) (string, error) {
	info, ok := r.info()
	if !ok {
		return "", ErrUnknownResource
	}
	if info.ops&op == 0 {
		return "", ErrOperationNotAllowed
	}
	if op != opList && name == "" {
		return "", ErrInvalidName
	}
	if op == opList && r == Deployments && namespace == "" {
		return "", ErrInvalidName
	}
	p, err := Path(r, namespace, name, op == opPatchStatus)
	if err != nil {
		return "", err
	}
	if tok == "" {
		return "", ErrEmptyToken
	}
	return p, nil
}

// List returns the elements of items; an empty namespace on a namespaced kind lists across namespaces.
func (c *Client) List(ctx context.Context, tok Token, r Resource, namespace string) ([]json.RawMessage, error) {
	p, err := check(tok, r, opList, namespace, "")
	if err != nil {
		return nil, err
	}
	body, err := c.do(ctx, tok, http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	var list map[string]json.RawMessage
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrBadResponse, http.MethodGet, p, err)
	}
	raw, ok := list["items"]
	if !ok {
		return nil, fmt.Errorf("%w: %s %s: no items", ErrBadResponse, http.MethodGet, p)
	}
	items := []json.RawMessage{}
	if !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("%w: %s %s: items is not an array", ErrBadResponse, http.MethodGet, p)
		}
	}
	return items, nil
}

// Get returns one object.
func (c *Client) Get(ctx context.Context, tok Token, r Resource, namespace, name string) (json.RawMessage, error) {
	p, err := check(tok, r, opGet, namespace, name)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, tok, http.MethodGet, p, nil)
}

// MergePatch sends patch unchanged as a JSON merge patch on the object.
func (c *Client) MergePatch(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error) {
	p, err := check(tok, r, opMergePatch, namespace, name)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, tok, http.MethodPatch, p, patch)
}

// PatchStatus sends patch unchanged as a JSON merge patch on the status subresource.
func (c *Client) PatchStatus(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error) {
	p, err := check(tok, r, opPatchStatus, namespace, name)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, tok, http.MethodPatch, p, patch)
}

// do sends one request on path p (built by Path) and returns the 2xx body as JSON.
func (c *Client) do(ctx context.Context, tok Token, method, p string, patch []byte) (json.RawMessage, error) {
	u := url.URL{Scheme: "https", Host: c.host, Path: p}
	var body io.Reader
	if method == http.MethodPatch {
		body = bytes.NewReader(patch)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("kube: %s %s: %w", method, p, err)
	}
	req.Header.Set("Authorization", "Bearer "+string(tok))
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPatch {
		req.Header.Set("Content-Type", mergePatchType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kube: %s %s: %w", method, p, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("kube: %s %s: %w", method, p, err)
	}
	tooLarge := len(data) > maxResponseSize
	if tooLarge {
		data = data[:maxResponseSize]
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &APIError{Method: method, Path: p, Code: resp.StatusCode, Body: data}
	}
	if tooLarge {
		return nil, fmt.Errorf("%w: %s %s: body over %d bytes", ErrBadResponse, method, p, maxResponseSize)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: %s %s: body is not JSON", ErrBadResponse, method, p)
	}
	return json.RawMessage(data), nil
}
