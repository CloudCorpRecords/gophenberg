// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"errors"
	"testing"
)

func TestMustFunctionsPanicsOnAListDbkitCannotBuild(t *testing.T) {
	t.Parallel()

	failed := errors.New("no function list")
	defer func() {
		if recovered := recover(); recovered != failed {
			t.Fatalf("recovered %v, want the error", recovered)
		}
	}()

	mustFunctions(nil, failed)
}
