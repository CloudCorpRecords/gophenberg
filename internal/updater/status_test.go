// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// keepFile writes body at the path inside the install root of layout, failing t when it cannot.
func keepFile(t *testing.T, layout updater.Layout, path, body string) {
	t.Helper()
	full := filepath.Join(layout.Root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("making the folder of %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// listsNothing is the status of an install with no install root, its lists empty rather than none, so JSON shows [].
var listsNothing = updater.Status{Update: updater.State{Refused: []string{}}, Backups: []string{}}

func TestReadStatusOfAnInstallWithNoRootListsNothing(t *testing.T) {
	t.Parallel()

	status, err := updater.ReadStatus(settingsOf(t, map[string]string{}))

	if err != nil || !reflect.DeepEqual(status, listsNothing) {
		t.Errorf("ReadStatus() with no install root = %#v, %v, want %#v", status, err, listsNothing)
	}
}

func TestStateWithEmptyListsKeepsTheVersionsItRefuses(t *testing.T) {
	t.Parallel()

	for _, refused := range [][]string{nil, {}, {"0.19.5"}} {
		state := updater.State{Step: updater.StepSwitched, Refused: refused}

		got := state.WithEmptyLists()

		want := updater.State{Step: updater.StepSwitched, Refused: append([]string{}, refused...)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("WithEmptyLists() of %#v = %#v, want %#v", state, got, want)
		}
	}
}

func TestReadStatusNamesTheBuildsTheLastUpdateAndTheBackupsTheNewestFirst(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	keepFile(t, f.layout, "backups/0.19.0-20261001T120000Z.dump", "an older dump")
	keepFile(t, f.layout, longJumpBackup, "a dump")
	keepFile(t, f.layout, "backups/0.20.0-20261021T120000Z.dump.partial", "half a dump")
	keepFile(t, f.layout, "backups/notes.txt", "a person's notes")

	status, err := updater.ReadStatus(settingsOf(t, f.env))

	want := updater.Status{
		Root: f.layout.Root, Current: offered, Previous: running, Update: switchedTo,
		Backups: []string{longJumpBackup, "backups/0.19.0-20261001T120000Z.dump"},
	}
	if err != nil || !reflect.DeepEqual(status, want) {
		t.Errorf("ReadStatus() = %+v, %v, want %+v", status, err, want)
	}
}

func TestReadStatusOfAnInstallRootWithNoLinksNamesNoBuild(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	status, err := updater.ReadStatus(settingsOf(t, map[string]string{"GOPHENBERG_INSTALL_ROOT": root}))

	want := listsNothing
	want.Root = root
	if err != nil || !reflect.DeepEqual(status, want) {
		t.Errorf("ReadStatus() of an empty install root = %#v, %v, want only its root named", status, err)
	}
}

func TestReadStatusShowsTheRestoreStepsOfAnUpdateThatStaysDown(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, longJumped(updater.StepFailed))

	status, err := updater.ReadStatus(settingsOf(t, f.env))

	want := "restore the backup taken before the update, " + filepath.Join(f.layout.Root, longJumpBackup) +
		", into an empty database, point current at versions/0.20.0 and start Gophenberg again, as " +
		"https://docs.gophenberg.org/self-hosting/updates-and-backups/#restoring-on-a-server-install shows, or fix " +
		"what failed and " +
		"start Gophenberg 0.21.0 again"
	if err != nil || status.Restore != want {
		t.Errorf("ReadStatus() = %+v, %v, want the restore steps %q", status, err, want)
	}
}

func TestReadStatusShowsNoRestoreStepsWhileAnUpdateThatRaisedTheMinimumVersionWaits(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, longJumped(updater.StepSwitched))

	status, err := updater.ReadStatus(settingsOf(t, f.env))

	if err != nil || status.Restore != "" {
		t.Errorf("ReadStatus() = %+v, %v, want no restore steps before the new build fails", status, err)
	}
}

func TestReadStatusRefusesAStateFileItCannotRead(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	if err := os.WriteFile(f.layout.StateFile(), []byte("step: switched"), 0o644); err != nil {
		t.Fatalf("writing the state file: %v", err)
	}

	_, err := updater.ReadStatus(settingsOf(t, f.env))

	wantRefused(t, err, "updater: read the update state "+f.layout.StateFile())
}
