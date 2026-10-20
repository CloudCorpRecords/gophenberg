// SPDX-License-Identifier: Apache-2.0

import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { createPost } from './api'
import { seededValues } from './fieldDefaults'
import type { ContentType } from './types'

/**
 * Returns the mutation creating a draft of a type on the defaults its fields name, then opening it in the editor.
 * @param listed - The type to create a draft of.
 * @returns The mutation, its failure read as the server's reason or as the draft that could not be created.
 */
export function useAddNew(listed: ContentType) {
	const navigate = useNavigate()
	return useMutation({
		mutationFn: () =>
			createPost(
				listed.key,
				{ title: '', fields: seededValues(listed.fields) },
				__('The draft could not be created.', 'gophenberg'),
			),
		onSuccess: (post) =>
			navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: listed.key, postId: post.id } }),
	})
}
