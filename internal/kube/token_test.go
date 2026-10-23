package kube_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

const secretToken = "s3cr3t-token-value"

const redacted = "[REDACTED]"

// Matrix row Redaction: fmt verbs print [REDACTED] only.
func TestTokenFmtIsRedacted(t *testing.T) {
	tok := kube.Token(secretToken)

	if got := tok.String(); got != redacted {
		t.Errorf("String() = %q, want %q", got, redacted)
	}
	if got := tok.GoString(); got != redacted {
		t.Errorf("GoString() = %q, want %q", got, redacted)
	}
	if got := fmt.Sprint(tok); got != redacted {
		t.Errorf("fmt.Sprint = %q, want %q", got, redacted)
	}
	for _, verb := range []string{"%v", "%s", "%+v", "%#v"} {
		got := fmt.Sprintf(verb, tok)
		if got != redacted {
			t.Errorf("fmt.Sprintf(%q) = %q, want %q", verb, got, redacted)
		}
	}
	for _, verb := range []string{"%q", "%x", "%X"} {
		got := fmt.Sprintf(verb, tok)
		if strings.Contains(got, secretToken) || strings.Contains(got, fmt.Sprintf(verb, secretToken)) {
			t.Errorf("fmt.Sprintf(%q) = %q leaks the token", verb, got)
		}
	}
}

// Matrix row Redaction: slog (JSON handler) prints [REDACTED] only.
func TestTokenSlogIsRedacted(t *testing.T) {
	tok := kube.Token(secretToken)

	var v slog.LogValuer = tok
	if got := v.LogValue(); got.Kind() != slog.KindString || got.String() != redacted {
		t.Errorf("LogValue() = %v (kind %v), want string %q", got, got.Kind(), redacted)
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("call", "token", tok)
	out := buf.String()
	if strings.Contains(out, secretToken) {
		t.Fatalf("slog output leaks the token: %s", out)
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("slog output is not JSON: %v: %s", err, out)
	}
	if rec["token"] != redacted {
		t.Errorf("slog token attribute = %#v, want %q", rec["token"], redacted)
	}
}

// Matrix row Redaction: json.Marshal gives "[REDACTED]".
func TestTokenJSONIsRedacted(t *testing.T) {
	tok := kube.Token(secretToken)

	b, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `"[REDACTED]"` {
		t.Errorf("json.Marshal = %s, want %q", b, `"[REDACTED]"`)
	}

	b, err = tok.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(b) != `"[REDACTED]"` {
		t.Errorf("MarshalJSON = %s, want %q", b, `"[REDACTED]"`)
	}

	wrapped := struct {
		T kube.Token `json:"t"`
	}{tok}
	b, err = json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal struct: %v", err)
	}
	if string(b) != `{"t":"[REDACTED]"}` {
		t.Errorf("json.Marshal struct = %s, want %s", b, `{"t":"[REDACTED]"}`)
	}
}
