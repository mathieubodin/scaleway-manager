package kube

import "strings"

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

const (
	coreGroup     = "/api/v1"
	scalewayGroup = "/apis/scaleway.mathieubodin.io/v1"
	crdGroup      = "/apis/apiextensions.k8s.io/v1"
	appsGroup     = "/apis/apps/v1"
)

type operation int

const (
	opList operation = 1 << iota
	opGet
	opMergePatch
	opPatchStatus
)

type resourceInfo struct {
	prefix     string
	plural     string
	kind       string
	namespaced bool
	ops        operation
}

var resourceTable = map[Resource]resourceInfo{
	Instances:       {scalewayGroup, "instances", "Instance", true, opList | opGet | opMergePatch | opPatchStatus},
	LoadBalancers:   {scalewayGroup, "loadbalancers", "LoadBalancer", true, opList | opGet | opMergePatch | opPatchStatus},
	ScalewaySecrets: {scalewayGroup, "scalewaysecrets", "ScalewaySecret", true, opList | opGet | opMergePatch | opPatchStatus},
	NamespaceRoles:  {scalewayGroup, "namespaceroles", "NamespaceRole", false, opList | opGet},
	Namespaces:      {coreGroup, "namespaces", "Namespace", false, opList | opGet},
	Secrets:         {coreGroup, "secrets", "Secret", true, 0},
	CRDs:            {crdGroup, "customresourcedefinitions", "CustomResourceDefinition", false, opGet},
	Deployments:     {appsGroup, "deployments", "Deployment", true, opList},
}

func (r Resource) info() (resourceInfo, bool) {
	i, ok := resourceTable[r]
	return i, ok
}

// String returns the plural, e.g. "instances".
func (r Resource) String() string { return r.infoOrZero().plural }

// Namespaced reports whether the kind lives in a namespace.
func (r Resource) Namespaced() bool { return r.infoOrZero().namespaced }

// Kind returns the kind, e.g. "Instance"; the list kind is Kind()+"List".
func (r Resource) Kind() string { return r.infoOrZero().kind }

func (r Resource) infoOrZero() resourceInfo {
	i, _ := r.info()
	return i
}

// Resources returns the closed list, in the order of the constants.
func Resources() []Resource {
	return []Resource{Instances, LoadBalancers, ScalewaySecrets, NamespaceRoles, Namespaces, Secrets, CRDs, Deployments}
}

// hasStatus reports whether the kind has a /status subresource.
func (r Resource) hasStatus() bool {
	return r == Instances || r == LoadBalancers || r == ScalewaySecrets
}

func isAlnum(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') }

// validLabel applies the DNS-1123 label rule.
func validLabel(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isAlnum(c) && c != '-' {
			return false
		}
	}
	return isAlnum(s[0]) && isAlnum(s[len(s)-1])
}

// validSubdomain applies the DNS-1123 subdomain rule.
func validSubdomain(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isAlnum(c) && c != '-' && c != '.' {
			return false
		}
	}
	return isAlnum(s[0]) && isAlnum(s[len(s)-1])
}

// Path builds the request path of every call.
func Path(r Resource, namespace, name string, status bool) (string, error) {
	info, ok := r.info()
	if !ok {
		return "", ErrUnknownResource
	}
	if status && !r.hasStatus() {
		return "", ErrOperationNotAllowed
	}
	if info.namespaced {
		if namespace == "" && name != "" {
			return "", ErrInvalidName
		}
		if namespace != "" && !validLabel(namespace) {
			return "", ErrInvalidName
		}
	} else if namespace != "" {
		return "", ErrInvalidName
	}
	if name != "" {
		valid := validSubdomain(name)
		if r == Namespaces {
			valid = validLabel(name)
		}
		if !valid {
			return "", ErrInvalidName
		}
	}
	if status && name == "" {
		return "", ErrInvalidName
	}

	var b strings.Builder
	b.WriteString(info.prefix)
	if namespace != "" {
		b.WriteString("/namespaces/")
		b.WriteString(namespace)
	}
	b.WriteString("/")
	b.WriteString(info.plural)
	if name != "" {
		b.WriteString("/")
		b.WriteString(name)
	}
	if status {
		b.WriteString("/status")
	}
	return b.String(), nil
}

// ParsePath is the inverse of Path: it accepts exactly the paths Path builds.
func ParsePath(p string) (r Resource, namespace, name string, status bool, err error) {
	for _, res := range Resources() {
		info := res.infoOrZero()
		rest, ok := strings.CutPrefix(p, info.prefix+"/")
		if !ok {
			continue
		}
		segs := strings.Split(rest, "/")
		var ns, n string
		var st bool
		switch {
		case len(segs) >= 3 && segs[0] == "namespaces" && segs[2] == info.plural:
			ns = segs[1]
			switch len(segs) {
			case 3:
			case 4:
				n = segs[3]
			case 5:
				if segs[4] != "status" {
					continue
				}
				n, st = segs[3], true
			default:
				continue
			}
		case segs[0] == info.plural && len(segs) <= 2:
			if len(segs) == 2 {
				n = segs[1]
			}
		default:
			continue
		}
		built, perr := Path(res, ns, n, st)
		if perr != nil || built != p {
			continue
		}
		return res, ns, n, st, nil
	}
	return 0, "", "", false, ErrUnknownResource
}
