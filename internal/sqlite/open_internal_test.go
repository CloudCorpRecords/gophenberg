// SPDX-License-Identifier: Apache-2.0

package sqlite

import "testing"

func TestDefaultMaxConnsKeepsAFloorOfFour(t *testing.T) {
	t.Parallel()

	for cpus, want := range map[int]int{1: 4, 4: 4, 16: 16} {
		if got := defaultMaxConns(cpus); got != want {
			t.Errorf("defaultMaxConns(%d) = %d, want %d", cpus, got, want)
		}
	}
}
