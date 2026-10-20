package make_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	checkoutPin = "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
	setupGoPin  = "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0"
)

func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jobBlock returns the lines of a top-level job of pr-checks.yml (two-space indent).
func jobBlock(t *testing.T, workflow, job string) string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	start := -1
	for i, l := range lines {
		if l == "  "+job+":" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if regexp.MustCompile(`^  \S`).MatchString(lines[i]) || regexp.MustCompile(`^\S`).MatchString(lines[i]) {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// Acceptance (CI part of make check passing on the PR): one job per Go target, named
// exactly as the target, set up like the existing jobs.
func TestPRChecksHaveGoJobs(t *testing.T) {
	wf := read(t, ".github/workflows/pr-checks.yml")
	for _, job := range goTargets {
		t.Run(job, func(t *testing.T) {
			block := jobBlock(t, wf, job)
			if block == "" {
				t.Fatalf("pr-checks.yml has no job %s", job)
			}
			for _, want := range []string{
				checkoutPin,
				"persist-credentials: false",
				setupGoPin,
				"go-version-file: go.mod",
				"cache: false",
			} {
				if !strings.Contains(block, want) {
					t.Errorf("job %s lacks %q:\n%s", job, want, block)
				}
			}
			if !regexp.MustCompile(`(?m)^\s+- run: make ` + regexp.QuoteMeta(job) + `\s*$`).MatchString(block) {
				t.Errorf("job %s does not run make %s:\n%s", job, job, block)
			}
		})
	}
}

// prek: a local lint-go hook on .go files and go.mod; no test-go hook.
func TestPrekHasLintGoHook(t *testing.T) {
	toml := read(t, "prek.toml")
	var hook string
	for _, l := range strings.Split(toml, "\n") {
		if strings.Contains(l, `id = "lint-go"`) {
			hook = l
		}
	}
	if hook == "" {
		t.Fatal(`prek.toml has no hook id = "lint-go"`)
	}
	for _, want := range []string{
		`name = "Go lint"`,
		`entry = "make lint-go"`,
		`files = "(\\.go|^go\\.mod)$"`,
		`language = "system"`,
		`pass_filenames = false`,
	} {
		if !strings.Contains(hook, want) {
			t.Errorf("lint-go hook lacks %s: %s", want, hook)
		}
	}
	if strings.Contains(toml, "make test-go") {
		t.Error("prek.toml runs make test-go; no test-go hook is wanted")
	}
}

// AD-1 on the repository: go.mod is the module line plus go 1.27.2, no go.sum or an
// empty one, no Go file in web/.
func TestRepositoryIsStandardLibraryOnly(t *testing.T) {
	var directives []string
	for _, l := range strings.Split(read(t, "go.mod"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			directives = append(directives, l)
		}
	}
	want := []string{"module github.com/mathieubodin/scaleway-manager", "go 1.27.2"}
	if strings.Join(directives, "\n") != strings.Join(want, "\n") {
		t.Errorf("go.mod = %q, want exactly %q", directives, want)
	}

	root := repoRoot(t)
	if fi, err := os.Stat(filepath.Join(root, "go.sum")); err == nil && fi.Size() > 0 {
		t.Error("go.sum is not empty")
	}

	err := filepath.WalkDir(filepath.Join(root, "web"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			t.Errorf("Go file in web/: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Errorf("walk web/: %v", err)
	}
}
