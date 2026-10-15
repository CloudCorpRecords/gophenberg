// SPDX-License-Identifier: Apache-2.0

// Package dbconns reads the connection cap of the site's database handle, the same setting on either engine.
package dbconns

import (
	"fmt"
	"math"
	"runtime"
	"strconv"

	"github.com/gopherium/framework/gonsole"
)

// MaxConnsFrom reads DATABASE_MAX_CONNS, the site's connection cap on either engine, from 2 to 2147483647.
func MaxConnsFrom(env gonsole.Env) (int, error) {
	conns, err := env.Count("DATABASE_MAX_CONNS", defaultMaxConns(runtime.NumCPU()), gonsole.AtMost(math.MaxInt32))
	if err != nil {
		return 0, err
	}
	if conns < 2 {
		return 0, fmt.Errorf("%s: must be 2 or more, got %q", env.Key("DATABASE_MAX_CONNS"), strconv.Itoa(conns))
	}
	return conns, nil
}

// defaultMaxConns returns the connection cap a machine with cpus processors gets, the larger of 4 and cpus.
func defaultMaxConns(cpus int) int {
	return max(4, cpus)
}
