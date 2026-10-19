// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk'
import { useToaster } from '@gopherium/godmin'
import { ConfirmDialog } from '@gopherium/godmin/dialogs'
import { __, sprintf } from '@wordpress/i18n'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { useRefresh } from './actions'
import { trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash, and the question it asks first.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control, and its confirm while it asks.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [asking, setAsking] = useState(false)
	const label = __('Move to trash', 'gophenberg')
	const named = toaster.name(title === '' ? __('(no title)', 'gophenberg') : title)
	const confirm = async () => {
		try {
			await trashPost(postId)
		} catch (failure) {
			return (failure as Error).message
		}
		await navigate({ to: '/content/$typeKey' })
		await refresh([postId])
		toaster.show(sprintf(__('"%s" moved to the trash.', 'gophenberg'), named))
	}
	return (
		<>
			<Button variant="outline" onClick={() => setAsking(true)}>{label}</Button>
			{asking ? (
				<ConfirmDialog
					size="small"
					confirmLabel={label}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={confirm}
					onClose={() => setAsking(false)}
				>
					{sprintf(__('Are you sure you want to move "%s" to the trash?', 'gophenberg'), named)}
				</ConfirmDialog>
			) : null}
		</>
	)
}
