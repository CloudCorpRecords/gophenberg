// SPDX-License-Identifier: Apache-2.0

package mediahost

import "context"

// slots bounds how many images decode and resize at once.
type slots chan struct{}

// take waits for a free slot while ctx lives, answering the context's error once ctx has ended.
func (s slots) take(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// give hands a taken slot back.
func (s slots) give() {
	<-s
}
