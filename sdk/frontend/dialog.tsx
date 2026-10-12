// SPDX-License-Identifier: Apache-2.0

import { Dialog as UiDialog, getWpCompatOverlaySlot } from '@wordpress/ui'
import type { ComponentProps } from 'react'

/**
 * Renders the dialog popup inside the overlay slot the selects it holds portal into.
 * @param props - The popup props, a portal named here rendering the popup instead.
 * @returns The popup.
 */
function Popup(props: ComponentProps<typeof UiDialog.Popup>) {
	return <UiDialog.Popup portal={<UiDialog.Portal container={getWpCompatOverlaySlot()} />} {...props} />
}

/** The dialog parts, its popup rendered where the selects it holds portal. */
export const Dialog = { ...UiDialog, Popup }
