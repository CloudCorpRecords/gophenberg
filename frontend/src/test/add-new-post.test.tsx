// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPostWithId } from './postFixture'

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/content/:id', ({ params }) =>
			HttpResponse.json(storedPostWithId(String(params.id))),
		),
	)
})

const NEW_POST_ID = '019fb000-0000-7000-8000-0000000000aa'

/**
 * Serves a created draft and records the request body.
 * @returns The recorded bodies, filled as requests arrive.
 */
function captureCreate(): { bodies: unknown[] } {
	const bodies: unknown[] = []
	server.use(
		http.post('/api/content', async ({ request }) => {
			bodies.push(await request.json())
			return HttpResponse.json(
				{ id: NEW_POST_ID, type: 'post', slug: 'untitled', title: '', status: 'draft' },
				{ status: 201 },
			)
		}),
	)
	return { bodies }
}

/**
 * Returns the Add New the list header holds, apart from the rail's.
 * @returns The header's Add New button.
 */
async function headerAddNew(): Promise<HTMLElement> {
	return within(await screen.findByRole('main')).findByRole('button', { name: 'Add New' })
}

test('the Add New of the list header creates a draft and opens it in the editor', async () => {
	const recorded = captureCreate()
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt('/content/post')

	await userEvent.click(await headerAddNew())

	expect(await screen.findByTitle('Editor canvas')).toBeInTheDocument()
	expect(router.state.location.pathname).toBe(`/content/post/${NEW_POST_ID}/edit`)
	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: {} }])
})

test('the Add New of the list header reports a failure above the list without navigating away', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.post('/api/content', () => HttpResponse.json({ error: 'nope' }, { status: 500 })),
	)
	renderAt('/content/post')

	await userEvent.click(await headerAddNew())

	const notice = await within(screen.getByRole('region', { name: 'Posts' })).findByRole('alert')
	expect(notice).toHaveTextContent('The draft could not be created.')
	expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument()
	expect(await headerAddNew()).toHaveFocus()
})

/** The post type whose rating field names a default of 5. */
const RATED_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [
		{
			key: 'rating',
			label: 'Rating',
			kind: 'number',
			many: false,
			required: false,
			settings: { default: 5 },
			updated_at: '2026-08-01T10:00:00Z',
		},
		{
			key: 'subtitle',
			label: 'Subtitle',
			kind: 'text',
			many: false,
			required: false,
			updated_at: '2026-08-01T10:00:00Z',
		},
	],
}

test('add new starts a draft on the defaults its fields name', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [RATED_TYPE] })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	const recorded = captureCreate()
	renderAt('/content/post')

	await userEvent.click(await headerAddNew())
	await screen.findByTitle('Editor canvas')

	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: { rating: 5 } }])
})

test('keeps Add New off until the type registry answers, then drafts on its defaults', async () => {
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({ items: [RATED_TYPE] })
		}),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	const recorded = captureCreate()
	renderAt('/content/post')
	const button = await headerAddNew()
	expect(button).toHaveAttribute('aria-disabled', 'true')

	await userEvent.click(button)

	expect(recorded.bodies).toEqual([])
	registry.release()
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.click(button)
	await screen.findByTitle('Editor canvas')
	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: { rating: 5 } }])
})
