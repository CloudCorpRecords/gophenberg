// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// entry is one entry of a test archive.
type entry struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	linkname string
}

// file returns a regular file entry of name holding body.
func file(name, body string) entry {
	return entry{name: name, body: body, mode: 0o644, typeflag: tar.TypeReg}
}

// binary returns the entry of the Gophenberg binary, executable.
func binary() entry {
	return entry{name: "gophenberg", body: "the Gophenberg 0.21.0 binary", mode: 0o755, typeflag: tar.TypeReg}
}

// releaseEntries returns the entries of a release archive, as make release-archives packs them.
func releaseEntries() []entry {
	return []entry{
		file("LICENSE", "the Apache License 2.0"),
		{name: "LICENSES/", mode: 0o755, typeflag: tar.TypeDir},
		file("LICENSES/GPL-3.0.txt", "the GNU General Public License 3.0"),
		file("NOTICE", "the notice of Gophenberg"),
		binary(),
		file("release.json", `{"version":"0.21.0"}`),
	}
}

// writeHeaders writes the header and the body of each entry into archive, failing the test when it cannot.
func writeHeaders(t *testing.T, archive *tar.Writer, entries []entry) {
	t.Helper()
	for _, e := range entries {
		header := &tar.Header{
			Name: e.name, Mode: e.mode, Typeflag: e.typeflag, Linkname: e.linkname, Size: int64(len(e.body)),
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatalf("writing the header of %s: %v", e.name, err)
		}
		if _, err := archive.Write([]byte(e.body)); err != nil {
			t.Fatalf("writing the body of %s: %v", e.name, err)
		}
	}
}

// gzipped returns body compressed with gzip.
func gzipped(t *testing.T, body []byte) []byte {
	t.Helper()
	var held bytes.Buffer
	zipped := gzip.NewWriter(&held)
	if _, err := zipped.Write(body); err != nil {
		t.Fatalf("compressing the archive: %v", err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatalf("closing the compressed archive: %v", err)
	}
	return held.Bytes()
}

// packed returns the tar.gz of entries.
func packed(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var held bytes.Buffer
	archive := tar.NewWriter(&held)
	writeHeaders(t, archive, entries)
	if err := archive.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return gzipped(t, held.Bytes())
}

// buildFolder returns the path of a build folder that is not there yet, in a new folder.
func buildFolder(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "0.21.0")
}

// wantFile fails t unless the file at path holds body.
func wantFile(t *testing.T, path, body string) {
	t.Helper()
	held, err := os.ReadFile(path)
	if err != nil || string(held) != body {
		t.Errorf("%s holds %q, %v, want %q", path, held, err, body)
	}
}

// executable reports whether the owner may run the file at path, failing t when it cannot be read.
func executable(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return info.Mode().Perm()&0o100 != 0
}

func TestUnpackWritesTheFilesOfARelease(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)

	if err := updater.Unpack(bytes.NewReader(packed(t, releaseEntries()...)), dir); err != nil {
		t.Fatalf("Unpack() = %v, want nil", err)
	}

	for _, e := range releaseEntries() {
		if e.typeflag == tar.TypeReg {
			wantFile(t, filepath.Join(dir, filepath.FromSlash(e.name)), e.body)
		}
	}
	if !executable(t, filepath.Join(dir, "gophenberg")) {
		t.Errorf("gophenberg is not executable, want it run as the next build")
	}
}

func TestUnpackWritesEveryLicenseTextOfTheLicensesFolder(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)
	added := file("LICENSES/MIT.txt", "the MIT License")
	added.mode = 0o755

	if err := updater.Unpack(bytes.NewReader(packed(t, append(releaseEntries(), added)...)), dir); err != nil {
		t.Fatalf("Unpack() of an archive with a second license text = %v, want nil", err)
	}

	wantFile(t, filepath.Join(dir, "LICENSES", "MIT.txt"), "the MIT License")
	if executable(t, filepath.Join(dir, "LICENSES", "MIT.txt")) {
		t.Errorf("LICENSES/MIT.txt packed as 0755 is executable, want it written as a plain file")
	}
}

func TestUnpackKeepsOnlyTheBinaryExecutableWhateverTheArchiveSays(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)
	plain := binary()
	plain.mode = 0o644
	runnable := file("release.json", `{"version":"0.21.0"}`)
	runnable.mode = 0o755

	if err := updater.Unpack(bytes.NewReader(packed(t, plain, runnable)), dir); err != nil {
		t.Fatalf("Unpack() = %v, want nil", err)
	}

	if !executable(t, filepath.Join(dir, "gophenberg")) {
		t.Errorf("gophenberg packed as 0644 is not executable, want it executable")
	}
	if executable(t, filepath.Join(dir, "release.json")) {
		t.Errorf("release.json packed as 0755 is executable, want it written as a plain file")
	}
}

func TestUnpackTakesNamesThatStartAtTheArchiveFolder(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)
	named := binary()
	named.name = "./gophenberg"

	if err := updater.Unpack(bytes.NewReader(packed(t, named, file("./LICENSES/GPL-3.0.txt", "GPL"))), dir); err != nil {
		t.Fatalf("Unpack() = %v, want nil", err)
	}

	wantFile(t, filepath.Join(dir, "gophenberg"), named.body)
	wantFile(t, filepath.Join(dir, "LICENSES", "GPL-3.0.txt"), "GPL")
}

