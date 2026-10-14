// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import '../content/editor.css'
import { catalogFor } from '../i18n/catalog'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

/** A type the site owner registered, which Gophenberg ships no words for. */
const GUIDE_TYPE = {
	key: 'guide',
	singular_label: 'Guide',
	plural_label: 'Guides',
	description: '',
	route_word: 'guides',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: false,
	active: true,
	fields: [],
}

/** The page type the demo seed registers, still holding the English Gophenberg ships for it. */
const PAGE_TYPE = {
	...GUIDE_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: 'Manage the pages on this site.',
	route_word: 'pages',
}

/**
 * Returns the foot of the editor, where the trail to the selected block sits.
 * @returns The foot element.
 */
function editorFoot(): HTMLElement {
	return document.querySelector('.gophenberg-editor__foot') as HTMLElement
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('names the post and its type in the header', async () => {
	renderAt(EDITOR_PATH)

	const bar = await screen.findByTestId('document-bar')

	expect(bar).toHaveTextContent('Welcome to Gophenberg')
	await waitFor(() => expect(bar).toHaveTextContent('Post'))
})

test('names the type of the post in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt(EDITOR_PATH)

	const bar = await screen.findByTestId('document-bar')

	await waitFor(() => expect(bar).toHaveTextContent(/·\s*Entrada$/))
	expect(screen.getByRole('link', { name: 'Volver a entradas' })).toBeInTheDocument()
})

test('sends a French reader back to the articles in the words of WordPress fr_FR', async () => {
	setLocaleData(await catalogFor('fr-FR'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt(EDITOR_PATH)

	await screen.findByTestId('document-bar')

	expect(await screen.findByRole('link', { name: 'Retour aux articles' })).toBeInTheDocument()
})

test('roots the block trail of a page at the name WordPress es_ES gives a page for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [PAGE_TYPE] })),
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, type: 'page' })),
	)
	renderAt(`/content/page/${storedPost.id}/edit`)
	await screen.findByTestId('document-bar')

	await waitFor(() => expect(within(editorFoot()).getByText('Página')).toBeInTheDocument())
	expect(within(editorFoot()).queryByText('Entrada')).not.toBeInTheDocument()
})

test('roots the block trail at the singular name of a type the site registered', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [GUIDE_TYPE] })),
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, type: 'guide' })),
	)
	renderAt(`/content/guide/${storedPost.id}/edit`)
	await screen.findByTestId('document-bar')

	await waitFor(() => expect(within(editorFoot()).getByText('Guide')).toBeInTheDocument())
	expect(within(editorFoot()).queryByText('Post')).not.toBeInTheDocument()
})

test('calls an untitled post untitled in the bar', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByTestId('document-bar')).toHaveTextContent('No title')
})

test('names a registered type by the label the registry carries', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [
					{
						key: 'guide',
						singular_label: 'Guide',
						plural_label: 'Guides',
						description: '',
						route_word: 'guides',
						hierarchical: false,
						revisions: true,
						revision_cap: 100,
						page_kind: 'single',
						default: false,
						active: true,
						fields: [],
					},
				],
			}),
		),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, type: 'guide' }),
		),
	)
	renderAt(`/content/guide/${storedPost.id}/edit`)

	const bar = await screen.findByTestId('document-bar')

	await waitFor(() => expect(bar).toHaveTextContent('Guide'))
})

test('shows a type it holds no label for as the server named it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, type: 'briefing' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByTestId('document-bar')).toHaveTextContent('briefing')
})

test('centres the bar between the header controls', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTestId('document-bar')

	const bar = document.querySelector('.gophenberg-editor__document') as HTMLElement
	const header = document.querySelector('.gophenberg-editor__header') as HTMLElement

	expect(getComputedStyle(header).position).toBe('relative')
	expect(getComputedStyle(bar).position).toBe('absolute')
	expect(getComputedStyle(bar).left).toBe('50%')
	expect(getComputedStyle(bar).transform).toBe('translateX(-50%)')
})

test('foots the editor with the trail to the selected block', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const foot = document.querySelector('.gophenberg-editor__foot')

	expect(foot).not.toBeNull()
	expect(document.querySelector('.gophenberg-editor')?.lastElementChild).toBe(foot)
})
