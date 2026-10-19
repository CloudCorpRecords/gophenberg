// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('offers a document tab and a block tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('tab', { name: 'Document' })).toBeInTheDocument()
	expect(screen.getByRole('tab', { name: 'Block' })).toBeInTheDocument()
})

test('opens on the document tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('tab', { name: 'Document' })).toHaveAttribute(
		'aria-selected',
		'true',
	)
})

test('tells the document tab what the post is', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('tab', { name: 'Document' })

	expect(screen.getByRole('combobox', { name: 'Status' })).toHaveTextContent('Draft')
})

test('turns to the block tab when asked', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('tab', { name: 'Block' }))

	expect(screen.getByRole('tab', { name: 'Block' })).toHaveAttribute('aria-selected', 'true')
	expect(screen.getByRole('tab', { name: 'Document' })).toHaveAttribute('aria-selected', 'false')
})

test('says which block is selected when none is', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('tab', { name: 'Block' }))

	expect(await screen.findByText(/no block selected/i)).toBeInTheDocument()
})

test('closes the settings from their own header and hands focus back to the settings button', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Close Settings' }))

	expect(document.querySelector('.gophenberg-editor__sidebar')).toHaveAttribute('hidden')
	expect(screen.getByRole('button', { name: 'Settings' })).toHaveFocus()
})

test('brings the settings back on the tab they showed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('tab', { name: 'Block' }))
	await userEvent.click(screen.getByRole('button', { name: 'Close Settings' }))

	await userEvent.click(screen.getByRole('button', { name: 'Settings' }))

	expect(screen.getByRole('tab', { name: 'Block' })).toHaveAttribute('aria-selected', 'true')
})

test('holds the tabs and the close button in one row', async () => {
	renderAt(EDITOR_PATH)

	const close = await screen.findByRole('button', { name: 'Close Settings' })
	const row = close.closest('.gophenberg-editor__sidebar-header') as HTMLElement

	expect(row).not.toBeNull()
	expect(within(row).getByRole('tablist')).toBeInTheDocument()
})

test('shows a status it holds no label for as the server named it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'scheduled' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('combobox', { name: 'Status' })).toHaveTextContent('scheduled')
})

test('shows a published post as published in the document tab', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({
				...storedPost,
				status: 'published',
				published_at: '2026-07-20T10:00:00Z',
			}),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('combobox', { name: 'Status' })).toHaveTextContent('Published')
})
