// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeEach(() => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
})

/**
 * Returns the rail once the posts list has drawn its heading.
 * @returns The navigation landmark of the rail.
 */
async function railBesidePosts(): Promise<HTMLElement> {
	await screen.findByRole('heading', { name: 'Posts', level: 1 })
	return screen.getByRole('navigation', { name: 'Navigation' })
}

test('the main menu offers the posts section', async () => {
	renderAt('/content/post')

	expect(await within(await railBesidePosts()).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
})

test('keeps the main menu in the rail while the posts list shows', async () => {
	renderAt('/content/post')

	const rail = await railBesidePosts()

	expect(await within(rail).findByRole('link', { name: 'Users' })).toBeInTheDocument()
	expect(within(rail).queryByRole('link', { name: 'Back' })).not.toBeInTheDocument()
})

test('marks the posts row as the current page while its list shows', async () => {
	renderAt('/content/post')

	const row = await within(await railBesidePosts()).findByRole('link', { name: 'Posts' })

	expect(row).toHaveAttribute('aria-current', 'page')
})

test('adds from the list header alone, the rail offering no row of its own', async () => {
	renderAt('/content/post')

	const rail = await railBesidePosts()
	await within(rail).findByRole('link', { name: 'Posts' })

	expect(within(rail).queryByRole('link', { name: 'All Posts' })).not.toBeInTheDocument()
	expect(within(rail).queryByRole('button', { name: 'Add Post' })).not.toBeInTheDocument()
	expect(within(screen.getByRole('main')).getByRole('button', { name: 'Add Post' })).toBeInTheDocument()
})
