// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// releaseRepo returns a git repository holding the Makefile, a stand-in admin package and an admin build whose
// page is page, with a pnpm on its bin folder that only records how it was called.
func releaseRepo(t *testing.T, page string) string {
	t.Helper()
	for _, tool := range []string{"make", "go", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so make build-release cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	files := map[string]string{
		"Makefile":                      string(makefile),
		"go.mod":                        "module example.com/site\n\ngo 1.27.1\n",
		"frontend/dist/index.html":      page,
		"internal/webui/dist/stale.txt": "left by an older build\n",
		"internal/webui/embed.go": "//go:build embedweb\n\npackage webui\n\nimport _ \"embed\"\n\n" +
			"//go:embed dist/index.html\nvar page string\n\nfunc Page() string { return page }\n",
		"internal/webui/plain.go": "//go:build !embedweb\n\npackage webui\n\nfunc Page() string { return \"no admin\" }\n",
		"internal/webui/release_test.go": "//go:build embedweb\n\npackage webui\n\n" +
			"import (\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestPage(t *testing.T) {\n\tif !strings.Contains(Page(), \"built admin\") {\n" +
			"\t\tt.Fatal(\"the page is not the built admin\")\n\t}\n}\n",
		"cmd/gophenberg/main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/site/internal/webui\"\n)\n\n" +
			"func main() { fmt.Print(webui.Page()) }\n",
		"bin/pnpm": "#!/bin/sh\nprintf '%s\\n' \"$*\" > pnpm-args\n",
	}
	for path, body := range files {
		writeFile(t, filepath.Join(root, path), body)
	}
	if err := os.Chmod(filepath.Join(root, "bin", "pnpm"), 0o755); err != nil {
		t.Fatalf("making the stand-in pnpm runnable: %v", err)
	}
	runGit(t, root, []string{"init", "-q"}, []string{"add", "-A"}, commitStep())
	return root
}

// releaseIn runs make in the repository with its stand-in pnpm first on PATH.
func releaseIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	path := "PATH=" + filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	return makeIn(t, root, false, append(args, path)...)
}

// platformFolder names the folder make build-release writes a platform's binary into.
func platformFolder(platform string) string {
	return strings.Replace(platform, "/", "_", 1)
}

func TestMakeBuildReleaseBuildsEachPlatformWithTheBuiltAdminInside(t *testing.T) {
	t.Parallel()

	root := releaseRepo(t, "<p>the built admin</p>")
	host, other := runtime.GOOS+"/"+runtime.GOARCH, "linux/arm64"
	if host == other {
		other = "linux/amd64"
	}

	out, err := releaseIn(t, root, "build-release", "RELEASE_PLATFORMS="+host+" "+other)

	if err != nil {
		t.Fatalf("make build-release = %v with output %q", err, out)
	}
	if args, err := os.ReadFile(filepath.Join(root, "pnpm-args")); err != nil ||
		string(args) != "--filter @gophenberg/frontend build\n" {
		t.Errorf("pnpm ran with %q (%v), want the admin app built", args, err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "webui", "dist", "stale.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an older admin copy stayed beside the new one: %v", err)
	}
	binary := filepath.Join(root, "dist", platformFolder(host), "gophenberg")
	if printed, err := exec.Command(binary).Output(); err != nil || string(printed) != "<p>the built admin</p>" {
		t.Errorf("the %s binary printed %q (%v), want the built admin page inside it", host, printed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", platformFolder(other), "gophenberg")); err != nil {
		t.Errorf("no %s binary: %v", other, err)
	}
	settings, err := exec.Command("go", "version", "-m", binary).Output()
	for _, want := range []string{"-tags=embedweb", "-trimpath=true", "CGO_ENABLED=0"} {
		if err != nil || !strings.Contains(string(settings), want) {
			t.Errorf("the binary was built with %q (%v), want %s", settings, err, want)
		}
	}
	if strings.Contains(string(settings), "vcs.revision") {
		t.Errorf("the binary carries the commit it was built from, %q, want no version control facts", settings)
	}
}

func TestMakeBuildReleaseStopsWhenTheCopiedAdminFailsTheReleaseCheck(t *testing.T) {
	t.Parallel()

	root := releaseRepo(t, "<p>a page that holds no admin</p>")

	out, err := releaseIn(t, root, "build-release")

	if err == nil || !strings.Contains(out, "the page is not the built admin") {
		t.Errorf("make build-release = %v with output %q, want it stopped by the release check", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("make build-release built binaries over an admin that failed its check: %v", err)
	}
}
