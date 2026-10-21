// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestHeldRefusesAFolderTheFileSystemWillNotLockAndLetsItGo(t *testing.T) {
	t.Parallel()

	folder, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatalf("opening the state folder: %v", err)
	}

	unlock, err := held(folder, func(int, int) error { return syscall.ENOLCK })

	named := err != nil && strings.HasPrefix(err.Error(), "updater: lock the update state: ")
	if unlock != nil || !errors.Is(err, syscall.ENOLCK) || !named {
		t.Errorf("held() = %v, want the lock the file system refused named, with nothing to let go", err)
	}
	if err := folder.Close(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("closing the folder again = %v, want it closed already when the lock failed", err)
	}
}

func TestHeldLocksTheFolderAndLetsItGo(t *testing.T) {
	t.Parallel()

	folder, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatalf("opening the state folder: %v", err)
	}

	unlock, err := held(folder, syscall.Flock)

	if err != nil {
		t.Fatalf("held() = %v, want the folder locked", err)
	}
	unlock()
	if err := folder.Close(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("closing the folder again = %v, want it closed by what let go of the lock", err)
	}
}
