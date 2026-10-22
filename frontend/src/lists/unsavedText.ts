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

/** The latest write each store sent, by the id of the item it went to. */
const latestWrites = new WeakMap<UnsavedText, Map<string, symbol>>()

/**
 * Marks a write as the latest one sent for an item, the only one that may keep its text.
 * @param unsaved - The store the write keeps its text in.
 * @param id - The item the write goes to.
 * @returns The check telling whether no later write went to the item since.
 */
export function markLatest(unsaved: UnsavedText, id: string): () => boolean {
	const writes = latestWrites.get(unsaved) ?? new Map<string, symbol>()
	latestWrites.set(unsaved, writes)
	const write = Symbol(id)
	writes.set(id, write)
	return () => writes.get(id) === write
}
