// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { DataForm, DataViews, filterSortAndPaginate } from '../dataviews'
import type { Field, View } from '../dataviews'

interface Post {
	id: string
	title: string
}

const fields: Field<Post>[] = [{ id: 'title', label: 'Title' }]

const view: View = {
	type: 'table',
	fields: ['title'],
	page: 1,
	perPage: 20,
}

test('re-exports the list view so screens never import the heavy package directly', () => {
	render(
		<DataViews<Post>
			data={[{ id: '1', title: 'Welcome to Gophenberg' }]}
			fields={fields}
			view={view}
			onChangeView={() => {}}
			paginationInfo={{ totalItems: 1, totalPages: 1 }}
			defaultLayouts={{ table: {} }}
		/>,
	)

	expect(screen.getByText('Welcome to Gophenberg')).not.toBeNull()
})

test('re-exports the form view', () => {
	expect(typeof DataForm).toBe('function')
})

test('re-exports the helper that searches, sorts and pages a list held whole', () => {
	const posts = [
		{ id: '1', title: 'Welcome to Gophenberg' },
		{ id: '2', title: 'Notes on the Next Release' },
	]
	const searchable: Field<Post>[] = [{ id: 'title', label: 'Title', enableGlobalSearch: true }]

	const shown = filterSortAndPaginate(posts, { ...view, search: 'notes' }, searchable)

	expect(shown).toEqual({ data: [posts[1]], paginationInfo: { totalItems: 1, totalPages: 1 } })
})
