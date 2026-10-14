// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { renderAt, renderRoutedAt } from './render'

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/media', () => HttpResponse.json({ items: [], total: 0, per_page: 20 })),
	)
})

test('holds the navigation behind a menu button on a narrow viewport', async () => {
	setViewport({ matches: true })

	renderAt('/')

	expect(await screen.findByRole('button', { name: 'Open navigation' })).toBeInTheDocument()
	expect(screen.queryByRole('navigation', { name: 'Navigation' })).toBeNull()
})

test('opens the navigation in a drawer', async () => {
	setViewport({ matches: true })
	renderAt('/')

	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation' }))

	const drawer = await screen.findByRole('dialog')
	expect(within(drawer).getByRole('navigation', { name: 'Navigation' })).toBeInTheDocument()
	expect(await within(drawer).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
})

test('puts the navigation away once it takes the reader somewhere', async () => {
	setViewport({ matches: true })
	renderAt('/media')
	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation' }))
	const drawer = await screen.findByRole('dialog')

	await userEvent.click(await within(drawer).findByRole('link', { name: 'Posts' }))

	expect(await screen.findByRole('button', { name: 'Open navigation' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test('keeps the navigation open when its row names the list the root already opened', async () => {
	setViewport({ matches: true })
	const { router } = renderRoutedAt('/')
	await screen.findByRole('heading', { name: 'Posts', level: 1 })
	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation' }))
	const drawer = await screen.findByRole('dialog')

	await userEvent.click(await within(drawer).findByRole('link', { name: 'Posts' }))
	await waitFor(() => expect(router.state.status).toBe('idle'))

	expect(router.state.location.pathname).toBe('/content/post')
	expect(screen.getByRole('dialog')).toBe(drawer)
})

test('keeps the rail beside the canvas when there is room', async () => {
	setViewport({ matches: false })

	renderAt('/')

	expect(await screen.findByRole('navigation', { name: 'Navigation' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Open navigation' })).toBeNull()
})
