// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { deleteFailure, deleteQuestion } from '../media/actions'
import { filteredKind } from '../media/MediaScreen'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	description: 'From the eastern pier',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {
		thumbnail: {
			file: '2026/08/harbor-150x150.jpg',
			width: 150,
			height: 150,
			mime_type: 'image/jpeg',
			filesize: 9000,
		},
	},
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-10T10:00:00Z',
}

const MANUAL = {
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	mime_type: 'application/pdf',
	filesize: 12000,
	sizes: {},
	created_at: '2026-08-09T10:00:00Z',
	updated_at: '2026-08-09T10:00:00Z',
}

/**
 * Serves the given media rows from the listing route.
 * @param items - The rows the listing answers.
 */
function listing(items: unknown[] = [HARBOR, MANUAL]) {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({ items, total: items.length, per_page: siteSettings.list_page_size }),
		),
	)
}

beforeEach(() => {
	listing()
})

test('lists every media item the API returned', async () => {
	renderAt('/media')

	expect(await screen.findByText('Harbor at dawn')).toBeInTheDocument()
	expect(screen.getByText('Manual')).toBeInTheDocument()
})

test('asks the page size the settings name', async () => {
	const asked: (string | null)[] = []
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_size: 100 })),
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams.get('per_page'))
			return HttpResponse.json({ items: [HARBOR], total: 1, per_page: 100 })
		}),
	)
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(asked).toEqual(['100'])
})

test('counts its pages by the size the server served', async () => {
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_size: 50 })),
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, MANUAL], total: 25, per_page: 10 })),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	const pages = within(screen.getByRole('combobox', { name: 'Current page' })).getAllByRole('option')

	expect(pages.map((page) => page.textContent)).toEqual(['1', '2', '3'])
})

test('draws a picture from its smallest fitting sub-size', async () => {
	renderAt('/media')

	const picture = await screen.findByAltText('Boats at sunrise')

	expect(picture).toHaveAttribute('src', '/media/2026/08/harbor-150x150.jpg')
	expect(picture).toHaveAttribute('loading', 'lazy')
})

test('names a picture that carries no alt text by its title', async () => {
	listing([{ ...HARBOR, alt_text: '' }])
	renderAt('/media')

	expect(await screen.findByAltText('Harbor at dawn')).toBeInTheDocument()
})

test('stands a file in for a picture it cannot draw', async () => {
	renderAt('/media')

	const stand = await screen.findByText('manual.pdf')

	expect(stand).toBeInTheDocument()
	expect(screen.queryByAltText('Manual')).not.toBeInTheDocument()
})

test('stands in for a picture whose file will not load', async () => {
	listing([HARBOR])
	renderAt('/media')
	const picture = await screen.findByAltText('Boats at sunrise')

	picture.dispatchEvent(new Event('error'))

	expect(await screen.findByText('harbor.jpg')).toBeInTheDocument()
})

test('reports a library that could not be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/media')

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test('says the library holds nothing yet, and how to add a file, when nothing narrows it', async () => {
	listing([])
	renderAt('/media')

	const empty = (await screen.findByText('No media items yet.')).closest('.godmin-empty') as HTMLElement

	expect(within(empty).getByText('Add one with Add Media File.')).toBeInTheDocument()
	expect(empty.querySelector('svg')).not.toBeNull()
})

test('says no media item was found when a search narrows the library to nothing', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	listing([])

	await userEvent.type(screen.getByRole('searchbox'), 'lighthouse')

	expect(await screen.findByText('No media items found.')).toBeInTheDocument()
	expect(screen.queryByText('Add one with Add Media File.')).not.toBeInTheDocument()
})

test('says under the Media title what the library holds', async () => {
	renderAt('/media')

	expect(await screen.findByText('Manage the images and files this site holds.')).toHaveClass('godmin-page__subtitle')
})

test('says no media item was found when the kind filter narrows the library to nothing', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	listing([])

	await userEvent.click(screen.getByRole('button', { name: /Kind/ }))
	await userEvent.click(await screen.findByRole('option', { name: 'Files' }))

	expect(await screen.findByText('No media items found.')).toBeInTheDocument()
	expect(screen.queryByText('Add one with Add Media File.')).not.toBeInTheDocument()
})

test('draws the library in the list region of the page, named Media', async () => {
	renderAt('/media')

	const region = await screen.findByRole('region', { name: 'Media' })

	expect(await within(region).findByText('Harbor at dawn')).toBeInTheDocument()
	expect(region).toHaveClass('godmin-page__list')
	expect(region.parentElement).toHaveClass('godmin-page')
	expect(region).toHaveAttribute('tabindex', '-1')
})

test('lays a drop zone reading Drop files to upload over the list region', async () => {
	renderAt('/media')

	const region = await screen.findByRole('region', { name: 'Media' })
	const zone = region.querySelector(':scope > .godmin-list-overlay') as HTMLElement

	expect(zone).not.toBeNull()
	expect(zone).toHaveTextContent('Drop files to upload')
})

