// Command check-go-mod enforces AD-1 (standard library only): go.mod has no
// require and go.sum, when present, is empty.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if err := check("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// check fails when <dir>/go.mod has a require or <dir>/go.sum is not empty.
func check(dir string) error {
	gomod := filepath.Join(dir, "go.mod")
	out, err := exec.Command("go", "mod", "edit", "-json", gomod).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("go.mod: cannot read %s: %v: %s", gomod, err, ee.Stderr)
		}
		return fmt.Errorf("go.mod: cannot read %s: %v", gomod, err)
	}
	var mod struct {
		Require []json.RawMessage
	}
	if err := json.Unmarshal(out, &mod); err != nil {
		return fmt.Errorf("go.mod: cannot parse go mod edit output: %v", err)
	}
	if len(mod.Require) > 0 {
		return errors.New("go.mod: require found")
	}

	fi, err := os.Stat(filepath.Join(dir, "go.sum"))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("go.sum: %v", err)
	case fi.Size() > 0:
		return errors.New("go.sum: not empty")
	}
	return nil
}
