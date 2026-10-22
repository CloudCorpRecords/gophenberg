// SPDX-License-Identifier: Apache-2.0

package main

import (
	"cmp"
	"crypto/ecdsa"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// siteFlags holds the flags that name a clone of the update site, the keys its key list names and its roots.
type siteFlags struct {
	clone string
	pins  map[update.Role]*string
	roots rootFlags
}

// rootFlags holds the root keys -root names, one for each time it is given.
type rootFlags []*ecdsa.PublicKey

// addSiteFlags adds to flags the clone of the update site, the keys its key list names and its roots.
func addSiteFlags(flags *flag.FlagSet) *siteFlags {
	site := &siteFlags{pins: keyFlags(flags)}
	flags.StringVar(&site.clone, "site", "", "a clone of the update site")
	flags.Var(&site.roots, "root", "the Multikey of a root that signs the key list, the official roots when left out")
	return site
}

// keyFlags adds -release, -standby and -timestamp to flags, the Multikeys of the keys a key list names.
func keyFlags(flags *flag.FlagSet) map[update.Role]*string {
	return map[update.Role]*string{
		update.RoleRelease:   flags.String("release", "", "the Multikey of the release key"),
		update.RoleStandby:   flags.String("standby", "", "the Multikey of the standby"),
		update.RoleTimestamp: flags.String("timestamp", "", "the Multikey of the timestamp key"),
	}
}

// missingKey reports whether -release, -standby or -timestamp was left out.
func missingKey(named map[update.Role]*string) bool {
	return slices.ContainsFunc(listedRoles, func(role update.Role) bool { return *named[role] == "" })
}

// String returns an empty string.
func (r *rootFlags) String() string {
	return ""
}

// Set adds the root key value names, refusing a value that names no root key or more than one.
func (r *rootFlags) Set(value string) error {
	roots, err := update.ParseRoots(value)
	if err != nil {
		return err
	}
	if len(roots) != 1 {
		return fmt.Errorf("names %d root keys, where -root takes one", len(roots))
	}
	*r = append(*r, roots...)
	return nil
}

// check refuses flags that leave out the clone or one of its three keys.
func (s *siteFlags) check(command string) error {
	if s.clone == "" || missingKey(s.pins) {
		return usageError{command + " needs -site, a clone of the update site, and -release, -standby and " +
			"-timestamp, the Multikeys its key list names"}
	}
	return nil
}

// keyList reads the key list of the clone as a site does at now, refusing one naming other keys than the flags.
func (s *siteFlags) keyList(now time.Time) (update.KeyList, error) {
	pinned, err := listedKeys(s.pins)
	if err != nil {
		return update.KeyList{}, err
	}
	body, sigs, err := s.read(update.KeyListPath)
	if err != nil {
		return update.KeyList{}, err
	}
	list, err := update.ReadKeyList(body, sigs, s.trustedRoots(), now)
	if err != nil {
		return update.KeyList{}, err
	}
	if !slices.Equal(sortedKeys(list.Keys), sortedKeys(pinned)) {
		return update.KeyList{}, fmt.Errorf("refused the key list of %s, which does not name exactly the keys "+
			"-release, -standby and -timestamp name", s.clone)
	}
	return list, nil
}

// trustedRoots returns the roots -root names, the official roots when it names none.
func (s *siteFlags) trustedRoots() []*ecdsa.PublicKey {
	if len(s.roots) == 0 {
		return update.Roots()
	}
	return s.roots
}

// sortedKeys returns a copy of keys sorted by role, then by Multikey.
func sortedKeys(keys []update.ListedKey) []update.ListedKey {
	return slices.SortedFunc(slices.Values(keys), func(a, b update.ListedKey) int {
		return cmp.Or(cmp.Compare(a.Role, b.Role), cmp.Compare(a.Multikey, b.Multikey))
	})
}

// channels returns the channels the clone holds an index of, by the names of their files.
func (s *siteFlags) channels() ([]string, error) {
	folder, suffix, _ := strings.Cut(update.IndexPath("*"), "*")
	entries, err := os.ReadDir(s.path(folder))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the update site: %w", err)
	}
	var channels []string
	for _, entry := range entries {
		if channel, isIndex := strings.CutSuffix(entry.Name(), suffix); isIndex {
			channels = append(channels, channel)
		}
	}
	return channels, nil
}

// hasIndex reports whether the clone holds an index of channel.
func (s *siteFlags) hasIndex(channel string) bool {
	_, err := os.Stat(s.path(update.IndexPath(channel)))
	return !errors.Is(err, fs.ErrNotExist)
}

// read returns the file at path of the update site in the clone, and the .sig beside it.
func (s *siteFlags) read(path string) ([]byte, []byte, error) {
	body, err := os.ReadFile(s.path(path))
	if err != nil {
		return nil, nil, fmt.Errorf("reading the update site: %w", err)
	}
	sig, err := os.ReadFile(s.path(path + update.SignatureSuffix))
	if err != nil {
		return nil, nil, fmt.Errorf("reading the update site: %w", err)
	}
	return body, sig, nil
}

// path returns where path of the update site sits in the clone.
func (s *siteFlags) path(path string) string {
	return filepath.Join(s.clone, filepath.FromSlash(path))
}
