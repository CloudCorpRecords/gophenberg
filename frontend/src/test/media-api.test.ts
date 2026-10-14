// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	deleteMedia,
	describeMedia,
	listMedia,
	listMediaByIDs,
	mediaSrc,
	uploadMedia,
} from '../media/api'

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
		full: {
			file: '2026/08/harbor.jpg',
			width: 1600,
			height: 1000,
			mime_type: 'image/jpeg',
			filesize: 254000,
		},
	},
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-10T10:00:00Z',
}

test('lists media with the query it was given', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1, per_page: 10 })
		}),
	)

	const page = await listMedia({ type: 'image', search: 'harbor', page: 2 })

	expect(page.total).toBe(1)
	expect(page.items[0].title).toBe('Harbor at dawn')
	expect(asked).toContain('type=image')
	expect(asked).toContain('search=harbor')
	expect(asked).toContain('page=2')
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 30 })
		}),
	)

	await listMedia({ perPage: 30 })

	expect(asked).toBe('?per_page=30')
})

test('leaves the page size to the server when the caller names none', async () => {
	let asked: string | null = null
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 10 })
		}),
	)

	await listMedia({})

	expect(asked).toBe('')
})

test('reads the page size the server used, which may fall short of the one asked for', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({ items: [HARBOR], total: 25, per_page: 10 })))

	const page = await listMedia({ perPage: 30 })

	expect(page).toMatchObject({ total: 25, perPage: 10 })
})

test('refuses a listing that names no page size', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({ items: [HARBOR], total: 1 })))

	await expect(listMedia({})).rejects.toThrow()
})

test('asks for the files a gallery holds by identity in one call', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	const held = await listMediaByIDs([7, 9])

	expect(asked).toBe('?ids=7%2C9')
	expect(held.map((item) => item.title)).toEqual(['Harbor at dawn'])
})

test('asks in pages when a gallery holds more files than one request carries', async () => {
	const asked: string[] = []
	server.use(
		http.get('/api/media', ({ request }) => {
			const ids = new URL(request.url).searchParams.get('ids') ?? ''
			asked.push(ids)
			return HttpResponse.json({
				items: ids.split(',').map((id) => ({ ...HARBOR, id: Number(id) })),
				total: ids.split(',').length,
			})
		}),
	)

	const held = await listMediaByIDs(Array.from({ length: 250 }, (_, at) => at + 1))

	expect(asked.map((page) => page.split(',').length)).toEqual([100, 100, 50])
	expect(held.map((item) => item.id)).toEqual(Array.from({ length: 250 }, (_, at) => at + 1))
})

test('reports the files it could not read by identity', async () => {
	server.use(http.get('/api/media', () => new HttpResponse(null, { status: 500 })))

	await expect(listMediaByIDs([7])).rejects.toThrow(/500/)
})

test('asks for no file when a gallery holds none', async () => {
	let called = false
	server.use(
		http.get('/api/media', () => {
			called = true
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	const held = await listMediaByIDs([])

	expect(held).toEqual([])
	expect(called).toBe(false)
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 30 })
		}),
	)

	await listMedia({ page: 1, perPage: 30 })

	expect(asked).toBe('?per_page=30')
})

test('fills in what a sparse media row leaves out', async () => {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [{ id: 8, type: 'file', file: 'manual.pdf', mime_type: 'application/pdf' }],
				total: 1,
				per_page: 10,
			}),
		),
	)

	const page = await listMedia({})

	expect(page.items[0]).toMatchObject({
		title: '',
		altText: '',
		caption: '',
		description: '',
		width: 0,
		height: 0,
		filesize: 0,
		sizes: {},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})
})

test('reports a listing that could not be read', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))

	await expect(listMedia({})).rejects.toThrow(/500/)
})

test('uploads a file as multipart under the field the server reads', async () => {
	let contentType: string | null = null
	let body = ''
	server.use(
		http.post('/api/media', async ({ request }) => {
			contentType = request.headers.get('content-type')
			body = await request.text()
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)

	const stored = await uploadMedia(new File(['the bytes'], 'harbor.jpg', { type: 'image/jpeg' }))

	expect(stored).toEqual(expect.objectContaining({ id: 7 }))
	expect(contentType).toMatch(/^multipart\/form-data; boundary=/)
	expect(body).toMatch(/name="file"; filename=/)
	expect(body).toContain('Content-Type: image/jpeg')
})

test('throws the reason an upload was refused, so a run never counts it as done', async () => {
	server.use(
		http.post('/api/media', () =>
			HttpResponse.json({ error: 'the file type is not allowed' }, { status: 422 }),
		),
	)

	await expect(uploadMedia(new File(['x'], 'notes.txt', { type: 'text/plain' }))).rejects.toThrow(
		'the file type is not allowed',
	)
})

test('throws a stand in when a refused upload carries no message', async () => {
	server.use(http.post('/api/media', () => new HttpResponse('', { status: 413 })))

	await expect(uploadMedia(new File(['x'], 'huge.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		'Something went wrong. Try again.',
	)
})

test('throws in the reader words an upload that never reached the server', async () => {
	server.use(http.post('/api/media', () => HttpResponse.error()))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		'The server could not be reached, so nothing was uploaded.',
	)
})

test('throws a stand in, never the words of a parser, when a stored upload answers with no JSON', async () => {
	server.use(http.post('/api/media', () => new HttpResponse('<html>gateway</html>', { status: 201 })))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		/^Something went wrong\. Try again\.$/,
	)
})

test('throws a stand in, never the words of a schema, when a stored upload answers with a misshapen item', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json({ id: 'seven' }, { status: 201 })))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		/^Something went wrong\. Try again\.$/,
	)
})

test('saves descriptions against the version it read', async () => {
	let sent: Record<string, unknown> = {}
	server.use(
		http.patch('/api/media/7', async ({ request }) => {
			sent = (await request.json()) as Record<string, unknown>
			return HttpResponse.json({ ...HARBOR, title: 'Harbor at noon' })
		}),
	)

	const outcome = await describeMedia(7, { title: 'Harbor at noon' }, HARBOR.updated_at)

	expect(outcome).toEqual({ kind: 'saved', item: expect.objectContaining({ title: 'Harbor at noon' }) })
	expect(sent).toEqual({ title: 'Harbor at noon', updated_at: HARBOR.updated_at })
})

test('reports an edit the server found stale', async () => {
	server.use(http.patch('/api/media/7', () => HttpResponse.json({}, { status: 409 })))

	const outcome = await describeMedia(7, { title: 'Harbor at noon' }, HARBOR.updated_at)

	expect(outcome.kind).toBe('stale')
})

test('carries the reason an edit was refused', async () => {
	server.use(
		http.patch('/api/media/7', () =>
			HttpResponse.json({ error: 'that is not a media item' }, { status: 404 }),
		),
	)

	const outcome = await describeMedia(7, { title: 'x' }, HARBOR.updated_at)

	expect(outcome).toEqual({ kind: 'rejected', message: 'that is not a media item' })
})

test('deletes a media item', async () => {
	let asked = ''
	server.use(
		http.delete('/api/media/7', ({ request }) => {
			asked = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)

	await deleteMedia(7)

	expect(asked).toBe('/api/media/7')
})

test('reports a delete that failed', async () => {
	server.use(http.delete('/api/media/7', () => HttpResponse.json({}, { status: 500 })))

	await expect(deleteMedia(7)).rejects.toThrow(/500/)
})

test('addresses a stored file under the public media prefix', () => {
	expect(mediaSrc('2026/08/harbor.jpg')).toBe('/media/2026/08/harbor.jpg')
})
