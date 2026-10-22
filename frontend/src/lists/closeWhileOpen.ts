// SPDX-License-Identifier: Apache-2.0

import { useEffect, useRef } from 'react'

/**
 * Returns the close of a modal body that acts only while that body is on screen, never on a modal opened since.
 * @param close - Closes the modal.
 * @returns The close, a no-op once the body left the screen.
 */
export function useCloseWhileOpen(close: (() => void) | undefined): () => void {
	const open = useRef(true)
	useEffect(() => {
		open.current = true
		return () => {
			open.current = false
		}
	}, [])
	return () => {
		if (open.current) {
			close?.()
		}
	}
}
