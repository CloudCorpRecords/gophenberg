// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
)

// startLimitHit is the service result the service manager reports once the starts of a service hit their limit.
const startLimitHit = "start-limit-hit"

// takenBackSteps are the steps of an update that switched builds, the ones update:rollback takes back.
var takenBackSteps = []Step{StepSwitched, StepCommitted, StepFailed}

// failedSteps are the steps of an update whose new build may have failed to start, the ones a failed start takes back.
var failedSteps = []Step{StepSwitched, StepFailed}

// Failure is what the service manager says about the start that failed, as it hands it to its rollback unit.
type Failure struct {
	ServiceResult string
	ExitStatus    string
}

// failedStart reports whether the service manager says a start failed its start check or the starts hit the limit.
func (f Failure) failedStart() bool {
	return f.ServiceResult == startLimitHit || f.ExitStatus == strconv.Itoa(ExitFailedCheck)
}

// Rollback takes back the last update that switched builds, pointing current at the build before it.
func Rollback(settings Settings) (State, error) {
	layout, state, err := lastUpdate(settings)
	if err != nil {
		return State{}, err
	}
	if !slices.Contains(takenBackSteps, state.Step) {
		return State{}, nothingToTakeBack("update:rollback", state)
	}
	return layout.takeBack(state)
}

// RollbackAfterFailure takes back an update whose new build failed to start, once, as the rollback unit runs it.
func RollbackAfterFailure(settings Settings, failure Failure) (State, error) {
	layout, state, err := lastUpdate(settings)
	if err != nil {
		return State{}, err
	}
	if !slices.Contains(failedSteps, state.Step) {
		return State{}, nothingToTakeBack("update:rollback -after-failure", state)
	}
	if !failure.failedStart() {
		return State{}, fmt.Errorf("updater: the service manager says the service result is %q and the exit "+
			"status is %q, not a start that failed its check or hit the start limit, so update:rollback "+
			"-after-failure leaves the update alone", failure.ServiceResult, failure.ExitStatus)
	}
	if mayHaveMigrated(state) {
		return State{}, layout.stayDown(state)
	}
	return layout.takeBack(state)
}

// mayHaveMigrated reports whether the build an update switched to raised the minimum version and has started once.
func mayHaveMigrated(state State) bool {
	return state.LongJump && state.Attempts > 0
}

// lastUpdate returns the install root the settings name and the state of its last update, refusing an empty root.
func lastUpdate(settings Settings) (Layout, State, error) {
	if settings.Root == "" {
		return Layout{}, State{}, fmt.Errorf("updater: %s is empty, so this install keeps no build to go back to, "+
			"go back by hand as %s shows", settings.names.Key("INSTALL_ROOT"), manualSteps)
	}
	layout := Layout{Root: settings.Root}
	state, err := layout.ReadState()
	return layout, state, err
}

// nothingToTakeBack refuses the rollback command when the state names no update it takes back, saying where it stands.
func nothingToTakeBack(command string, state State) error {
	if state.Step == "" {
		return fmt.Errorf("updater: no update ran on this install root, so %s has nothing to take back", command)
	}
	return fmt.Errorf("updater: the last update, from %s to %s, stands at the step %s, so %s has nothing to take back",
		state.From, state.To, state.Step, command)
}

// takeBack points current at the build before the update and writes the update taken back, refusing one that migrated.
func (l Layout) takeBack(state State) (State, error) {
	if mayHaveMigrated(state) {
		return State{}, errors.Join(fmt.Errorf("updater: %s and update:rollback cannot go back to it",
			raisedPast(state)), errors.New("updater: "+l.restoreText(state)))
	}
	if err := l.Point(Current, state.From); err != nil {
		return State{}, err
	}
	state.Step, state.Refused = StepRolledBack, refusing(state.Refused, state.To)
	if err := l.WriteState(state); err != nil {
		return State{}, err
	}
	return state, nil
}
