package kube

import "errors"

// errNotImplemented is returned by the contract stubs until story 1 is built.
var errNotImplemented = errors.New("kube: not implemented")

// Resource is one entry of the AD-2 closed list.
type Resource int

const (
	Instances Resource = iota + 1
	LoadBalancers
	ScalewaySecrets
	NamespaceRoles
	Namespaces
	Secrets
	CRDs
	Deployments
)

func (r Resource) String() string { return "" }

func (r Resource) Namespaced() bool { return false }

func (r Resource) Kind() string { return "" }

// Resources returns the closed list, in the order of the constants.
func Resources() []Resource { return nil }

// Path builds the request path of every call.
func Path(r Resource, namespace, name string, status bool) (string, error) {
	return "", errNotImplemented
}

// ParsePath is the inverse of Path.
func ParsePath(p string) (r Resource, namespace, name string, status bool, err error) {
	return 0, "", "", false, errNotImplemented
}
