// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query'

/** The writes each query client still waits on, by the item they write to. */
const inFlight = new WeakMap<QueryClient, Map<string, Promise<unknown>>>()

/**
 * Returns the writes a query client still waits on.
 * @param client - The query client the writes belong to.
 * @returns The writes by item.
 */
function writesOf(client: QueryClient): Map<string, Promise<unknown>> {
	const known = inFlight.get(client)
	if (known !== undefined) {
		return known
	}
	const fresh = new Map<string, Promise<unknown>>()
	inFlight.set(client, fresh)
	return fresh
}

/**
 * Holds a write to an item until it answers, so a later read or write of that item waits for it.
 * @param client - The query client the write belongs to.
 * @param itemId - The item the write goes to.
 * @param write - The write in flight.
 * @returns The same write.
 */
export function holdWrite<T>(client: QueryClient, itemId: string, write: Promise<T>): Promise<T> {
	const writes = writesOf(client)
	const entry = Promise.allSettled([writes.get(itemId), write])
	writes.set(itemId, entry)
	void entry.then(() => {
		if (writes.get(itemId) === entry) {
			writes.delete(itemId)
		}
	})
	return write
}

/**
 * Returns the writes still held for an item, or nothing when none is in flight.
 * @param client - The query client the writes belong to.
 * @param itemId - The item to look at.
 * @returns A promise settling once they have answered, or nothing.
 */
export function heldWrites(client: QueryClient, itemId: string): Promise<unknown> | undefined {
	return writesOf(client).get(itemId)
}

/**
 * Returns a promise settling once every write held for an item has answered.
 * @param client - The query client the writes belong to.
 * @param itemId - The item to wait for.
 * @returns The promise.
 */
export function writesSettled(client: QueryClient, itemId: string): Promise<unknown> {
	return heldWrites(client, itemId) ?? Promise.resolve()
}
