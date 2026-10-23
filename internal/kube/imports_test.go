package kube_test

import (
	"os/exec"
	"strings"
	"testing"
)

const kubeImportPath = "github.com/mathieubodin/scaleway-manager/internal/kube"

func goList(t *testing.T, args ...string) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go tool not found: %v", err)
	}
	out, err := exec.Command(gobin, append([]string{"list"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Acceptance: the package internal/kube exists as non-test code named kube, with the
// three source files the story names.
func TestPackageHasSourceFiles(t *testing.T) {
	out := goList(t, "-f", "{{.Name}}|{{join .GoFiles \" \"}}", ".")
	name, files, _ := strings.Cut(out, "|")
	if name != "kube" {
		t.Errorf("package name = %q, want kube", name)
	}
	got := map[string]bool{}
	for _, f := range strings.Fields(files) {
		got[f] = true
	}
	for _, want := range []string{"token.go", "resource.go", "client.go"} {
		if !got[want] {
			t.Errorf("non-test Go file %s missing from internal/kube (GoFiles: %q)", want, files)
		}
	}
}

// Acceptance: `go list -deps ./internal/kube` shows only standard-library packages, and
// the package really is an HTTPS client (it depends on net/http and crypto/tls).
func TestImportsStandardLibraryOnly(t *testing.T) {
	out := goList(t, "-deps", "-f", "{{.ImportPath}}|{{.Standard}}", ".")
	sawSelf := false
	std := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		path, standard, _ := strings.Cut(line, "|")
		switch {
		case path == kubeImportPath:
			sawSelf = true
		case standard == "true":
			std[path] = true
		default:
			t.Errorf("non-standard dependency of internal/kube: %s", path)
		}
	}
	if !sawSelf {
		t.Errorf("go list output does not name %s", kubeImportPath)
	}
	for _, want := range []string{"net/http", "crypto/tls", "crypto/x509", "encoding/json", "log/slog"} {
		if !std[want] {
			t.Errorf("internal/kube does not depend on %s; the client is not implemented", want)
		}
	}
}

// Never: a dependency. The non-test imports of internal/kube itself are all standard.
func TestDirectImportsStandardLibraryOnly(t *testing.T) {
	out := goList(t, "-f", "{{join .Imports \"\\n\"}}", ".")
	if out == "" {
		t.Fatal("internal/kube has no imports; the client is not implemented")
	}
	for _, imp := range strings.Split(out, "\n") {
		first, _, _ := strings.Cut(imp, "/")
		if strings.Contains(first, ".") {
			t.Errorf("internal/kube imports non-standard package %s", imp)
		}
	}
}
