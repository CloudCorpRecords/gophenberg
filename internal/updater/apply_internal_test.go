// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

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

	err := r.putBack(saveErr)

	stands := "current names Gophenberg 0.21.0, so restart Gophenberg to start it"
	if !errors.Is(err, saveErr) || !strings.Contains(err.Error(), stands) {
		t.Errorf("putBack() = %v, want it to say the switch stands and to carry %v", err, saveErr)
	}
	if _, statErr := os.Stat(layout.Build("0.21.0")); statErr != nil {
		t.Errorf("the build current names is gone (%v), want it kept", statErr)
	}
	if held, readErr := layout.ReadState(); readErr != nil || !reflect.DeepEqual(held, switched) {
		t.Errorf("the state after putBack() is %+v, %v, want the switched state %+v kept", held, readErr, switched)
	}
}
