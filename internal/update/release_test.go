// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

// archives returns the release record of version listing its archives for linux-amd64 and linux-arm64.
func archives(t *testing.T, version string) json.RawMessage {
	t.Helper()
	listed := make([]update.Artifact, 0, 2)
	for _, arch := range []string{"amd64", "arm64"} {
		name := "gophenberg_" + version + "_linux_" + arch + ".tar.gz"
		listed = append(listed, update.Artifact{
			ID: "linux-" + arch, Platform: "linux/" + arch, ContentType: "application/gzip",
			URL: "https://example.com/download/" + name, Checksum: "sha256:" + name, Signature: "signature-of-" + name,
		})
	}
	return marshal(t, update.Release{Version: version, Artifacts: update.Artifacts{Package: listed}})
}

// readRelease returns the release record body holds, failing the test when it cannot be read.
func readRelease(t *testing.T, body json.RawMessage) update.Release {
	t.Helper()
	var held update.Release
	if err := json.Unmarshal(body, &held); err != nil {
		t.Fatalf("reading the release record: %v", err)
	}
	return held
}

func TestArchiveForPicksTheArchiveOfThePlatformNamed(t *testing.T) {
	t.Parallel()

	held := readRelease(t, archives(t, "0.21.0"))

	got, err := held.ArchiveFor("linux-arm64")

	if want := "https://example.com/download/gophenberg_0.21.0_linux_arm64.tar.gz"; err != nil || got.URL != want {
		t.Errorf("ArchiveFor(linux-arm64) = %+v, %v, want the linux-arm64 archive", got, err)
	}
}

func TestArchiveForRefusesAPlatformTheReleaseBuildsNoArchiveFor(t *testing.T) {
	t.Parallel()

	held := readRelease(t, archives(t, "0.21.0"))
	for _, id := range []string{"windows-amd64", "darwin-amd64"} {
		_, err := held.ArchiveFor(id)
		wantRefused(t, err, "Gophenberg 0.21.0 publishes no archive for "+id)
	}
}

func TestArchiveForTakesOnlyATarGzArchive(t *testing.T) {
	t.Parallel()

	zipped := func(id string) update.Artifact {
		return update.Artifact{
			ID: id, ContentType: "application/zip", URL: "https://example.com/download/gophenberg_0.21.0_" + id + ".zip",
		}
	}
	held := readRelease(t, archives(t, "0.21.0"))
	held.Artifacts.Package = append([]update.Artifact{zipped("linux-amd64")}, held.Artifacts.Package...)
	held.Artifacts.Package = append(held.Artifacts.Package, zipped("windows-amd64"))

	got, err := held.ArchiveFor("linux-amd64")
	if want := "https://example.com/download/gophenberg_0.21.0_linux_amd64.tar.gz"; err != nil || got.URL != want {
		t.Errorf("ArchiveFor(linux-amd64) = %+v, %v, want the linux-amd64 tar.gz listed after its zip", got, err)
	}
	_, err = held.ArchiveFor("windows-amd64")
	wantRefused(t, err, "Gophenberg 0.21.0 publishes no archive for windows-amd64")
}

func TestPlatformNamesTheSystemAndProcessorThisBinaryRunsOn(t *testing.T) {
	t.Parallel()

	if got, want := update.Platform(), runtime.GOOS+"-"+runtime.GOARCH; got != want {
		t.Errorf("Platform() = %q, want %q", got, want)
	}
}
