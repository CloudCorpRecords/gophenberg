// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// switchedFrom is the build an update switched away from in the boot guard tests, to the version of this binary.
const switchedFrom = "0.18.0"

// switchedRoot returns a new install root where an update switched from 0.18.0 to this version, its state edited.
func switchedRoot(t *testing.T, edit func(*updater.State)) string {
	t.Helper()
	root := t.TempDir()
	if err := switchedInstall(root, switchedFrom, version.Version()); err != nil {
		t.Fatalf("making the install root: %v", err)
	}
	if err := editState(root, edit); err != nil {
		t.Fatalf("editing the update state: %v", err)
	}
	return root
}

// triedThreeTimes records that the build the update switched to already tried to start three times.
func triedThreeTimes(state *updater.State) {
	state.Attempts = 3
}

// runServe runs serve through the command line over env and returns what it answered, with the code main exits with.
func runServe(t *testing.T, env map[string]string) testkit.Result {
	t.Helper()
	exits := &exitCodes{}
	result := testkit.Run(t, commandLine(testkit.Getenv(env), noPlugins, exits, version.Stamp()), "", "serve")
	result.Code = exits.of(result.Code)
	return result
}

func TestServeGoesBackBeforeTheDatabaseOpensOnceTheBuildTriedToStartTooOften(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, triedThreeTimes)

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: root,
	})

	want := "gophenberg: updater: Gophenberg " + version.Version() + " failed its start check after the " +
		"update from 0.18.0: it tried to start 4 times and GOPHENBERG_UPDATE_ATTEMPTS allows 3\n"
	if result.Code != exitFailedCheck || !strings.HasPrefix(result.Stderr, want) {
		t.Errorf("serve = %d with stderr %q, want %d and the failed check first", result.Code, result.Stderr,
			exitFailedCheck)
	}
	held, err := os.Readlink(filepath.Join(root, updater.Current))
	if held != filepath.Join("versions", switchedFrom) {
		t.Errorf("the current link names %q (%v), want the build before the update", held, err)
	}
}

func TestServeRefusesAnUpdateStateItCannotRead(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, triedThreeTimes)
	if err := os.WriteFile(updater.Layout{Root: root}.StateFile(), []byte("step: switched"), 0o644); err != nil {
		t.Fatalf("writing the state file: %v", err)
	}

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: root,
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "updater: read the update state") {
		t.Errorf("serve = %d with stderr %q, want 1 and the state file named", result.Code, result.Stderr)
	}
}

func TestServeRefusesAnUpdateSettingOutOfRange(t *testing.T) {
	t.Parallel()

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_UPDATE_ATTEMPTS": "0",
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "GOPHENBERG_UPDATE_ATTEMPTS:") {
		t.Errorf("serve = %d with stderr %q, want 1 and the setting named", result.Code, result.Stderr)
	}
}

func TestServeKeepsAnUpdateWhoseFirstStartAnswersItsCheck(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, func(*updater.State) {})
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t), "GOPHENBERG_ADDR": "127.0.0.1:0", installRootSetting: root,
	}
	ctx, cancel := context.WithTimeout(t.Context(), siteStartLimit)
	defer cancel()
	stderr := &keptWatch{cancel: cancel}

	err := run(ctx, testkit.Getenv(env), stderr, noPlugins)

	if err != nil || ctx.Err() == nil {
		t.Fatalf("run() = %v with stderr %q, want the update kept and then a clean stop", err, stderr)
	}
	state, err := updater.Layout{Root: root}.ReadState()
	want := updater.State{Step: updater.StepCommitted, From: switchedFrom, To: version.Version(), Attempts: 1}
	if err != nil || state.Step != want.Step || state.Attempts != want.Attempts || state.To != want.To {
		t.Errorf("the update state is %+v (%v), want %+v", state, err, want)
	}
}

func TestExitCodesAnswerTheCodeAnErrorAskedForOnlyInPlaceOfAFailure(t *testing.T) {
	t.Parallel()

	asked := &exitCodes{code: exitFailedCheck}
	for given, want := range map[int]int{
		gonsole.ExitDone:    gonsole.ExitDone,
		gonsole.ExitFailed:  exitFailedCheck,
		gonsole.ExitMisused: gonsole.ExitMisused,
	} {
		if got := asked.of(given); got != want {
			t.Errorf("of(%d) with %d kept = %d, want %d", given, exitFailedCheck, got, want)
		}
	}
	if got := (&exitCodes{}).of(gonsole.ExitFailed); got != gonsole.ExitFailed {
		t.Errorf("of(1) with no code kept = %d, want 1", got)
	}
}

func TestMainAnswersTheExitCodeTheBootGuardAsksFor(t *testing.T) {
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"gophenberg", "serve"}
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: switchedRoot(t, triedThreeTimes),
	}

	if code := Main(testkit.Getenv(env), noPlugins); code != exitFailedCheck {
		t.Errorf("Main() = %d, want %d once the boot guard went back", code, exitFailedCheck)
	}
}
