// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// stateName is the file of the state folder that holds the state of the last update.
const stateName = "update.json"

// Step names how far an update got, written to the state file before the step runs.
type Step string

const (
	// StepDownload downloads the archive and checks its checksum and signature.
	StepDownload Step = "download"
	// StepStage unpacks the archive into its build folder and checks the build.
	StepStage Step = "stage"
	// StepBackup backs up the database before the switch.
	StepBackup Step = "backup"
	// StepSwitched points current at the new build, which then waits for a restart and its first start check.
	StepSwitched Step = "switched"
	// StepCommitted keeps an update whose new build passed its start check.
	StepCommitted Step = "committed"
	// StepFailed keeps the site down after a new build that raised the minimum version failed its start check.
	StepFailed Step = "failed"
	// StepRolledBack takes back an update, current naming the build before it again and the version after it refused.
	StepRolledBack Step = "rolled-back"
)

// State is what the state file holds of the last update, and the versions that failed their start check.
type State struct {
	Step     Step     `json:"step"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Attempts int      `json:"attempts"`
	LongJump bool     `json:"long_jump"`
	Backup   string   `json:"backup"`
	Refused  []string `json:"refused"`
}

// WithEmptyLists returns the state with an empty list of refused versions in place of none, which JSON shows as [].
func (s State) WithEmptyLists() State {
	if s.Refused == nil {
		s.Refused = []string{}
	}
	return s
}

// StateFile returns the file that holds the state of the last update.
func (l Layout) StateFile() string {
	return filepath.Join(l.Root, stateFolder, stateName)
}

// ReadState returns the state the state file holds, an empty state before the first update.
func (l Layout) ReadState() (State, error) {
	body, err := os.ReadFile(l.StateFile())
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	var state State
	if err == nil {
		err = json.Unmarshal(body, &state)
	}
	if err != nil {
		return State{}, fmt.Errorf("updater: read the update state %s: %w", l.StateFile(), err)
	}
	return state, nil
}

// WriteState writes state into a new file renamed over the state file, so a sudden stop leaves one or the other.
func (l Layout) WriteState(state State) error {
	folder := filepath.Dir(l.StateFile())
	_ = os.MkdirAll(folder, 0o755)
	body, _ := json.MarshalIndent(state, "", "  ")
	temp, err := os.CreateTemp(folder, "."+stateName+"-*")
	if err != nil {
		return fmt.Errorf("updater: write the update state: %w", err)
	}
	err = save(temp, append(body, '\n'))
	if err == nil {
		err = os.Rename(temp.Name(), l.StateFile())
	}
	if err == nil {
		err = syncFolder(folder)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		return fmt.Errorf("updater: write the update state: %w", err)
	}
	return nil
}

// save writes body into file, flushes it to disk and closes it.
func save(file *os.File, body []byte) error {
	_, err := file.Write(body)
	return closed(file, err)
}

// closed flushes file to disk and closes it once err, what writing it ended with, is nil, and only closes it otherwise.
func closed(file *os.File, err error) error {
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
