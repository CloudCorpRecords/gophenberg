// SPDX-License-Identifier: Apache-2.0

import type { Politeness } from '@gopherium/godmin/testing'
import { onTestFinished } from 'vitest'

/**
 * Adds every message the given changes wrote into a live region, without the space a repeated message carries.
 * @param heard - The messages heard so far.
 * @param records - The changes the region went through.
 */
function keep(heard: string[], records: MutationRecord[]) {
	for (const record of records) {
		for (const node of record.addedNodes) {
			heard.push((node.textContent ?? '').trim())
		}
	}
}

/**
 * Listens to the live region of an urgency until the test ends.
 * @param politeness - The urgency to listen at.
 * @returns The reader of every message the region took so far, in order, held by this test alone.
 */
export function listenAt(politeness: Politeness): () => string[] {
	const heard: string[] = []
	const observer = new MutationObserver((records) => keep(heard, records))
	observer.observe(document.getElementById(`a11y-speak-${politeness}`) as HTMLElement, { childList: true })
	onTestFinished(() => observer.disconnect())
	return () => {
		keep(heard, observer.takeRecords())
		return heard
	}
}
