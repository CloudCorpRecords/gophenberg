// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// officialAbout is a staged binary that answers about -json as the official build of 0.21.0.
const officialAbout = "#!/bin/sh\nprintf '{\"version\":\"0.21.0\",\"stamp\":\"official\"}'\n"

// heldOpen writes the staged binary and keeps it open for writing until wait has passed.
func heldOpen(t *testing.T, wait time.Duration) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "gophenberg")
	if err := os.WriteFile(binary, []byte(officialAbout), 0o755); err != nil {
		t.Fatalf("writing the staged binary: %v", err)
	}
	writer, err := os.OpenFile(binary, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("opening the staged binary for writing: %v", err)
	}
	time.AfterFunc(wait, func() { _ = writer.Close() })
	t.Cleanup(func() { _ = writer.Close() })
	return binary
}

func TestCheckBuildWaitsForABinaryAnotherProcessStillHoldsOpenForWriting(t *testing.T) {
	t.Parallel()

	binary := heldOpen(t, 200*time.Millisecond)

	if err := checkBuild(t.Context(), binary, "0.21.0"); err != nil {
		t.Errorf("checkBuild() = %v, want the build checked once the writer let go of it", err)
	}
}

func TestCheckBuildStopsWaitingForABinaryNoWriterLetsGoOf(t *testing.T) {
	t.Parallel()

	binary := heldOpen(t, time.Hour)

	err := checkBuild(t.Context(), binary, "0.21.0")

	if err == nil || !strings.Contains(err.Error(), "text file busy") {
		t.Errorf("checkBuild() = %v, want the busy binary refused once the wait is over", err)
	}
}

func TestPutBackKeepsASwitchThatTookEffectBeforeItsSaveFailed(t *testing.T) {
	t.Parallel()

	layout := Layout{Root: t.TempDir()}
	switched := State{Step: StepSwitched, From: "0.20.0", To: "0.21.0"}
	if err := os.MkdirAll(layout.Build("0.21.0"), 0o755); err != nil {
		t.Fatalf("making the build folder: %v", err)
	}
	if err := layout.Point(Current, "0.21.0"); err != nil {
		t.Fatalf("pointing current at the new build: %v", err)
	}
	if err := layout.WriteState(switched); err != nil {
		t.Fatalf("writing the switched state: %v", err)
	}
	r := &run{
		layout: layout, held: State{Step: "committed", From: "0.19.0", To: "0.20.0"},
		chosen: update.Chosen{Found: update.Found{Version: "0.21.0"}},
	}
	saveErr := errors.New("updater: point current at 0.21.0: input/output error")

	err := r.putBack(StepSwitched, saveErr)

	stands := "current names Gophenberg 0.21.0, so restart Gophenberg to start it"
	if !errors.Is(err, saveErr) || !strings.Contains(err.Error(), stands) {
		t.Errorf("putBack() = %v, want it to say the switch stands and to carry %v", err, saveErr)
	}
	if held := (SwitchedError{}); !errors.As(err, &held) || held.Release != "0.21.0" {
		t.Errorf("putBack() = %v, want it marked as a switch that stands for Gophenberg 0.21.0", err)
	}
	if _, statErr := os.Stat(layout.Build("0.21.0")); statErr != nil {
		t.Errorf("the build current names is gone (%v), want it kept", statErr)
	}
	if held, readErr := layout.ReadState(); readErr != nil || !reflect.DeepEqual(held, switched) {
		t.Errorf("the state after putBack() is %+v, %v, want the switched state %+v kept", held, readErr, switched)
	}
}
