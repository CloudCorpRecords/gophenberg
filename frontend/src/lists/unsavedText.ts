// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'

/** The text a failed write left unsaved, by the id of the item it was typed for. */
export type UnsavedText = Map<string, string>

/**
 * Returns the text failed writes left unsaved, the same store for as long as the screen stays.
 * @returns The store a row form takes its first value from when it opens again.
 */
export function useUnsavedText(): UnsavedText {
	const [held] = useState<UnsavedText>(() => new Map())
	return held
}
