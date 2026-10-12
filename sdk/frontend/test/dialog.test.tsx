// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { getWpCompatOverlaySlot, useEnableWpCompatOverlaySlot } from '@wordpress/ui'
import type { ReactNode } from 'react'
import { expect, test } from 'vitest'

import { Dialog, SelectControl } from '../index'

/** The roles the select inside the dialog offers. */
const roles = [
	{ value: 'editor', label: 'Editor' },
	{ value: 'author', label: 'Author' },
]

/**
 * Renders its children under the overlay slot the admin turns on for the whole app.
 * @param props - The elements to render.
 * @returns The children.
 */
function SlotHost({ children }: { children?: ReactNode }) {
	useEnableWpCompatOverlaySlot()
	return <>{children}</>
}

test('renders a dialog inside the overlay slot the selects it holds portal into', async () => {
	const view = render(<SlotHost />)
	const slot = getWpCompatOverlaySlot()

	view.rerender(
		<SlotHost>
			<Dialog.Root open>
				<Dialog.Popup>
					<Dialog.Title>Change role</Dialog.Title>
					<SelectControl label="Role" items={roles} />
				</Dialog.Popup>
			</Dialog.Root>
		</SlotHost>,
	)

	const dialog = await screen.findByRole('dialog', { name: 'Change role' })
	expect(slot).toBeInstanceOf(HTMLDivElement)
	expect(slot?.contains(dialog)).toBe(true)
})
