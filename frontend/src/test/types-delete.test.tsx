// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'

const POST_TYPE = {
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

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: 'Manage the pages on this site.',
	route_word: 'pages',
	default: false,
}

/**
 * Records every delete the registry is asked for and answers each with the given response.
 * @param answer - Builds the response each delete gets.
 * @returns The paths asked for, filled as requests arrive.
 */
function recordDeletes(answer: () => Response = () => new HttpResponse(null, { status: 204 })): string[] {
	const asked: string[] = []
	server.use(
		http.delete('/api/types/:key', ({ request }) => {
			asked.push(new URL(request.url).pathname)
			return answer()
		}),
	)
	return asked
}

/**
 * Opens the delete confirm of the Pages row.
 * @returns The confirm dialog.
 */
async function openDelete(): Promise<HTMLElement> {
	const table = await screen.findByRole('region', { name: 'Content Types' })
	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Delete' }))
	return screen.findByRole('alertdialog')
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
})

test('asks before deleting a type, naming it in quotes, and deletes nothing yet', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(dialog).toHaveAccessibleName('Delete "Pages"?')
	expect(dialog).toHaveAccessibleDescription('This cannot be undone.')
	expect(asked).toEqual([])
})

test('cuts a long type name in the delete confirm title at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 3 })))
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(dialog).toHaveAccessibleName('Delete "Pag…"?')
})

test('draws the delete confirm as an irreversible one with no close button', async () => {
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Delete' }).className).toMatch(/__irreversible-action\b/)
})

test('opens the delete confirm on Cancel, as the WordPress design system confirm does', async () => {
	renderAt('/content-types')

	const dialog = await openDelete()

	await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus())
})

test('deletes nothing when the confirm is cancelled', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(asked).toEqual([])
})

test('removes a type the registry lets go once the confirm is accepted, and says so', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	await waitFor(() => expect(asked).toEqual(['/api/types/page']))
	expect(await screen.findByText('Pages removed.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test.each([
	{
		locale: 'es-ES',
		region: 'Tipos de contenido',
		pages: /Páginas/,
		remove: 'Borrar',
		removed: 'Tipo de contenido Páginas eliminado.',
	},
	{
		locale: 'fr-FR',
		region: 'Types de contenu',
		pages: /Pages/,
		remove: 'Supprimer',
		removed: 'Type de contenu Pages retiré.',
	},
])('names a removed built-in type by its kind for a $locale reader', async (reader) => {
	setLocaleData(await catalogFor(reader.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	recordDeletes()
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: reader.region })

	const pages = within(table).getByRole('row', { name: reader.pages })
	await userEvent.click(within(pages).getByRole('button', { name: reader.remove }))
	const dialog = await screen.findByRole('alertdialog')
	await userEvent.click(within(dialog).getByRole('button', { name: reader.remove }))

	expect(await screen.findByText(reader.removed, { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('lands the focus on the types table once the deleted type has left it', async () => {
	let deleted = false
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: deleted ? [POST_TYPE] : [POST_TYPE, PAGE_TYPE] })),
		http.delete('/api/types/:key', () => {
			deleted = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(await screen.findByText('Pages removed.', { selector: '.godmin-toast' })).toBeInTheDocument()
	const table = screen.getByRole('region', { name: 'Content Types' })
	await waitFor(() => expect(within(table).queryByRole('row', { name: /Pages/ })).not.toBeInTheDocument())
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
	await waitFor(() => expect(table).toHaveFocus())
})

test('hands the focus back to Delete when the confirm is cancelled', async () => {
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	const pages = within(screen.getByRole('region', { name: 'Content Types' })).getByRole('row', { name: /Pages/ })
	await waitFor(() => expect(within(pages).getByRole('button', { name: 'Delete' })).toHaveFocus())
})

test('keeps the confirm open on the reason a type could not be removed', async () => {
	recordDeletes(() => HttpResponse.json({ error: 'content: the type is in use' }, { status: 422 }))
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(await within(dialog).findByText(/the type is in use/)).toBeInTheDocument()
	expect(screen.getByRole('alertdialog')).toBeInTheDocument()
})