test('ignores a drop that carries no file, while the drop after it carrying one uploads it', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/media', () => {
			uploaded += 1
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const zone = document.querySelector('.godmin-list-overlay') as HTMLElement
	const dropped = (files: File[]) =>
		zone.dispatchEvent(
			Object.assign(new Event('drop', { bubbles: true, cancelable: true }), {
				dataTransfer: { files, items: [], types: files.length > 0 ? ['Files'] : [], getData: () => '' },
			}),
		)

	dropped([])
	dropped([new File(['bytes'], 'harbor.jpg', { type: 'image/jpeg' })])

	expect(await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(uploaded).toBe(1)
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('deletes a media item for good after a confirmation', async () => {
	let deleted = 0
	server.use(
		http.delete('/api/media/7', () => {
			deleted += 1
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /delete permanently/i }))
	await userEvent.click(await screen.findByRole('button', { name: /delete permanently/i }))

	await waitFor(() => expect(deleted).toBe(1))
})

test('reports a delete that failed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/media/7', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /delete permanently/i }))
	await userEvent.click(await screen.findByRole('button', { name: /delete permanently/i }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not delete/i)
})

test('saves the descriptions of a media item', async () => {
	let sent: Record<string, unknown> = {}
	server.use(
		http.patch('/api/media/7', async ({ request }) => {
			sent = (await request.json()) as Record<string, unknown>
			return HttpResponse.json({ ...HARBOR, title: 'Harbor at noon' })
		}),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /describe/i }))
	const title = await screen.findByLabelText('Title')
	await userEvent.clear(title)
	await userEvent.type(title, 'Harbor at noon')
	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	await waitFor(() => expect(sent.title).toBe('Harbor at noon'))
})

test('reports descriptions the server found stale', async () => {
	server.use(http.patch('/api/media/7', () => HttpResponse.json({}, { status: 409 })))
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /describe/i }))
	await userEvent.click(await screen.findByRole('button', { name: /save/i }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/changed while/i)
})

test('reports descriptions that never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/media/7', () => HttpResponse.error()))
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /describe/i }))
	await userEvent.click(await screen.findByRole('button', { name: /save/i }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not be reached/i)
})

test('reports descriptions the server refused', async () => {
	server.use(
		http.patch('/api/media/7', () =>
			HttpResponse.json({ error: 'that is not a media item' }, { status: 404 }),
		),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getAllByRole('button', { name: /actions/i })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: /describe/i }))
	await userEvent.click(await screen.findByRole('button', { name: /save/i }))

	expect(await screen.findByRole('alert')).toHaveTextContent('that is not a media item')
})

test('shows the size and shape of a picture', async () => {
	renderAt('/media')

	expect(await screen.findByText('1600 × 1000')).toBeInTheDocument()
	expect(screen.getByText('248,05 KB')).toBeInTheDocument()
})

test('says nothing about the shape of a file that has none', async () => {
	listing([MANUAL])
	renderAt('/media')

	expect(await screen.findByText('11,72 KB')).toBeInTheDocument()
	expect(screen.queryByText(/×/)).not.toBeInTheDocument()
})

test('describes every stored file when the library is laid out as a table', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: /layout/i }))
	await userEvent.click(await screen.findByRole('menuitemradio', { name: /table/i }))

	expect(await screen.findByText('harbor.jpg')).toBeInTheDocument()
	expect(screen.getByText('image/jpeg')).toBeInTheDocument()
	expect(screen.getByText('10/08/2026')).toBeInTheDocument()
})

test('dates a media item that carries no timestamp', async () => {
	listing([{ ...MANUAL, created_at: undefined }])
	renderAt('/media')
	await screen.findByText('Manual')

	await userEvent.click(screen.getByRole('button', { name: /layout/i }))
	await userEvent.click(await screen.findByRole('menuitemradio', { name: /table/i }))

	expect(await screen.findAllByText('manual.pdf')).not.toHaveLength(0)
})

test('asks for every kind until a kind is filtered for', () => {
	const view = { type: 'grid', fields: [] } as unknown as Parameters<typeof filteredKind>[0]

	expect(filteredKind(view)).toBe('')
	expect(filteredKind({ ...view, filters: [{ field: 'type', operator: 'is', value: 'image' }] })).toBe(
		'image',
	)
	expect(filteredKind({ ...view, filters: [{ field: 'type', operator: 'is', value: 7 }] })).toBe('')
})

test('asks about every item a delete was asked over', () => {
	const one = [{ title: 'Harbor at dawn', file: 'a.jpg' }] as Parameters<typeof deleteQuestion>[0]
	const two = [...one, { title: 'Manual', file: 'b.pdf' }] as Parameters<typeof deleteQuestion>[0]

	expect(deleteQuestion(one)).toContain('Delete Harbor at dawn for good?')
	expect(deleteQuestion(two)).toContain('Delete these 2 items for good?')
	expect(deleteFailure(one)).toBe('Could not delete that item.')
	expect(deleteFailure(two)).toBe('Could not delete every item.')
})

test('groups the count a delete asks about in the site format', () => {
	const many = Array.from({ length: 1234 }, () => ({ title: 'Manual', file: 'b.pdf' })) as Parameters<
		typeof deleteQuestion
	>[0]

	expect(deleteQuestion(many)).toContain('Delete these 1.234 items for good?')
})
