// SPDX-License-Identifier: Apache-2.0

import { InitialsAvatar } from '@gopherium/godmin'
import { render } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'

/** One name the fixture pins with the circle colour, the letter colour and the letter its avatar draws. */
interface AvatarRow {
	name: string
	circle: string
	letter_colour: string
	letter: string
}

/** The avatars internal/accountbridge/testdata/initials.json pins for the server and the admin alike. */
const FIXTURE = JSON.parse(
	readFileSync(join(repositoryRoot(), 'internal', 'accountbridge', 'testdata', 'initials.json'), 'utf8'),
) as { avatars: AvatarRow[] }

/**
 * Returns a colour the browser reports as rgb() in the #rrggbb form the fixture writes.
 * @param rgb - The colour as the browser reports it.
 * @returns The colour as #rrggbb.
 */
function hexOf(rgb: string): string {
	const channels = rgb.match(/\d+/g) ?? []
	return `#${channels.map((channel) => Number(channel).toString(16).padStart(2, '0')).join('')}`
}

test('pins at least one name', () => {
	expect(FIXTURE.avatars.length).toBeGreaterThan(0)
})

test.each(FIXTURE.avatars)('draws $name as the letter $letter in $letter_colour on $circle', (row) => {
	const { container } = render(<InitialsAvatar name={row.name} />)
	const avatar = container.querySelector('.godmin-avatar') as HTMLElement

	expect([row.name, hexOf(avatar.style.backgroundColor), hexOf(avatar.style.color), avatar.textContent]).toEqual([
		row.name,
		row.circle,
		row.letter_colour,
		row.letter,
	])
})
