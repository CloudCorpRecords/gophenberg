// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { render, screen, within } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { ComponentType } from 'react'

import { typesQueryKey } from '../content/nav'
import { coreNav } from '../menu/coreNav'
import { plugins } from '../plugins'
import { createAppRouter } from '../router'
import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

/** The built-in post type, the default one unless a test hands the root to another. */
const builtInType = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: 'Manage the posts on this site.',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

/** A second type a test may make the default. */
const pageType = { ...builtInType, key: 'page', singular_label: 'Page', plural_label: 'Pages', route_word: 'pages' }

beforeEach(() => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
})

test('mounts the application under the admin basepath', () => {
	expect(createAppRouter().options.basepath).toBe('/admin')
})

test('opens the list of the default content type at the root path', async () => {
	const { router } = renderRoutedAt('/')

	expect(await screen.findByRole('heading', { name: 'Posts', level: 1 })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/content/post')
})

test('opens whichever type is the default, not the first one listed', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...builtInType, default: false }, { ...pageType, default: true }] }),
		),
	)
	const { router } = renderRoutedAt('/')

	expect(await screen.findByRole('heading', { name: 'Pages', level: 1 })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/content/page')
})

test('leaves the root out of the history, so going back never lands on it', async () => {
	const { router } = renderRoutedAt('/')

	await screen.findByRole('heading', { name: 'Posts', level: 1 })

	expect(router.history.length).toBe(1)
})

test('holds the loading ghost at the root until the type registry answers', async () => {
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({ items: [builtInType] })
		}),
	)
	const { router } = renderRoutedAt('/')

	expect(await screen.findByRole('status')).toHaveTextContent('Loading the screen.')
	registry.release()
	await screen.findByRole('heading', { name: 'Posts', level: 1 })
	expect(router.state.location.pathname).toBe('/content/post')
})

test('reports a type registry it could not read in place of the list the root opens', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/')

	expect(await screen.findByRole('alert')).toHaveTextContent('The content types could not be loaded.')
})

test('keeps one first level heading over a type registry the root could not read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/')

	await screen.findByRole('alert')
	const main = screen.getByRole('main')
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('Content')
})

test('opens the default list from the types it holds even when their last refresh failed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const { client, router } = renderRoutedAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })
	server.use(http.get('/api/types', () => HttpResponse.json({}, { status: 500 })))
	await client.refetchQueries({ queryKey: typesQueryKey })
	expect(client.getQueryState(typesQueryKey)?.status).toBe('error')

	await router.navigate({ to: '/' })

	expect(await screen.findByRole('heading', { name: 'Posts', level: 1 })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/content/post')
})

test('leaves the heading rank to the screen, not the masthead', async () => {
	renderAt('/')

	expect(await screen.findByRole('link', { name: 'Gophenberg' })).toBeInTheDocument()
	expect(screen.queryByRole('heading', { name: 'Gophenberg' })).toBeNull()
})

test('renders a navigation entry for every active type, core and plugin section', async () => {
	renderAt('/')

	const nav = await screen.findByRole('navigation', { name: 'Navigation' })
	await within(nav).findByRole('link', { name: 'Posts' })
	expect(within(nav).queryAllByRole('link')).toHaveLength(
		1 + coreNav.length + plugins.flatMap((plugin) => plugin.nav).length,
	)
})

test('offers the field groups a way in from the menu', async () => {
	renderAt('/')

	const nav = await screen.findByRole('navigation', { name: 'Navigation' })

	expect(within(nav).getByRole('link', { name: 'Field Groups' })).toHaveAttribute(
		'href',
		'/admin/field-groups',
	)
})

test('frames the active route inside the main content region', async () => {
	renderAt('/')

	const main = await screen.findByRole('main')
	expect(await within(main).findByRole('heading', { name: 'Posts', level: 1 })).toBeInTheDocument()
	expect(within(main).queryByRole('navigation', { name: 'Navigation' })).toBeNull()
})

test('holds a loading ghost for routes whose chunk is still arriving', () => {
	const Pending = createAppRouter().options.defaultPendingComponent as ComponentType
	render(<Pending />)

	const status = screen.getByRole('status')
	expect(status).toHaveTextContent('Loading the screen.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})

test('holds the previous screen briefly and never holds the ghost', () => {
	const options = createAppRouter().options

	expect(options.defaultPendingMs).toBe(200)
	expect(options.defaultPendingMinMs).toBe(0)
})
