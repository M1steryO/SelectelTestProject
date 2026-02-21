package loglint_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_ModuleMode(t *testing.T) {
	// Build the loglint binary from the current module.
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "loglint")

	build := exec.Command("go", "build", "-o", tool, "./cmd/loglint")
	// build from repo root: go test's working dir is package dir, so set Dir to module root
	// (two levels up from pkg/loglint)
	build.Dir = filepath.Clean(filepath.Join(".", "..", ".."))
	build.Env = append(os.Environ(), "GO111MODULE=on")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tool: %v\n%s", err, out)
	}

	// Run the tool against an isolated fixture module that depends on real zap.
	fixture := filepath.Clean(filepath.Join(build.Dir, "pkg", "loglint", "testdata", "integration"))

	// Ensure the fixture module has go.sum entries (packages.Load uses `go list`, which does not write go.sum).
	dl := exec.Command("go", "mod", "download")
	dl.Dir = fixture
	dl.Env = append(os.Environ(), "GO111MODULE=on", "GOWORK=off")
	if out, err := dl.CombinedOutput(); err != nil {
		t.Fatalf("failed to prepare fixture module (go mod download): %v\n%s", err, out)
	}
	run := exec.Command(tool, "./...")
	run.Dir = fixture
	run.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := run.CombinedOutput()
	// singlechecker typically returns non-zero when diagnostics are reported — that's OK.
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("failed to run tool: %v\n%s", err, out)
		}
	}
	s := string(out)

	// Assertions are substring-based to avoid OS-specific path differences.
	mustContain := func(substr string) {
		if !strings.Contains(s, substr) {
			t.Fatalf("expected output to contain %q\n--- output ---\n%s", substr, s)
		}
	}

	mustContain("log message should start with a lowercase letter")
	mustContain("log message should be English-only")
	mustContain("log message should not contain punctuation/symbols/emoji")
	mustContain("log message construction may leak sensitive data")
}