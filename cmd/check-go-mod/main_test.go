package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Matrix row AD-1: a require line, a require block, a non-empty go.sum fail naming
// the file; a clean module passes.
func TestCheck(t *testing.T) {
	cases := []struct {
		fixture string
		want    string // "" means check passes
		notWant string
	}{
		{"require-line", "go.mod: require found", "go.sum"},
		{"require-block", "go.mod: require found", "go.sum"},
		{"gosum", "go.sum: not empty", "go.mod"},
		{"clean", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			err := check(filepath.Join("testdata", tc.fixture))
			if tc.want == "" {
				if err != nil {
					t.Errorf("check(%s) = %v, want nil", tc.fixture, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("check(%s) = nil, want an error naming %q", tc.fixture, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("check(%s) = %q, want it to contain %q", tc.fixture, err, tc.want)
			}
			if strings.Contains(err.Error(), tc.notWant) {
				t.Errorf("check(%s) = %q, names %q too", tc.fixture, err, tc.notWant)
			}
		})
	}
}

// An empty go.sum is allowed: only size > 0 fails.
func TestCheckEmptyGoSum(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/fixture\n\ngo 1.27.2\n")
	write(t, filepath.Join(dir, "go.sum"), "")
	if err := check(dir); err != nil {
		t.Errorf("check with an empty go.sum = %v, want nil", err)
	}
}

// A missing or unreadable go.mod is an error, never a pass.
func TestCheckMissingGoMod(t *testing.T) {
	if err := check(t.TempDir()); err == nil {
		t.Error("check of a directory without go.mod = nil, want an error")
	}
}

// main checks ".", prints the error and exits 1; passes with exit 0 on a clean module.
func TestMainExitCode(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "check-go-mod")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cases := []struct {
		fixture  string
		wantCode int
		wantOut  string
	}{
		{"require-line", 1, "go.mod"},
		{"require-block", 1, "go.mod"},
		{"gosum", 1, "go.sum"},
		{"clean", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("run: %v", err)
			}
			if code != tc.wantCode {
				t.Errorf("exit code %d, want %d; output:\n%s", code, tc.wantCode, out)
			}
			if tc.wantOut != "" && !strings.Contains(string(out), tc.wantOut) {
				t.Errorf("output %q does not name %q", out, tc.wantOut)
			}
		})
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
