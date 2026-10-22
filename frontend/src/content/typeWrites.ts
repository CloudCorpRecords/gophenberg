// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query'

import { typesQueryKey } from './nav'
import { holdWrite, writesSettled } from './postWrites'
import { deleteType, updateType } from './types'
import type { ContentType, TypeEdit } from './types'

/**
 * Reads the registry again, for the list, the menu and every screen holding it.
 * @param client - The query client holding the registry.
 * @returns A promise settling once the registry was read again.
 */
export function refreshTypes(client: QueryClient): Promise<void> {
	return client.invalidateQueries({ queryKey: typesQueryKey })
}

/**
 * Returns the name the writes to a type are held under, apart from the writes to any item.
 * @param key - The key of the type.
 * @returns The name.
 */
function slotOf(key: string): string {
	return `content type ${key}`
}

/**
 * Sends a write to a type once every earlier write to it has answered, holding it until it answers in turn.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @param write - Sends the write.
 * @returns The write.
 */
function inTurn<T>(client: QueryClient, key: string, write: () => Promise<T>): Promise<T> {
	const slot = slotOf(key)
	return holdWrite(client, slot, writesSettled(client, slot).then(write))
}

/**
 * Edits a type once every earlier write to it has answered, so the registry never takes two writes of one type at once.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @param edit - The fields to move.
 * @returns The stored type.
 */
export function editInTurn(client: QueryClient, key: string, edit: TypeEdit): Promise<ContentType> {
	return inTurn(client, key, () => updateType(key, edit))
}

/**
 * Removes a type once every earlier write to it has answered.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @returns The removal.
 */
export function deleteInTurn(client: QueryClient, key: string): Promise<void> {
	return inTurn(client, key, () => deleteType(key))
}
