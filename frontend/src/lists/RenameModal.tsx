// SPDX-License-Identifier: Apache-2.0

import type { RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RenameBody, useToaster } from '@gopherium/godmin'
import { useMutation } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'

/** What the rename modal of a list takes beside the props DataViews hands a modal. */
interface RenameModalProps<Item> extends RenderModalProps<Item> {
	/** Returns the name an item carries now. */
	nameOf: (item: Item) => string
	/** Writes the new name of an item. */
	rename: (item: Item, name: string) => Promise<unknown>
	/** Reloads the list once an item took its new name. */
	refresh: (item: Item) => Promise<unknown>
}

/**
 * Renders the modal writing a new name for one item, its failure kept inside under the field.
 * @param props - The item, the handler closing the modal, the name reader, the write and the refresh.
 * @returns The rename form.
 */
export function RenameModal<Item>({ items: [item], closeModal, nameOf, rename, refresh }: RenameModalProps<Item>) {
	const toaster = useToaster()
	const write = useMutation({
		mutationFn: (name: string) => rename(item, name),
		onSuccess: async () => {
			await refresh(item)
			toaster.show(__('Name updated.', 'gophenberg'))
			closeModal?.()
		},
	})
	return (
		<RenameBody
			name={nameOf(item)}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Rename', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			busy={write.isPending}
			failure={write.error?.message}
			onSubmit={(name) => write.mutate(name)}
			onCancel={write.isPending ? undefined : closeModal}
		/>
	)
}
