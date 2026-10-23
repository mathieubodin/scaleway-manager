package make_test

// Story 7 of #42 (CAP-9): packaging and isolation of the fake operator. The
// manifests in tests/cluster/fake-operator/ are decoded with encoding/json; no
// Kubernetes call is made.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	fakeOperatorManifestDir = "tests/cluster/fake-operator"
	fakeOperatorImage       = "scaleway-manager-fake-operator:local"
	fakeOperatorTarget      = "build-fake-operator-image"
	fakeOperatorMountPath   = "/var/run/secrets/kubernetes.io/serviceaccount"
)

// rbacTriple is one (apiGroup, resource, verb) granted by a rule.
type rbacTriple struct{ group, resource, verb string }

func (r rbacTriple) String() string {
	return fmt.Sprintf("(%q, %q, %q)", r.group, r.resource, r.verb)
}

type rbacRule struct {
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	Verbs           []string `json:"verbs"`
	ResourceNames   []string `json:"resourceNames"`
	NonResourceURLs []string `json:"nonResourceURLs"`
}

type rbacObject struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Rules []rbacRule `json:"rules"`
}

// The "RBAC rules" table of fake-operator-contract.md.
var (
	wantClusterRoleRules = []rbacRule{
		{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{"scaleway.mathieubodin.io"}, Resources: []string{"namespaceroles"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{"scaleway.mathieubodin.io"}, Resources: []string{"instances", "loadbalancers", "scalewaysecrets"}, Verbs: []string{"list", "patch"}},
		{APIGroups: []string{"scaleway.mathieubodin.io"}, Resources: []string{"instances/status", "loadbalancers/status", "scalewaysecrets/status"}, Verbs: []string{"patch"}},
	}
	wantRoleRules = []rbacRule{
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"list"}},
	}
)

// rbacTriples expands rules into the set of (apiGroup, resource, verb) triples.
func rbacTriples(rules []rbacRule) map[rbacTriple]bool {
	set := map[rbacTriple]bool{}
	for _, r := range rules {
		for _, g := range r.APIGroups {
			for _, res := range r.Resources {
				for _, v := range r.Verbs {
					set[rbacTriple{g, res, v}] = true
				}
			}
		}
	}
	return set
}

// compareRBAC is the comparison helper: it returns one line per triple missing
// from got or extra in got, plus one per rule field the table never uses. Empty
// means equal, order-insensitive.
func compareRBAC(got, want []rbacRule) []string {
	var diffs []string
	g, w := rbacTriples(got), rbacTriples(want)
	for tr := range w {
		if !g[tr] {
			diffs = append(diffs, "missing "+tr.String())
		}
	}
	for tr := range g {
		if !w[tr] {
			diffs = append(diffs, "extra "+tr.String())
		}
	}
	for i, r := range got {
		if len(r.ResourceNames) > 0 || len(r.NonResourceURLs) > 0 {
			diffs = append(diffs, fmt.Sprintf("rule %d has resourceNames or nonResourceURLs", i))
		}
		if len(r.APIGroups) == 0 || len(r.Resources) == 0 || len(r.Verbs) == 0 {
			diffs = append(diffs, fmt.Sprintf("rule %d has an empty apiGroups, resources or verbs", i))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// forbiddenRBAC returns one line per rule naming secrets or *.
func forbiddenRBAC(rules []rbacRule) []string {
	var bad []string
	for i, r := range rules {
		for _, field := range [][]string{r.APIGroups, r.Resources, r.Verbs, r.ResourceNames, r.NonResourceURLs} {
			for _, v := range field {
				if v == "*" || strings.Contains(v, "*") || v == "secrets" || strings.HasPrefix(v, "secrets/") {
					bad = append(bad, fmt.Sprintf("rule %d names %q", i, v))
				}
			}
		}
	}
	return bad
}

// decodeOne decodes exactly one JSON object from b into v; trailing data is an error.
func decodeOne(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("more than one JSON value in the file")
	}
	return nil
}

func readRBACFile(t *testing.T, path string) rbacObject {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var o rbacObject
	if err := decodeOne(b, &o); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return o
}

// manifests decodes every JSON file of tests/cluster/fake-operator/, keyed by base name.
func manifests(t *testing.T) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(repoRoot(t), fakeOperatorManifestDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", fakeOperatorManifestDir, err)
	}
	out := map[string]map[string]any{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := decodeOne(b, &m); err != nil {
			t.Errorf("%s/%s: not one JSON object: %v", fakeOperatorManifestDir, e.Name(), err)
			continue
		}
		out[e.Name()] = m
	}
	return out
}

// path walks a decoded JSON value by object keys and array indexes.
func jsonPath(v any, keys ...any) (any, bool) {
	for _, k := range keys {
		switch key := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil, false
			}
			if v, ok = m[key]; !ok {
				return nil, false
			}
		case int:
			a, ok := v.([]any)
			if !ok || key >= len(a) {
				return nil, false
			}
			v = a[key]
		}
	}
	return v, true
}

func str(v any, keys ...any) string {
	x, _ := jsonPath(v, keys...)
	s, _ := x.(string)
	return s
}

// scanFakeOperatorLeaks is the scan helper: it returns, relative to root, every
// release artifact (Dockerfile, any file under charts/, .github/workflows/deliver.yml)
// that contains the string fake-operator.
func scanFakeOperatorLeaks(root string) ([]string, error) {
	var leaks []string
	check := func(rel string) error {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte("fake-operator")) {
			leaks = append(leaks, filepath.ToSlash(rel))
		}
		return nil
	}
	for _, rel := range []string{"Dockerfile", filepath.Join(".github", "workflows", "deliver.yml")} {
		if err := check(rel); err != nil {
			return nil, err
		}
	}
	charts := filepath.Join(root, "charts")
	err := filepath.WalkDir(charts, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		return check(rel)
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(leaks)
	return leaks, nil
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		copyFile(t, p, filepath.Join(dst, rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Check 1: every file is one JSON object, the seven manifests exist with their kinds.
func TestFakeOperatorManifestsAreJSONObjects(t *testing.T) {
	dir := filepath.Join(repoRoot(t), fakeOperatorManifestDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", fakeOperatorManifestDir, err)
	}
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			t.Errorf("YAML file in %s: %s", fakeOperatorManifestDir, e.Name())
		}
	}
	ms := manifests(t)
	wantKinds := map[string]string{
		"namespace.json":          "Namespace",
		"serviceaccount.json":     "ServiceAccount",
		"clusterrole.json":        "ClusterRole",
		"clusterrolebinding.json": "ClusterRoleBinding",
		"role.json":               "Role",
		"rolebinding.json":        "RoleBinding",
		"deployment.json":         "Deployment",
	}
	for file, kind := range wantKinds {
		m, ok := ms[file]
		if !ok {
			t.Errorf("%s/%s missing", fakeOperatorManifestDir, file)
			continue
		}
		if got := str(m, "kind"); got != kind {
			t.Errorf("%s: kind %q, want %q", file, got, kind)
		}
	}
	if ns := ms["namespace.json"]; ns != nil {
		if got := str(ns, "metadata", "name"); got != "fake-operator" {
			t.Errorf("namespace.json: name %q, want fake-operator", got)
		}
	}
}

// Check 1: ClusterRole and Role rules equal the contract table, as sets of triples;
// the Role is in scaleway-system.
func TestFakeOperatorRBACMatchesContract(t *testing.T) {
	dir := filepath.Join(repoRoot(t), fakeOperatorManifestDir)
	cr := readRBACFile(t, filepath.Join(dir, "clusterrole.json"))
	if cr.Kind != "ClusterRole" {
		t.Errorf("clusterrole.json: kind %q", cr.Kind)
	}
	for _, d := range compareRBAC(cr.Rules, wantClusterRoleRules) {
		t.Errorf("ClusterRole: %s", d)
	}
	role := readRBACFile(t, filepath.Join(dir, "role.json"))
	if role.Kind != "Role" {
		t.Errorf("role.json: kind %q", role.Kind)
	}
	if role.Metadata.Namespace != "scaleway-system" {
		t.Errorf("Role namespace %q, want scaleway-system", role.Metadata.Namespace)
	}
	for _, d := range compareRBAC(role.Rules, wantRoleRules) {
		t.Errorf("Role: %s", d)
	}
}

// The bindings tie the ClusterRole and the Role to the ServiceAccount the
// Deployment runs as; the RoleBinding is in scaleway-system.
func TestFakeOperatorBindingsReferenceServiceAccount(t *testing.T) {
	ms := manifests(t)
	sa, cr, role, dep := ms["serviceaccount.json"], ms["clusterrole.json"], ms["role.json"], ms["deployment.json"]
	if sa == nil || cr == nil || role == nil || dep == nil {
		t.Fatal("serviceaccount.json, clusterrole.json, role.json or deployment.json missing")
	}
	saName, saNS := str(sa, "metadata", "name"), str(sa, "metadata", "namespace")
	if saName == "" {
		t.Fatal("serviceaccount.json has no metadata.name")
	}
	if got := str(dep, "spec", "template", "spec", "serviceAccountName"); got != saName {
		t.Errorf("Deployment serviceAccountName %q, want %q", got, saName)
	}
	for file, roleKind := range map[string]string{"clusterrolebinding.json": "ClusterRole", "rolebinding.json": "Role"} {
		b := ms[file]
		if b == nil {
			t.Errorf("%s missing", file)
			continue
		}
		target := cr
		if roleKind == "Role" {
			target = role
			if got := str(b, "metadata", "namespace"); got != "scaleway-system" {
				t.Errorf("%s: namespace %q, want scaleway-system", file, got)
			}
		}
		if got := str(b, "roleRef", "kind"); got != roleKind {
			t.Errorf("%s: roleRef.kind %q, want %q", file, got, roleKind)
		}
		if got, want := str(b, "roleRef", "name"), str(target, "metadata", "name"); got != want {
			t.Errorf("%s: roleRef.name %q, want %q", file, got, want)
		}
		subjects, _ := jsonPath(b, "subjects")
		list, _ := subjects.([]any)
		found := false
		for _, s := range list {
			if str(s, "kind") == "ServiceAccount" && str(s, "name") == saName && str(s, "namespace") == saNS {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no subject ServiceAccount %s/%s: %v", file, saNS, saName, subjects)
		}
	}
}

// "Extra verb" row: a scratch copy of the ClusterRole with watch added makes the
// comparison helper report the difference.
func TestFakeOperatorCompareRBACReportsExtraVerb(t *testing.T) {
	src := filepath.Join(repoRoot(t), fakeOperatorManifestDir, "clusterrole.json")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := decodeOne(b, &raw); err != nil {
		t.Fatal(err)
	}
	rules, _ := raw["rules"].([]any)
	if len(rules) == 0 {
		t.Fatal("clusterrole.json has no rules")
	}
	first := rules[0].(map[string]any)
	first["verbs"] = append(first["verbs"].([]any), "watch")
	scratch := filepath.Join(t.TempDir(), "clusterrole.json")
	out, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scratch, out, 0o644); err != nil {
		t.Fatal(err)
	}
	cr := readRBACFile(t, scratch)
	diffs := compareRBAC(cr.Rules, wantClusterRoleRules)
	if len(diffs) == 0 {
		t.Fatal("compareRBAC reports no difference on a copy with watch added")
	}
	if !strings.Contains(strings.Join(diffs, "\n"), `"watch"`) {
		t.Errorf("compareRBAC does not name watch: %v", diffs)
	}
	if d := compareRBAC(wantClusterRoleRules, wantClusterRoleRules); len(d) != 0 {
		t.Errorf("compareRBAC on equal rules: %v", d)
	}
}

// Check 2: no rule names secrets or *.
func TestFakeOperatorRBACHasNoSecretsNorWildcard(t *testing.T) {
	dir := filepath.Join(repoRoot(t), fakeOperatorManifestDir)
	for _, file := range []string{"clusterrole.json", "role.json"} {
		o := readRBACFile(t, filepath.Join(dir, file))
		if len(o.Rules) == 0 {
			t.Errorf("%s has no rules", file)
		}
		for _, bad := range forbiddenRBAC(o.Rules) {
			t.Errorf("%s: %s", file, bad)
		}
	}
	if len(forbiddenRBAC([]rbacRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}})) == 0 {
		t.Error("forbiddenRBAC misses secrets")
	}
	if len(forbiddenRBAC([]rbacRule{{APIGroups: []string{"*"}, Resources: []string{"pods"}, Verbs: []string{"get"}}})) == 0 {
		t.Error("forbiddenRBAC misses *")
	}
}

// Check 3: the Deployment as the story specifies it.
func TestFakeOperatorDeployment(t *testing.T) {
	dep := manifests(t)["deployment.json"]
	if dep == nil {
		t.Fatal("deployment.json missing")
	}
	if r, _ := jsonPath(dep, "spec", "replicas"); r != float64(1) {
		t.Errorf("replicas %v, want 1", r)
	}
	pod, _ := jsonPath(dep, "spec", "template", "spec")
	if a, ok := jsonPath(pod, "automountServiceAccountToken"); !ok || a != false {
		t.Errorf("automountServiceAccountToken %v, want false", a)
	}
	cs, _ := jsonPath(pod, "containers")
	containers, _ := cs.([]any)
	if len(containers) != 1 {
		t.Fatalf("want one container, got %d", len(containers))
	}
	c := containers[0]
	if got := str(c, "image"); got != fakeOperatorImage {
		t.Errorf("image %q, want %s", got, fakeOperatorImage)
	}
	if got := str(c, "imagePullPolicy"); got != "Never" {
		t.Errorf("imagePullPolicy %q, want Never", got)
	}

	// Projected volume: serviceAccountToken path token + ConfigMap kube-root-ca.crt key ca.crt.
	vs, _ := jsonPath(pod, "volumes")
	volumes, _ := vs.([]any)
	projected := ""
	for _, v := range volumes {
		srcs, ok := jsonPath(v, "projected", "sources")
		if !ok {
			continue
		}
		hasToken, hasCA := false, false
		list, _ := srcs.([]any)
		for _, s := range list {
			if str(s, "serviceAccountToken", "path") == "token" {
				hasToken = true
			}
			if str(s, "configMap", "name") == "kube-root-ca.crt" {
				items, _ := jsonPath(s, "configMap", "items")
				il, _ := items.([]any)
				for _, it := range il {
					if str(it, "key") == "ca.crt" {
						hasCA = true
					}
				}
			}
		}
		if hasToken && hasCA {
			projected = str(v, "name")
		}
	}
	if projected == "" {
		t.Errorf("no projected volume with serviceAccountToken path token and ConfigMap kube-root-ca.crt key ca.crt: %v", vs)
	} else {
		ms, _ := jsonPath(c, "volumeMounts")
		mounts, _ := ms.([]any)
		ok := false
		for _, m := range mounts {
			ro, _ := jsonPath(m, "readOnly")
			if str(m, "name") == projected && str(m, "mountPath") == fakeOperatorMountPath && ro == true {
				ok = true
			}
		}
		if !ok {
			t.Errorf("volume %s not mounted read-only at %s: %v", projected, fakeOperatorMountPath, ms)
		}
	}

	es, _ := jsonPath(c, "env")
	env, _ := es.([]any)
	delay := false
	for _, e := range env {
		if str(e, "name") == "FAKE_OPERATOR_STEP_DELAY" && str(e, "value") == "2s" {
			delay = true
		}
	}
	if !delay {
		t.Errorf("env FAKE_OPERATOR_STEP_DELAY=2s missing: %v", es)
	}

	ps, _ := jsonPath(c, "ports")
	ports, _ := ps.([]any)
	portName := ""
	has8081 := false
	for _, p := range ports {
		if n, _ := jsonPath(p, "containerPort"); n == float64(8081) {
			has8081 = true
			portName = str(p, "name")
		}
	}
	if !has8081 {
		t.Errorf("container port 8081 missing: %v", ps)
	}

	probe, ok := jsonPath(c, "livenessProbe")
	if !ok {
		t.Fatal("no livenessProbe")
	}
	if got := str(probe, "httpGet", "path"); got != "/healthz" {
		t.Errorf("livenessProbe httpGet.path %q, want /healthz", got)
	}
	port, _ := jsonPath(probe, "httpGet", "port")
	if !(port == float64(8081) || (portName != "" && port == portName)) {
		t.Errorf("livenessProbe port %v, want 8081", port)
	}
	if v, _ := jsonPath(probe, "periodSeconds"); v != float64(10) {
		t.Errorf("livenessProbe periodSeconds %v, want 10", v)
	}
	if v, _ := jsonPath(probe, "failureThreshold"); v != float64(3) {
		t.Errorf("livenessProbe failureThreshold %v, want 3", v)
	}

	sc, _ := jsonPath(c, "securityContext")
	nonRoot, _ := jsonPath(sc, "runAsNonRoot")
	if nonRoot != true {
		nonRoot, _ = jsonPath(pod, "securityContext", "runAsNonRoot")
	}
	if nonRoot != true {
		t.Error("runAsNonRoot is not true")
	}
	if v, _ := jsonPath(sc, "readOnlyRootFilesystem"); v != true {
		t.Error("readOnlyRootFilesystem is not true")
	}
	if v, _ := jsonPath(sc, "allowPrivilegeEscalation"); v != false {
		t.Error("allowPrivilegeEscalation is not false")
	}
	drop, _ := jsonPath(sc, "capabilities", "drop")
	if dl, _ := drop.([]any); len(dl) != 1 || dl[0] != "ALL" {
		t.Errorf("capabilities.drop %v, want [ALL]", drop)
	}
}

// Check 4 and the Dockerfile rules: same FROM lines as the root Dockerfile, the
// build line, USER 65532, ENTRYPOINT; scripts/check-dockerfile.sh exits 0.
func TestFakeOperatorDockerfile(t *testing.T) {
	root := repoRoot(t)
	df := read(t, "cmd/fake-operator/Dockerfile")
	fromRe := regexp.MustCompile(`(?mi)^\s*FROM\s.*$`)
	got, want := fromRe.FindAllString(df, -1), fromRe.FindAllString(read(t, "Dockerfile"), -1)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("FROM lines %q, want those of the root Dockerfile %q", got, want)
	}
	for _, line := range []string{
		"CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/fake-operator ./cmd/fake-operator",
		"USER 65532",
		`ENTRYPOINT ["/fake-operator"]`,
	} {
		if !strings.Contains(df, line) {
			t.Errorf("cmd/fake-operator/Dockerfile lacks %q", line)
		}
	}

	cmd := exec.Command(lookPath(t, "bash"), "scripts/check-dockerfile.sh", "cmd/fake-operator/Dockerfile")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile: %v\n%s", err, out)
	}
}

// Check 5: no release artifact mentions fake-operator.
func TestFakeOperatorAbsentFromReleaseArtifacts(t *testing.T) {
	leaks, err := scanFakeOperatorLeaks(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range leaks {
		t.Errorf("release artifact mentions fake-operator: %s", l)
	}
}

// "Leak" row: a scratch file under a temporary charts/ copy is reported.
func TestFakeOperatorScanReportsLeak(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	copyFile(t, filepath.Join(root, "Dockerfile"), filepath.Join(dir, "Dockerfile"))
	copyFile(t, filepath.Join(root, ".github", "workflows", "deliver.yml"), filepath.Join(dir, ".github", "workflows", "deliver.yml"))
	copyTree(t, filepath.Join(root, "charts"), filepath.Join(dir, "charts"))

	if leaks, err := scanFakeOperatorLeaks(dir); err != nil || len(leaks) != 0 {
		t.Fatalf("clean copy: leaks %v, err %v", leaks, err)
	}
	scratch := filepath.Join(dir, "charts", "scaleway-manager", "templates", "scratch.yaml")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scratch, []byte("image: scaleway-manager-fake-operator:local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	leaks, err := scanFakeOperatorLeaks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(leaks) != 1 || leaks[0] != "charts/scaleway-manager/templates/scratch.yaml" {
		t.Errorf("leaks %v, want [charts/scaleway-manager/templates/scratch.yaml]", leaks)
	}
}

// Acceptance: make help lists build-fake-operator-image.
func TestFakeOperatorMakeHelpListsTarget(t *testing.T) {
	out, code := runMake(t, repoRoot(t), nil, "-s", "help")
	if code != 0 {
		t.Fatalf("make help exit %d:\n%s", code, out)
	}
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(fakeOperatorTarget) + `\s+\S`).MatchString(out) {
		t.Errorf("make help does not list %s:\n%s", fakeOperatorTarget, out)
	}
}

