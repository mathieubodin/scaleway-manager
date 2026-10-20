// Package make_test runs the Go make targets of story 1 of #41 for real, in
// throwaway copies of the repository where a run needs a broken input.
package make_test

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const goHint = "Error: go not found. Install Go 1.27.2: https://go.dev/dl/"

var goTargets = []string{"lint-go", "test-go", "check-go-mod"}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Makefile")); err != nil {
		t.Fatalf("no Makefile at %s: %v", root, err)
	}
	return root
}

func lookPath(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not found: %v", name, err)
	}
	return p
}

// runMake runs make in dir with the given extra environment; returns output and exit code.
func runMake(t *testing.T, dir string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(lookPath(t, "make"), append([]string{"--no-print-directory"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &ee):
		return string(out), ee.ExitCode()
	default:
		t.Fatalf("make %v: %v", args, err)
		return "", -1
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(lookPath(t, "git"), args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// Acceptance: make help lists lint-go, test-go, check-go-mod.
func TestMakeHelpListsGoTargets(t *testing.T) {
	out, code := runMake(t, repoRoot(t), nil, "-s", "help")
	if code != 0 {
		t.Fatalf("make help exit %d:\n%s", code, out)
	}
	for _, target := range goTargets {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(target) + `\s`).MatchString(out) {
			t.Errorf("make help does not list %s:\n%s", target, out)
		}
	}
	if regexp.MustCompile(`(?m)^\s+check-go\s`).MatchString(out) {
		t.Errorf("make help lists the guard check-go, which has no ## comment:\n%s", out)
	}
}

// Acceptance (make check): the three targets and the guard are declared, public ones
// with a ## comment, all in .PHONY, and check runs the three.
func TestMakefileDeclaresGoTargets(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	mk := string(b)

	phony := map[string]bool{}
	var checkPrereqs []string
	sc := bufio.NewScanner(strings.NewReader(mk))
	for sc.Scan() {
		line := sc.Text()
		if rest, ok := strings.CutPrefix(line, ".PHONY:"); ok {
			for _, f := range strings.Fields(rest) {
				phony[f] = true
			}
		}
		if rest, ok := strings.CutPrefix(line, "check:"); ok {
			rest, _, _ = strings.Cut(rest, "##")
			checkPrereqs = strings.Fields(rest)
		}
	}
	for _, target := range append([]string{"check-go"}, goTargets...) {
		if !phony[target] {
			t.Errorf("%s is not in .PHONY", target)
		}
	}
	for _, target := range goTargets {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `:[^\n#]*\bcheck-go\b[^\n]*## \S`)
		if !re.MatchString(mk) {
			t.Errorf("target %s lacks the check-go prerequisite or its ## comment", target)
		}
		found := false
		for _, p := range checkPrereqs {
			if p == target {
				found = true
			}
		}
		if !found {
			t.Errorf("check does not run %s (prerequisites %v)", target, checkPrereqs)
		}
	}
	if regexp.MustCompile(`(?m)^check-go:[^\n]*##`).MatchString(mk) {
		t.Error("the guard check-go has a ## comment")
	}
	if !strings.Contains(mk, goHint) {
		t.Errorf("Makefile lacks the install hint %q", goHint)
	}
}

// Acceptance: with go absent from PATH, each of the three targets prints the install
// hint and fails. The recipe exits 1; make itself then reports a failure.
func TestGoAbsentPrintsInstallHint(t *testing.T) {
	root := repoRoot(t)
	bin := t.TempDir()
	// Every tool a recipe may reasonably need, except go and gofmt.
	for _, tool := range []string{"bash", "sh", "env", "cat", "echo", "printf", "test", "grep", "awk", "sed", "git", "which", "command", "true", "false"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		if err := os.Symlink(p, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range goTargets {
		t.Run(target, func(t *testing.T) {
			out, code := runMake(t, root, []string{"PATH=" + bin}, target)
			if code == 0 {
				t.Errorf("make %s exit 0 without go, want a failure:\n%s", target, out)
			}
			if !strings.Contains(out, goHint) {
				t.Errorf("make %s output lacks %q:\n%s", target, goHint, out)
			}
		})
	}
}

// Acceptance (make check, check-go-mod part): the targets pass on the repository
// itself. test-go is left out: this test runs under it.
func TestGoTargetsPassOnRepository(t *testing.T) {
	lookPath(t, "go")
	out, code := runMake(t, repoRoot(t), nil, "lint-go", "check-go-mod")
	if code != 0 {
		t.Errorf("make lint-go check-go-mod exit %d, want 0:\n%s", code, out)
	}
}

// Acceptance: a require added to go.mod makes make check-go-mod fail naming go.mod.
// Run in a copy holding the Makefile, go.mod and the check program.
func TestCheckGoModFailsOnScratchRequire(t *testing.T) {
	lookPath(t, "go")
	root := repoRoot(t)
	dir := t.TempDir()
	copyFile(t, filepath.Join(root, "Makefile"), filepath.Join(dir, "Makefile"))
	copyFile(t, filepath.Join(root, "go.mod"), filepath.Join(dir, "go.mod"))
	srcs, err := filepath.Glob(filepath.Join(root, "cmd", "check-go-mod", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range srcs {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		copyFile(t, src, filepath.Join(dir, "cmd", "check-go-mod", filepath.Base(src)))
	}

	if out, code := runMake(t, dir, nil, "check-go-mod"); code != 0 {
		t.Fatalf("make check-go-mod on the unchanged copy exit %d, want 0:\n%s", code, out)
	}

	f, err := os.OpenFile(filepath.Join(dir, "go.mod"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nrequire example.com/scratch v1.0.0\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out, code := runMake(t, dir, nil, "check-go-mod")
	if code == 0 {
		t.Errorf("make check-go-mod with a require exit 0, want a failure:\n%s", out)
	}
	if !strings.Contains(out, "go.mod: require found") {
		t.Errorf("make check-go-mod output does not name go.mod (\"go.mod: require found\"):\n%s", out)
	}
}

// Acceptance: an unformatted tracked .go file makes make lint-go fail listing it.
// Run in a throwaway git repository holding the Makefile and go.mod.
func TestLintGoFailsOnUnformattedFile(t *testing.T) {
	lookPath(t, "go")
	root := repoRoot(t)
	dir := t.TempDir()
	copyFile(t, filepath.Join(root, "Makefile"), filepath.Join(dir, "Makefile"))
	copyFile(t, filepath.Join(root, "go.mod"), filepath.Join(dir, "go.mod"))
	src := filepath.Join(dir, "pkg", "scratch.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package pkg\n\nfunc F() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	git(t, dir, "add", ".")

	if out, code := runMake(t, dir, nil, "lint-go"); code != 0 {
		t.Fatalf("make lint-go on formatted code exit %d, want 0:\n%s", code, out)
	}

	if err := os.WriteFile(src, []byte("package pkg\n\nfunc   F( ) int {\nreturn 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runMake(t, dir, nil, "lint-go")
	if code == 0 {
		t.Errorf("make lint-go with an unformatted file exit 0, want a failure:\n%s", out)
	}
	if !strings.Contains(out, "pkg/scratch.go") {
		t.Errorf("make lint-go output does not list pkg/scratch.go:\n%s", out)
	}
}
