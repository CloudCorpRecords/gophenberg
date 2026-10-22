// SPDX-License-Identifier: Apache-2.0

package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// errUnsignedTimestamp refuses a timestamp file no timestamp key of the key list signed.
var errUnsignedTimestamp = errors.New("update: refused the timestamp file, no timestamp key the list names signed it")

// ReadTimestamp returns the timestamp file body holds once a timestamp key the list names signed it, valid at now.
func (l KeyList) ReadTimestamp(body, sig []byte, now time.Time) (Timestamp, error) {
	if !signedBy(body, sig, l.TimestampKeys()) {
		return Timestamp{}, errUnsignedTimestamp
	}
	var stamp Timestamp
	if err := json.Unmarshal(body, &stamp); err != nil {
		return Timestamp{}, fmt.Errorf("update: refused the timestamp file, which is not JSON: %w", err)
	}
	if err := readable("timestamp file", stamp.Schema, stamp.Expires, now); err != nil {
		return Timestamp{}, err
	}
	return stamp, nil
}

// indexOf returns the serial the timestamp file names for the index of channel, refusing a channel it leaves out.
func (t Timestamp) indexOf(channel string) (int64, error) {
	serial, found := t.Indexes[channel]
	if !found {
		return 0, fmt.Errorf("update: refused the timestamp file, which names no index of channel %q", channel)
	}
	return serial, nil
}
