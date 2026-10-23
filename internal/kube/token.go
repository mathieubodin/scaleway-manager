// Package kube is the only Kubernetes API client (AD-2, AD-3, AD-4).
package kube

import "log/slog"

const redacted = "[REDACTED]"

// Token is a bearer token, redacted wherever it is printed.
type Token string

// String returns [REDACTED], never the token.
func (t Token) String() string { return redacted }

// GoString returns [REDACTED], never the token.
func (t Token) GoString() string { return redacted }

// LogValue returns [REDACTED], never the token.
func (t Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns "[REDACTED]", never the token.
func (t Token) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }
