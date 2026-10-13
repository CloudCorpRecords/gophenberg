// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// isolated returns the environment without the git variables that point at the caller's repository.
func isolated() []string {
	return slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, "GIT_") })
}

// inRepo runs the command in the directory and returns what it printed.
func inRepo(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = isolated()
	out, err := command.CombinedOutput()
	return string(out), err
}

// kitRepo returns a git repository holding the Makefile and a kit manifest at the version, tagged when asked.
func kitRepo(t *testing.T, version string, tagged bool) string {
	t.Helper()
	for _, tool := range []string{"make", "npm", "node", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the kit bump cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	kit := filepath.Join(root, "sdk", "astro")
	if err := os.MkdirAll(kit, 0o755); err != nil {
		t.Fatalf("making the kit directory: %v", err)
	}
	manifest := `{"name": "@gophenberg/astro", "version": "` + version + `"}`
	for path, body := range map[string][]byte{
		filepath.Join(root, "Makefile"):    makefile,
		filepath.Join(kit, "package.json"): []byte(manifest),
	} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	steps := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=Maria Perez", "-c", "user.email=maria@example.com", "commit", "-q", "-m", "kit"},
	}
	if tagged {
		steps = append(steps, []string{"tag", "astro@" + version})
	}
	for _, step := range steps {
		if out, err := inRepo(t, root, "git", step...); err != nil {
			t.Fatalf("git %v: %v\n%s", step, err, out)
		}
	}
	return root
}

// kitVersion returns the version the kit manifest in the repository names.
func kitVersion(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "sdk", "astro", "package.json"))
	if err != nil {
		t.Fatalf("reading the kit manifest: %v", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decoding the kit manifest: %v", err)
	}
	return manifest.Version
}

func TestBumpKitRefusesToReplaceAVersionThatWasNeverTagged(t *testing.T) {
	t.Parallel()

	root := kitRepo(t, "0.15.0", false)

	out, err := inRepo(t, root, "make", "bump-kit", "V=0.16.0")

	if err == nil {
		t.Fatalf("make bump-kit succeeded, want it refused while astro@0.15.0 carries no tag\n%s", out)
	}
	if !strings.Contains(out, "astro@0.15.0") {
		t.Errorf("output = %q, want it to name the untagged astro@0.15.0", out)
	}
	if got := kitVersion(t, root); got != "0.15.0" {
		t.Errorf("kit version = %q, want 0.15.0 left as it was", got)
	}
}

func TestBumpKitReplacesAVersionThatWasTagged(t *testing.T) {
	t.Parallel()

	root := kitRepo(t, "0.15.0", true)

	out, err := inRepo(t, root, "make", "bump-kit", "V=0.16.0")

	if err != nil {
		t.Fatalf("make bump-kit error = %v, want the tagged version replaced\n%s", err, out)
	}
	if got := kitVersion(t, root); got != "0.16.0" {
		t.Errorf("kit version = %q, want 0.16.0", got)
	}
}

// decoyRepo returns the git directory of an empty repository standing in for the caller's own.
func decoyRepo(t *testing.T) string {
	t.Helper()
	gitDir := filepath.Join(t.TempDir(), ".git")
	command := exec.Command("git", "init", "-q")
	command.Env = append(os.Environ(), "GIT_DIR="+gitDir)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init of the decoy: %v\n%s", err, out)
	}
	return gitDir
}

// decoyRefs returns every branch and tag the git directory holds, one per line.
func decoyRefs(t *testing.T, gitDir string) string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", gitDir, "for-each-ref", "--format=%(refname)").Output()
	if err != nil {
		t.Fatalf("listing the decoy's refs: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestFixtureRepositoriesLeaveTheRepositoryGitDirNamesAlone(t *testing.T) {
	decoy := decoyRepo(t)
	t.Setenv("GIT_DIR", decoy)

	kitRepo(t, "0.15.0", true)
	sdkRepo(t, tagStyles["lightweight"])

	if refs := decoyRefs(t, decoy); refs != "" {
		t.Errorf("the repository GIT_DIR names gained %q, want the fixtures kept in their own folders", refs)
	}
}

func TestMakeRunsFreeOfTheCallersGitRepository(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not on PATH")
	}
	dir := t.TempDir()
	recipe := "show:\n\t@printf '%s|%s' \"$$GIT_DIR\" \"$$GIT_WORK_TREE\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(recipe), 0o644); err != nil {
		t.Fatalf("writing the Makefile: %v", err)
	}
	t.Setenv("GIT_DIR", filepath.Join(dir, "elsewhere"))
	t.Setenv("GIT_WORK_TREE", dir)

	out, err := makeIn(t, dir, false, "show")

	if err != nil || out != "|" {
		t.Errorf("make show = %v with output %q, want no git directory or work tree passed on", err, out)
	}
}
