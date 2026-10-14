// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { declarations } from '../../test/stylesheet'

/** The token the phone drawer paints its surface with. */
const DRAWER_SURFACE = /^var\(--wpds-color-background-surface-neutral-strong\)$/

test('sets a menu row as the Site Editor sets a navigation item, a 13px label on a 40px row', () => {
	expect(declarations('.gophenberg-menu__item')).toMatchObject({
		'box-sizing': 'border-box',
		'min-height': '40px',
		padding: '8px 6px 8px 16px',
		'font-size': expect.stringMatching(/^var\(--wpds-typography-font-size-md\)$/),
		color: expect.stringMatching(/^var\(--wpds-color-foreground-interactive-neutral-weak\)$/),
	})
})

test('sets the current row in the emphasis weight on the interactive fill', () => {
	expect(declarations(".gophenberg-menu__item[data-status='active']")).toMatchObject({
		'font-weight': expect.stringMatching(/^var\(--wpds-typography-font-weight-emphasis\)$/),
		background: expect.stringMatching(/^var\(--wpds-color-background-interactive-neutral-weak-active\)$/),
		color: expect.stringMatching(/^var\(--wpds-color-foreground-interactive-neutral-weak-active\)$/),
	})
})

test('fills the current row apart from the drawer surface it sits on', () => {
	const fill = declarations(".gophenberg-menu__item[data-status='active']").background

	expect(fill).toMatch(/^var\(--wpds-/)
	expect(fill).not.toMatch(DRAWER_SURFACE)
})

test('changes only the text colour of a row under the pointer', () => {
	expect(declarations('.gophenberg-menu__item:hover')).toEqual({
		color: expect.stringMatching(/^var\(--wpds-color-foreground-interactive-neutral-weak-active\)$/),
	})
})