// Makefile: the target line, .PHONY, out of check; out of scripts/evidence.sh.
func TestFakeOperatorMakeTarget(t *testing.T) {
	mk := read(t, "Makefile")
	line := fakeOperatorTarget + ": check-docker ## Check the fake operator Dockerfile and build " + fakeOperatorImage + " (CAP-9)"
	if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(line) + `$`).MatchString(mk) {
		t.Errorf("Makefile lacks the line %q", line)
	}
	phony := regexp.MustCompile(`(?m)^\.PHONY:(.*)$`).FindStringSubmatch(mk)
	if phony == nil || !strings.Contains(" "+phony[1]+" ", " "+fakeOperatorTarget+" ") {
		t.Errorf("%s is not in .PHONY", fakeOperatorTarget)
	}
	check := regexp.MustCompile(`(?m)^check:(.*)$`).FindStringSubmatch(mk)
	if check == nil {
		t.Fatal("no check target")
	}
	if strings.Contains(check[1], fakeOperatorTarget) {
		t.Errorf("check runs %s: %s", fakeOperatorTarget, check[0])
	}
	if strings.Contains(read(t, "scripts/evidence.sh"), "fake-operator") {
		t.Error("scripts/evidence.sh mentions fake-operator")
	}
}

// Build script: executable, checks the Dockerfile, builds the local image, never pushes.
func TestFakeOperatorBuildScript(t *testing.T) {
	root := repoRoot(t)
	fi, err := os.Stat(filepath.Join(root, "scripts", "build-fake-operator-image.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("scripts/build-fake-operator-image.sh is not executable")
	}
	s := read(t, "scripts/build-fake-operator-image.sh")
	for _, want := range []string{
		"scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile",
		"DOCKER_BUILDKIT=1 docker build -f cmd/fake-operator/Dockerfile -t " + fakeOperatorImage + " .",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("scripts/build-fake-operator-image.sh lacks %q", want)
		}
	}
	if regexp.MustCompile(`docker\s+(push|login|run)`).MatchString(s) {
		t.Error("scripts/build-fake-operator-image.sh pushes, logs in or runs a container")
	}
	if !strings.Contains(read(t, "Makefile"), "scripts/build-fake-operator-image.sh") {
		t.Error("the Makefile does not run scripts/build-fake-operator-image.sh")
	}
}

// CI: the build-image job runs make build-fake-operator-image after make build-image.
func TestFakeOperatorCIStep(t *testing.T) {
	block := jobBlock(t, read(t, ".github/workflows/pr-checks.yml"), "build-image")
	if block == "" {
		t.Fatal("pr-checks.yml has no job build-image")
	}
	bi := regexp.MustCompile(`(?m)^\s+- run: make build-image\s*$`).FindStringIndex(block)
	fo := regexp.MustCompile(`(?m)^\s+- run: make ` + fakeOperatorTarget + `\s*$`).FindStringIndex(block)
	if bi == nil || fo == nil {
		t.Fatalf("job build-image lacks make build-image or make %s:\n%s", fakeOperatorTarget, block)
	}
	if fo[0] < bi[0] {
		t.Errorf("make %s runs before make build-image", fakeOperatorTarget)
	}
	if strings.Contains(read(t, ".github/workflows/pr-checks.yml"), "  "+fakeOperatorTarget+":") {
		t.Errorf("pr-checks.yml has a job %s; want a step only", fakeOperatorTarget)
	}
}
