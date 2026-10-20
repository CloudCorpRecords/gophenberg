// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/updater"
)

const (
	// exitRestart is the exit code of a command that switched builds and asks for a restart to start the new one.
	exitRestart = 75
	// exitFailedCheck is the exit code of a start whose build failed its start check after an update.
	exitFailedCheck = updater.ExitFailedCheck
)

// exitCodes keeps the exit code the error of a command asks for in place of the code of a failed command.
type exitCodes struct {
	code int
}

// restartError asks for a restart once a command pointed current at another build, which the restart starts.
type restartError struct {
	release string
}

// Error says which version the restart starts.
func (e restartError) Error() string {
	return "restart Gophenberg to start " + e.release
}

// keep returns run, keeping the exit code the error it returns asks for.
func (c *exitCodes) keep(run func(context.Context, gonsole.Call) error) func(context.Context, gonsole.Call) error {
	return func(ctx context.Context, call gonsole.Call) error {
		err := run(ctx, call)
		c.code = exitCodeOf(err)
		return err
	}
}

// of returns the exit code kept in place of code once the run failed, and code itself otherwise.
func (c *exitCodes) of(code int) int {
	if code != gonsole.ExitFailed || c.code == 0 {
		return code
	}
	return c.code
}

// exitCodeOf returns the exit code err asks for in place of the code of a failed command, zero for none.
func exitCodeOf(err error) int {
	switch {
	case errors.As(err, &updater.CheckError{}):
		return exitFailedCheck
	case errors.As(err, &restartError{}):
		return exitRestart
	}
	return 0
}