func TestUnpackRefusesAnEntryOutsideTheBuildFolder(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../gophenberg", "/gophenberg", "LICENSES/../../gophenberg", ""} {
		dir := buildFolder(t)
		escaping := binary()
		escaping.name = name

		err := updater.Unpack(bytes.NewReader(packed(t, escaping)), dir)

		wantRefused(t, err, "refused the archive, its entry \""+name+"\" is outside the build folder")
		wantMissing(t, dir)
		wantMissing(t, filepath.Join(filepath.Dir(dir), "gophenberg"))
	}
}

func TestUnpackRefusesALink(t *testing.T) {
	t.Parallel()

	for _, link := range []entry{
		{name: "gophenberg", mode: 0o777, typeflag: tar.TypeSymlink, linkname: "/bin/sh"},
		{name: "NOTICE", mode: 0o644, typeflag: tar.TypeLink, linkname: "LICENSE"},
	} {
		dir := buildFolder(t)

		err := updater.Unpack(bytes.NewReader(packed(t, file("LICENSE", "the license"), link, binary())), dir)

		wantRefused(t, err, "refused the archive, its entry \""+link.name+"\" is a link")
		wantMissing(t, dir)
	}
}

func TestUnpackRefusesAnEntryAReleaseDoesNotHold(t *testing.T) {
	t.Parallel()

	for _, unknown := range []entry{
		file("plugins.so", "a plugin"),
		file("LICENSES", "a license"),
		file("licenses/GPL-3.0.txt", "GPL"),
		file("LICENSES/more/MIT.txt", "MIT"),
		{name: "LICENSES/more/", mode: 0o755, typeflag: tar.TypeDir},
		{name: "LICENSES/MIT.txt", mode: 0o644, typeflag: tar.TypeFifo},
		{name: "themes/", mode: 0o755, typeflag: tar.TypeDir},
		{name: "gophenberg/", mode: 0o755, typeflag: tar.TypeDir},
		{name: "NOTICE", mode: 0o644, typeflag: tar.TypeFifo},
	} {
		dir := buildFolder(t)

		err := updater.Unpack(bytes.NewReader(packed(t, binary(), unknown)), dir)

		wantRefused(t, err, "refused the archive, its entry \""+unknown.name+"\" is not a file a release holds")
		wantMissing(t, dir)
	}
}

func TestUnpackRefusesAnArchiveWithNoBinary(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)

	err := updater.Unpack(bytes.NewReader(packed(t, file("LICENSE", "the license"), file("NOTICE", "the notice"))), dir)

	wantRefused(t, err, "refused the archive, which holds no gophenberg binary")
	wantMissing(t, dir)
}

func TestUnpackRefusesAnEntryTwice(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)

	err := updater.Unpack(bytes.NewReader(packed(t, binary(), binary())), dir)

	wantRefused(t, err, "file exists")
	wantMissing(t, dir)
}

func TestUnpackRefusesAnArchiveItCannotRead(t *testing.T) {
	t.Parallel()

	var cut bytes.Buffer
	archive := tar.NewWriter(&cut)
	if err := archive.WriteHeader(&tar.Header{Name: "gophenberg", Mode: 0o755, Size: 100}); err != nil {
		t.Fatalf("writing the header: %v", err)
	}
	if _, err := archive.Write([]byte("short")); err != nil {
		t.Fatalf("writing the body: %v", err)
	}
	for text, body := range map[string][]byte{
		"refused the archive, which is not gzip":    []byte("not an archive"),
		"refused the archive, which cannot be read": gzipped(t, []byte(strings.Repeat("x", 1024))),
		"unpack gophenberg: unexpected EOF":         gzipped(t, cut.Bytes()),
	} {
		dir := buildFolder(t)

		err := updater.Unpack(bytes.NewReader(body), dir)

		wantRefused(t, err, text)
		wantMissing(t, dir)
	}
}

func TestUnpackRefusesABuildItCannotSaveToDisk(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which reads a folder whatever its mode says")
	}
	dir := buildFolder(t)
	versions := filepath.Dir(dir)
	if err := os.Chmod(versions, 0o300); err != nil {
		t.Fatalf("denying read on %s: %v", versions, err)
	}
	t.Cleanup(func() { _ = os.Chmod(versions, 0o755) })

	err := updater.Unpack(bytes.NewReader(packed(t, releaseEntries()...)), dir)

	wantRefused(t, err, "save the build to disk")
	wantMissing(t, dir)
}

func TestUnpackLeavesAFolderThatIsThereAlone(t *testing.T) {
	t.Parallel()

	dir := buildFolder(t)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("making the folder: %v", err)
	}
	kept := filepath.Join(dir, "gophenberg")
	if err := os.WriteFile(kept, []byte("the running binary"), 0o755); err != nil {
		t.Fatalf("writing the running binary: %v", err)
	}

	err := updater.Unpack(bytes.NewReader(packed(t, releaseEntries()...)), dir)

	wantRefused(t, err, "file exists")
	wantFile(t, kept, "the running binary")
}
