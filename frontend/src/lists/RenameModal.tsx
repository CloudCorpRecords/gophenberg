// SPDX-License-Identifier: Apache-2.0

import type { RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RenameBody, useToaster } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

import type { UnsavedText } from './unsavedText'

/** What the rename modal of a list takes beside the props DataViews hands a modal. */
interface RenameModalProps<Item> extends RenderModalProps<Item> {
	/** Returns the name an item carries now. */
	nameOf: (item: Item) => string
	/** Returns the id an item is known by. */
	idOf: (item: Item) => string
	/** Writes the new name of an item. */
	rename: (item: Item, name: string) => Promise<unknown>
	/** Reloads the list once an item took its new name. */
	refresh: (item: Item) => Promise<unknown>
	/** The names failed writes left unsaved, offered again when the modal opens for the same item. */
	unsaved: UnsavedText
}

/**
 * Renders the modal writing a new name for one item, closed the moment Save is pressed, its failure in a toast.
 * @param props - The item, the handler closing the modal, the name and id readers, the write, the refresh and the
 *   unsaved names.
 * @returns The rename form.
 */
export function RenameModal<Item>(props: RenameModalProps<Item>) {
	const { items: [item], closeModal, nameOf, idOf, rename, refresh, unsaved } = props
	const toaster = useToaster()
	const id = idOf(item)
	const save = (name: string) => {
		closeModal?.()
		unsaved.delete(id)
		void rename(item, name).then(
			async () => {
				await refresh(item)
				toaster.show(__('Name updated.', 'gophenberg'))
			},
			(failure: Error) => {
				unsaved.set(id, name)
				toaster.show(failure.message)
			},
		)
	}
	return (
		<RenameBody
			name={unsaved.get(id) ?? nameOf(item)}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={save}
			onCancel={closeModal}
		/>
	)
}
