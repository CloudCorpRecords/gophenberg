// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// switchedState is the state of an update that switched from 0.20.0 to 0.21.0 and started twice.
var switchedState = updater.State{
	Step: updater.StepSwitched, From: "0.20.0", To: "0.21.0", Attempts: 2, LongJump: true,
	Backup: "backups/0.20.0-20261020T120000Z.dump", Refused: []string{"0.20.5"},
}

// writeState writes state into the install root of layout, failing t when it cannot.
func writeState(t *testing.T, layout updater.Layout, state updater.State) {
	t.Helper()
	if err := layout.WriteState(state); err != nil {
		t.Fatalf("WriteState() = %v, want nil", err)
	}
}

// readState returns the state the install root of layout holds, failing t when it cannot be read.
func readState(t *testing.T, layout updater.Layout) updater.State {
	t.Helper()
	state, err := layout.ReadState()
	if err != nil {
		t.Fatalf("ReadState() = %v, want nil", err)
	}
	return state
}

func TestReadStateIsEmptyBeforeTheFirstUpdate(t *testing.T) {
	t.Parallel()

	state := readState(t, updater.Layout{Root: t.TempDir()})

	if !reflect.DeepEqual(state, updater.State{}) {
		t.Errorf("ReadState() of a new install root = %+v, want an empty state", state)
	}
}

func TestWriteStateKeepsEveryFieldUnderItsFrozenName(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}

	writeState(t, layout, switchedState)

	body, err := os.ReadFile(layout.StateFile())
	if err != nil {
		t.Fatalf("reading the state file: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("reading the state file as JSON: %v", err)
	}
	want := map[string]any{
		"step": "switched", "from": "0.20.0", "to": "0.21.0", "attempts": 2.0, "long_jump": true,
		"backup": "backups/0.20.0-20261020T120000Z.dump", "refused": []any{"0.20.5"},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("the state file holds %v, want %v", fields, want)
	}
	if read := readState(t, layout); !reflect.DeepEqual(read, switchedState) {
		t.Errorf("ReadState() = %+v, want the state written %+v", read, switchedState)
	}
}

func TestWriteStateLeavesOnlyTheStateFile(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}

	writeState(t, layout, updater.State{Step: updater.StepDownload, From: "0.20.0", To: "0.21.0"})
	writeState(t, layout, switchedState)

	entries, err := os.ReadDir(filepath.Dir(layout.StateFile()))
	if err != nil {
		t.Fatalf("listing the state folder: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"update.json"}) {
		t.Errorf("the state folder holds %v, want only update.json", names)
	}
}

func TestReadStateRefusesAFileThatIsNotJSON(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	writeState(t, layout, switchedState)
	if err := os.WriteFile(layout.StateFile(), []byte("step: switched"), 0o644); err != nil {
		t.Fatalf("writing the state file: %v", err)
	}

	_, err := layout.ReadState()

	wantRefused(t, err, "updater: read the update state "+layout.StateFile())
}

func TestWriteStateRefusesAStateFolderItCannotWrite(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	writeState(t, layout, switchedState)
	denied(t, filepath.Dir(layout.StateFile()), 0o500)

	err := layout.WriteState(updater.State{})

	wantRefused(t, err, "updater: write the update state")
	if read := readState(t, layout); !reflect.DeepEqual(read, switchedState) {
		t.Errorf("ReadState() after a refused write = %+v, want the state before it", read)
	}
}

func TestWriteStateRefusesAStateFileItCannotReplace(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(layout.StateFile(), "kept"), 0o755); err != nil {
		t.Fatalf("making a folder called update.json: %v", err)
	}

	err := layout.WriteState(switchedState)

	wantRefused(t, err, "updater: write the update state")
	entries, _ := os.ReadDir(filepath.Dir(layout.StateFile()))
	if len(entries) != 1 {
		t.Errorf("the state folder holds %d entries after a refused write, want only update.json", len(entries))
	}
}

func TestWriteStateRefusesAStateFolderItCannotSaveToDisk(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	writeState(t, layout, updater.State{})
	denied(t, filepath.Dir(layout.StateFile()), 0o300)

	err := layout.WriteState(switchedState)

	wantRefused(t, err, "updater: write the update state")
}
