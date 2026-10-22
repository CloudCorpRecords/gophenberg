// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

// timestampOptions holds the flags of one timestamp run.
type timestampOptions struct {
	site   *siteFlags
	newest newestFlags
	valid  time.Duration
}

// newestFlags holds the newest version of each channel, as -newest names them.
type newestFlags map[string]string

// timestamp writes the timestamp file of a clone of the update site, naming its key list and each index a site takes.
func timestamp(args []string, stdout, stderr io.Writer) error {
	opts, err := parseTimestampFlags(args, stderr)
	if err != nil {
		return err
	}
	now := time.Now()
	list, err := opts.site.keyList(now)
	if err != nil {
		return err
	}
	channels, err := opts.channels()
	if err != nil {
		return err
	}
	indexes, leftOut := opts.indexes(list, channels, now)
	note := update.Timestamp{
		Schema: 1, Expires: now.Add(opts.valid).UTC().Truncate(time.Second), Keys: list.Serial, Indexes: indexes,
	}
	return errors.Join(writeJSON(stdout, note), leftOut)
}

// parseTimestampFlags reads the flags of a timestamp run, refusing one that leaves out the clone, a key or -valid.
func parseTimestampFlags(args []string, stderr io.Writer) (timestampOptions, error) {
	opts := timestampOptions{newest: newestFlags{}}
	flags := newFlagSet("timestamp", stderr)
	opts.site = addSiteFlags(flags)
	flags.Var(opts.newest, "newest", "a channel and the newest version its index offers, as stable=0.20.1")
	flags.DurationVar(&opts.valid, "valid", 0, "how long the timestamp file stays valid")
	if err := parseFlags(flags, args); err != nil {
		return opts, err
	}
	if err := opts.site.check("timestamp"); err != nil {
		return opts, err
	}
	if opts.valid <= 0 || flags.NArg() != 0 {
		return opts, usageError{"timestamp needs a -valid above zero and reads no file"}
	}
	return opts, nil
}

// channels returns the channels the clone holds an index of and the channels -newest names, each once.
func (o timestampOptions) channels() ([]string, error) {
	held, err := o.site.channels()
	if err != nil {
		return nil, err
	}
	channels := slices.AppendSeq(held, maps.Keys(o.newest))
	slices.Sort(channels)
	return slices.Compact(channels), nil
}

// indexes returns the serial of each channel's index a site takes at now, and an error naming each channel left out.
func (o timestampOptions) indexes(list update.KeyList, channels []string, now time.Time) (map[string]int64, error) {
	named := map[string]int64{}
	var leftOut []error
	for _, channel := range channels {
		serial, err := o.serialOf(list, channel, now)
		if err != nil {
			leftOut = append(leftOut, fmt.Errorf("left the channel %q out of the timestamp file: %w", channel, err))
			continue
		}
		named[channel] = serial
	}
	return named, errors.Join(leftOut...)
}

// serialOf returns the serial of channel's index, refusing one missing, one a site refuses or one without the newest.
func (o timestampOptions) serialOf(list update.KeyList, channel string, now time.Time) (int64, error) {
	if !o.site.hasIndex(channel) {
		return 0, fmt.Errorf("the update site holds no index to offer %s, the newest version -newest names",
			o.newest[channel])
	}
	body, sig, err := o.site.read(update.IndexPath(channel))
	if err != nil {
		return 0, err
	}
	index, err := list.ReadIndex(body, sig, channel, now)
	if err != nil {
		return 0, err
	}
	return index.Serial, o.newest.offered(index)
}

// String returns an empty string.
func (n newestFlags) String() string {
	return ""
}

// Set adds the channel and the newest version value names as channel=version.
func (n newestFlags) Set(value string) error {
	channel, newest, _ := strings.Cut(value, "=")
	if channel == "" || !version.Valid(newest) {
		return fmt.Errorf("%q names no channel=version, such as stable=0.20.1", value)
	}
	if _, twice := n[channel]; twice {
		return fmt.Errorf("names the channel %q twice", channel)
	}
	n[channel] = newest
	return nil
}

// offered refuses an index that does not offer the newest version -newest names for its channel.
func (n newestFlags) offered(index update.Index) error {
	newest, named := n[index.Channel]
	if !named || slices.ContainsFunc(index.Offers, func(offer update.Offer) bool { return offer.Version() == newest }) {
		return nil
	}
	return fmt.Errorf("refused the index of channel %q, which does not offer %s, the newest version -newest names",
		index.Channel, newest)
}
