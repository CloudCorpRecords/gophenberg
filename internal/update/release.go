// SPDX-License-Identifier: Apache-2.0

package update

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// archiveType is the content type of the tar.gz archive an update unpacks, the only kind ArchiveFor takes.
const archiveType = "application/gzip"

// Release is the release record an offer carries: the version and the archives it publishes.
type Release struct {
	Version   string    `json:"version"`
	Artifacts Artifacts `json:"artifacts"`
}

// Artifacts holds the files of a release record by kind.
type Artifacts struct {
	Package []Artifact `json:"package"`
}

// Artifact is one archive a release record lists, with what an update needs to pick, fetch and check it.
type Artifact struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	ContentType string `json:"content-type"`
	URL         string `json:"url"`
	Checksum    string `json:"checksum"`
	Signature   string `json:"signature"`
}

// Platform returns the id of the archive built for the system and processor this binary runs on, like linux-amd64.
func Platform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// ArchiveFor returns the tar.gz archive the release record lists under id, refusing a platform it publishes none for.
func (r Release) ArchiveFor(id string) (Artifact, error) {
	for _, listed := range r.Artifacts.Package {
		if listed.ID == id && listed.ContentType == archiveType {
			return listed, nil
		}
	}
	return Artifact{}, fmt.Errorf("update: Gophenberg %s publishes no archive for %s", r.Version, id)
}

// Version returns the version of the release an offer carries.
func (o Offer) Version() string {
	var held Release
	_ = json.Unmarshal(o.Release, &held)
	return held.Version
}

// record returns the release record an offer carries, refusing one that cannot be read.
func (o Offer) record() (Release, error) {
	var held Release
	if err := json.Unmarshal(o.Release, &held); err != nil {
		return Release{}, fmt.Errorf("update: refused the release record of Gophenberg %s, which cannot be read: %w",
			o.Version(), err)
	}
	return held, nil
}
