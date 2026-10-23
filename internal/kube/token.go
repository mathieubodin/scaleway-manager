// Package kube is the only Kubernetes API client (AD-2, AD-3, AD-4).
//
// Contract stub (#42): the bodies are written by story 1.
package kube

import "log/slog"

// Token is a bearer token, redacted wherever it is printed.
type Token string

func (t Token) String() string { return "" }

func (t Token) GoString() string { return "" }

func (t Token) LogValue() slog.Value { return slog.Value{} }

func (t Token) MarshalJSON() ([]byte, error) { return nil, errNotImplemented }
