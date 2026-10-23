// Package kubetest is the fake Kubernetes API server of the Go tests (AD-15).
//
// Contract stub (#42): the bodies are written by story 2.
package kubetest

import (
	"encoding/json"
	"testing"

	"github.com/mathieubodin/scaleway-manager/internal/kube"
)

// Server is a TLS test server holding objects in memory.
type Server struct {
	Host  string
	Port  string
	CAPEM []byte
}

// Request is one recorded request.
type Request struct {
	Method      string
	RequestURI  string
	ContentType string
	Token       string
	Body        []byte
}

func New(t testing.TB) *Server { return &Server{} }

func (s *Server) Put(r kube.Resource, obj json.RawMessage) {}

func (s *Server) Object(r kube.Resource, namespace, name string) (json.RawMessage, bool) {
	return nil, false
}

func (s *Server) Delete(r kube.Resource, namespace, name string) {}

func (s *Server) ExpectToken(tok string) {}

func (s *Server) Tokens() []string { return nil }

func (s *Server) Requests() []Request { return nil }

func (s *Server) Fail(method, pathPrefix string, code int) {}

func (s *Server) ClearFaults() {}
