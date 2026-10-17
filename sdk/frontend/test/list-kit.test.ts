// SPDX-License-Identifier: Apache-2.0

import * as godmin from '@gopherium/godmin'
import { expect, test } from 'vitest'

import { RunConfirm, useBulkRun, useListRun } from '../index'
import type { BulkRun, BulkWords, ListRun, RunConfirmProps } from '../index'

/** The words a plugin's bulk run reports its outcome in, typed through the SDK. */
const WORDS: BulkWords<{ id: string }> = {
	done: (count) => `Done: ${count}.`,
	failed: (failures, asked) => `Failed: ${failures.length} of ${asked}.`,
}

test('hands plugins the list kit the admin runs, the very functions godmin ships', () => {
	expect([RunConfirm, useBulkRun, useListRun]).toEqual([godmin.RunConfirm, godmin.useBulkRun, godmin.useListRun])
})

test('hands plugins the types the list kit is written in', () => {
	const props: Pick<RunConfirmProps<{ id: string }>, 'confirmLabel' | 'cancelLabel'> = {
		confirmLabel: 'Trash',
		cancelLabel: 'Cancel',
	}
	const run: BulkRun<{ id: string }> | undefined = undefined
	const list: ListRun | undefined = undefined

	expect([props.confirmLabel, run, list, WORDS.done(2, undefined)]).toEqual(['Trash', undefined, undefined, 'Done: 2.'])
})
