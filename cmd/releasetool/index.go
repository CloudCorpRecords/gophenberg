// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// indexValidity is how long an index stays valid when -valid names no other span.
	indexValidity = 720 * time.Hour
	// indexKeep is how many of the newest offers an index keeps when -keep names no other number.
	indexKeep = 20
)

// indexOptions holds the flags of one index run.
type indexOptions struct {
	channel     string
	previous    string
	record      string
	releaseJSON string
	withdraw    string
	valid       time.Duration
	keep        int
	extra       int
}

// releaseFacts is the part of an archive's release.json an offer repeats.
type releaseFacts struct {
	Version        string   `json:"version"`
	MinimumVersion string   `json:"minimum_version"`
	PostgreSQL     int      `json:"postgresql"`
	Kits           []string `json:"kits"`
	Notes          string   `json:"notes"`
}

// index writes the next index of a channel: the previous one with a release added or withdrawn.
func index(args []string, stdout, stderr io.Writer) error {
	opts, err := parseIndexFlags(args, stderr)
	if err != nil {
		return err
	}
	next, err := opts.previousIndex()
	if err != nil {
		return err
	}
	if err := opts.change(&next); err != nil {
		return err
	}
	now := time.Now()
	next.Offers = newestOffers(next.Offers, opts.keep)
	next.Serial = max(next.Serial+1, now.Unix())
	next.Expires = now.Add(opts.valid).UTC().Truncate(time.Second)
	return writeIndex(stdout, next)
}

// writeIndex writes the index to w, refusing one larger than a site reads.
func writeIndex(w io.Writer, next update.Index) error {
	var written bytes.Buffer
	_ = writeJSON(&written, next)
	if written.Len() > update.MaxFileSize {
		return fmt.Errorf("refused to write an index of %d bytes, above the %d a site reads, so keep fewer offers "+
			"with -keep", written.Len(), update.MaxFileSize)
	}
	_, err := written.WriteTo(w)
	return err
}

// parseIndexFlags reads the flags of an index run, refusing a run that neither adds nor withdraws one release.
func parseIndexFlags(args []string, stderr io.Writer) (indexOptions, error) {
	var opts indexOptions
	flags := newFlagSet("index", stderr)
	flags.StringVar(&opts.channel, "channel", "stable", "the channel the index offers")
	flags.StringVar(&opts.previous, "previous", "", "the index to change, none for the first one of the channel")
	flags.StringVar(&opts.record, "record", "", "the release record of the release to add")
	flags.StringVar(&opts.releaseJSON, "release-json", "", "the release.json of the release to add")
	flags.StringVar(&opts.withdraw, "withdraw", "", "the version to withdraw")
	flags.DurationVar(&opts.valid, "valid", indexValidity, "how long the index stays valid")
	flags.IntVar(&opts.keep, "keep", indexKeep, "how many of the newest offers the index keeps")
	if err := parseFlags(flags, args); err != nil {
		return opts, err
	}
	opts.extra = flags.NArg()
	return opts, opts.check()
}

// check refuses options that do not name one change, or name one with what it needs left out.
func (o indexOptions) check() error {
	switch {
	case o.valid <= 0 || o.extra != 0:
		return usageError{"index needs a -valid above zero and reads no other file"}
	case o.keep < 1:
		return usageError{"index needs a -keep of one offer or more"}
	case o.withdraw != "":
		return o.checkWithdrawal()
	case o.record == "" || o.releaseJSON == "":
		return usageError{"index needs -record and -release-json to add a release, or -withdraw to withdraw one"}
	}
	return nil
}

// checkWithdrawal refuses a withdrawal that also adds a release or names no index to withdraw from.
func (o indexOptions) checkWithdrawal() error {
	switch {
	case o.record != "" || o.releaseJSON != "":
		return usageError{"index adds a release or withdraws one, but not both in one run"}
	case o.previous == "":
		return usageError{"index needs -previous, the index to withdraw a version from"}
	}
	return nil
}

// previousIndex returns the index the run changes, an empty one of the channel when -previous names none.
func (o indexOptions) previousIndex() (update.Index, error) {
	if o.previous == "" {
		return update.Index{Schema: 1, Channel: o.channel}, nil
	}
	var previous update.Index
	if err := readJSON(o.previous, "the previous index", &previous); err != nil {
		return previous, err
	}
	if previous.Channel != o.channel {
		return previous, fmt.Errorf("refused %s, which offers the channel %q, not %q", o.previous, previous.Channel,
			o.channel)
	}
	return previous, nil
}

// change withdraws the version -withdraw names from the index, or adds the release the record names.
func (o indexOptions) change(next *update.Index) error {
	if o.withdraw != "" {
		return withdraw(next, o.withdraw)
	}
	offer, err := o.offer()
	if err != nil {
		return err
	}
	next.Offers = slices.DeleteFunc(next.Offers, func(held update.Offer) bool { return held.Version() == offer.Version() })
	next.Offers = append(next.Offers, offer)
	return nil
}

// newestOffers returns the keep newest offers, the newest first.
func newestOffers(offers []update.Offer, keep int) []update.Offer {
	slices.SortStableFunc(offers, newestFirst)
	return offers[:min(len(offers), keep)]
}

// offer returns the offer of the release the record and its release.json describe, to every site and notify only.
func (o indexOptions) offer() (update.Offer, error) {
	var record json.RawMessage
	if err := readJSON(o.record, "the release record", &record); err != nil {
		return update.Offer{}, err
	}
	var facts releaseFacts
	if err := readJSON(o.releaseJSON, "the release.json", &facts); err != nil {
		return update.Offer{}, err
	}
	offer := update.Offer{
		Rollout: 100, MinimumVersion: facts.MinimumVersion, PostgreSQL: facts.PostgreSQL, Kits: facts.Kits,
		Notes: facts.Notes, Release: compacted(record),
	}
	switch {
	case offer.Version() != facts.Version:
		return offer, fmt.Errorf("refused %s, a record of %q, beside a release.json of %q", o.record, offer.Version(),
			facts.Version)
	case !version.Valid(facts.MinimumVersion):
		return offer, fmt.Errorf("refused %s, whose minimum_version %q is no full version", o.releaseJSON,
			facts.MinimumVersion)
	}
	return offer, nil
}

// withdraw marks the offer of version as withdrawn, refusing a version the index does not offer.
func withdraw(next *update.Index, withdrawn string) error {
	for i := range next.Offers {
		if next.Offers[i].Version() == withdrawn {
			next.Offers[i].Withdrawn = true
			return nil
		}
	}
	return fmt.Errorf("refused to withdraw %s, which the index does not offer", withdrawn)
}

// newestFirst orders two offers by version, the newest first.
func newestFirst(a, b update.Offer) int {
	return version.Compare(b.Version(), a.Version())
}

// readJSON reads the JSON file at path, described as what, into v.
func readJSON(path, what string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", what, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("refused %s %s, which is not JSON: %w", what, path, err)
	}
	return nil
}

// compacted returns the JSON of raw without the spaces between its tokens.
func compacted(raw json.RawMessage) json.RawMessage {
	var held bytes.Buffer
	_ = json.Compact(&held, raw)
	return held.Bytes()
}
