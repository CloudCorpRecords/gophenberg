// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const CONTENT_SIZE = 700

const TOP = 'Top paragraph'

const GROUPS = {
	full: 'Inside full group',
	fullInFull: 'Inside nested full group',
	plain: 'Inside plain group',
	fullInPlain: 'Inside full group in plain group',
	flow: 'Inside flow full group',
	fullInFlow: 'Inside full group in flow group',
}

const SIZES = [
	{ name: 'on a 360 phone', viewport: { width: 360, height: 800 }, phone: true, canvas: 360, gutter: 30 },
	{ name: 'on a 390 phone', viewport: { width: 390, height: 844 }, phone: true, canvas: 390, gutter: 30 },
	{
		name: 'at 1280 with the settings open',
		viewport: { width: 1280, height: 900 },
		phone: false,
		canvas: 1000,
		gutter: 50,
	},
]

const created: string[] = []

/**
 * Returns the markup of a paragraph block.
 * @param words - The words it holds.
 * @returns The block markup.
 */
function paragraph(words: string) {
	return `<!-- wp:paragraph -->\n<p>${words}</p>\n<!-- /wp:paragraph -->`
}

/**
 * Returns the markup of a group block around the blocks it holds.
 * @param attributes - Its alignment and layout.
 * @param inner - The markup of the blocks inside.
 * @returns The block markup.
 */
function group(attributes: { align?: string, layout: { type: string } }, inner: string[]) {
	const classes = attributes.align === 'full' ? 'wp-block-group alignfull' : 'wp-block-group'
	const opening = `<!-- wp:group ${JSON.stringify(attributes)} -->\n<div class="${classes}">`
	return `${opening}${inner.join('\n\n')}</div>\n<!-- /wp:group -->`
}

const FULL = { align: 'full', layout: { type: 'constrained' } }

const PLAIN = { layout: { type: 'constrained' } }

const FLOW = { align: 'full', layout: { type: 'default' } }

const CONTENT = [
	paragraph(TOP),
	group(FULL, [paragraph(GROUPS.full), group(FULL, [paragraph(GROUPS.fullInFull)])]),
	group(PLAIN, [paragraph(GROUPS.plain), group(FULL, [paragraph(GROUPS.fullInPlain)])]),
	group(FLOW, [paragraph(GROUPS.flow), group(FULL, [paragraph(GROUPS.fullInFlow)])]),
].join('\n\n')

/**
 * Stores a draft holding a top paragraph and full, plain and flow groups that hold full groups, and opens it.
 * @param page - The page to drive.
 */
async function openDraft(page: Page) {
	const title = `A draft the full width spec opened ${RUN}`
	const answer = await page.request.post('/api/content', { data: { type: 'post', title, content: CONTENT } })
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	created.push(id)
	await page.goto(`/admin/content/post/${id}/edit`)
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
}

/**
 * Reads the canvas width and the box of its body, root container, top paragraph and each group with its paragraph.
 * @param page - The page holding the editor.
 * @returns The canvas width and every box, keyed by what it holds.
 */
async function measure(page: Page) {
	const frame = page.frameLocator('iframe[name="editor-canvas"]')
	await expect(frame.getByText(GROUPS.fullInFlow)).toBeVisible()
	return frame.locator('body').evaluate(
		(body, { top, groups }) => {
			const width = body.ownerDocument.documentElement.clientWidth
			const whole = (value: number) => Math.round(value) + 0
			const px = (value: string) => whole(Number.parseFloat(value))
			const read = (element: Element) => {
				const box = element.getBoundingClientRect()
				const style = getComputedStyle(element)
				return {
					gaps: [whole(box.left), whole(width - box.right)],
					width: whole(box.width),
					padding: [px(style.paddingLeft), px(style.paddingRight)],
					margin: [px(style.marginLeft), px(style.marginRight)],
					full: element.classList.contains('alignfull'),
					aligned: element.getAttribute('data-align'),
					padded: element.classList.contains('has-global-padding'),
				}
			}
			const holding = (words: string) =>
				Array.from(body.querySelectorAll('p')).find((held) => held.textContent?.trim() === words) as Element
			const boxes: Record<string, ReturnType<typeof read>> = {
				body: read(body),
				root: read(body.querySelector('.is-root-container') as Element),
				top: read(holding(top)),
			}
			for (const [key, words] of Object.entries(groups)) {
				const held = holding(words)
				boxes[key] = read(held.parentElement as Element)
				boxes[`${key}Paragraph`] = read(held)
			}
			return { width, boxes }
		},
		{ top: TOP, groups: GROUPS },
	)
}

/**
 * Returns where the WordPress post editor puts each box on a canvas of a width.
 * @param canvas - The width of the canvas.
 * @param gutter - The root padding at that width.
 * @returns The gaps to the canvas edges, width, padding, margin and classes each box takes.
 */
function relations(canvas: number, gutter: number) {
	const column = Math.min(CONTENT_SIZE, canvas - 2 * gutter)
	const side = (canvas - column) / 2
	const edges = { gaps: [0, 0], width: canvas }
	const inColumn = { gaps: [side, side], width: column }
	const fullGroup = { full: true, aligned: null, padded: true }
	const brokenOut = { ...edges, ...fullGroup, margin: [-gutter, -gutter], padding: [gutter, gutter] }
	return {
		body: { padding: [0, 0] },
		root: { ...edges, padding: [gutter, gutter], padded: true },
		top: inColumn,
		full: brokenOut,
		fullParagraph: inColumn,
		fullInFull: brokenOut,
		fullInFullParagraph: inColumn,
		plain: { ...inColumn, padding: [0, 0], full: false, padded: true },
		plainParagraph: inColumn,
		fullInPlain: { ...inColumn, ...fullGroup, margin: [0, 0], padding: [gutter, gutter] },
		fullInPlainParagraph: { gaps: [side + gutter, side + gutter], width: column - 2 * gutter },
		flow: { ...edges, margin: [-gutter, -gutter], padding: [0, 0], full: true, aligned: null, padded: false },
		flowParagraph: edges,
		fullInFlow: { ...edges, padding: [gutter, gutter], full: false, aligned: null, padded: true },
		fullInFlowParagraph: inColumn,
	}
}

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

for (const size of SIZES) {
	test.describe(size.name, () => {
		test.use({ viewport: size.viewport, isMobile: size.phone, hasTouch: size.phone })

		test('lays full, plain and flow groups out against the canvas gutter as WordPress does', async ({ page }) => {
			await openDraft(page)

			const measured = await measure(page)

			expect(measured.width).toBe(size.canvas)
			expect(measured.boxes).toMatchObject(relations(size.canvas, size.gutter))
		})
	})
}
