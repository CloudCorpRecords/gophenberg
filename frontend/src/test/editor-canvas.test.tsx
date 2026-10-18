// SPDX-License-Identifier: Apache-2.0

import {
	BlockEditorProvider,
	BlockList,
	CANVAS_LAYOUT,
	apiFetchAttempts,
	clearApiFetchAttempts,
	parse,
} from '@gophenberg/frontend-sdk/editor'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { render, screen } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { CANVAS_ROOT, EDITOR_SETTINGS, editorSettings } from '../content/editorSetup'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const GROUPS = {
	full: 'Inside full group',
	fullInFull: 'Inside nested full group',
	plain: 'Inside plain group',
	fullInPlain: 'Inside full group in plain group',
	flow: 'Inside flow full group',
	fullInFlow: 'Inside full group in flow group',
}

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

const NESTED_GROUPS = [
	group(FULL, [paragraph(GROUPS.full), group(FULL, [paragraph(GROUPS.fullInFull)])]),
	group(PLAIN, [paragraph(GROUPS.plain), group(FULL, [paragraph(GROUPS.fullInPlain)])]),
	group(FLOW, [paragraph(GROUPS.flow), group(FULL, [paragraph(GROUPS.fullInFlow)])]),
].join('\n\n')

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	clearApiFetchAttempts()
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

/**
 * Renders full, plain and flow groups holding full groups, under the settings and root the editor screen uses.
 */
function renderGroups() {
	render(
		<BlockEditorProvider value={parse(NESTED_GROUPS)} settings={editorSettings(false)}>
			<BlockList {...CANVAS_ROOT} />
		</BlockEditorProvider>,
	)
}

/**
 * Returns the group block holding a paragraph.
 * @param words - The words of the paragraph.
 * @returns The group element.
 */
function groupOf(words: string): HTMLElement {
	return screen.getByText(words).parentElement as HTMLElement
}

test('mounts the block canvas in an iframe', async () => {
	renderAt(EDITOR_PATH)

	const canvas = await screen.findByTitle('Editor canvas')

	expect(canvas.tagName).toBe('IFRAME')
})

test('mounts the canvas the block editor drives, not a bare frame', async () => {
	renderAt(EDITOR_PATH)

	const canvas = await screen.findByTitle('Editor canvas')

	expect(canvas).toHaveAttribute('name', 'editor-canvas')
	expect(canvas.getAttribute('src')).toMatch(/^blob:/)
})

test('reaches for no WordPress REST endpoint while the canvas renders', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTitle('Editor canvas')

	expect(apiFetchAttempts()).toEqual([])
})

test('carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTitle('Editor canvas')

	expect(document.querySelector('.godmin-layout')).toBeNull()
	expect(document.querySelector('.godmin-layout__canvas')).toBeNull()
})

test('turns on the root padding aware alignments of a WordPress block theme, at the canvas column sizes', () => {
	expect(EDITOR_SETTINGS.__experimentalFeatures).toEqual({
		layout: CANVAS_LAYOUT,
		useRootPaddingAwareAlignments: true,
	})
	expect(EDITOR_SETTINGS.__unstableIsBlockBasedTheme).toBe(true)
})

test('lays the canvas root out as the WordPress post editor does, a padded constrained column', () => {
	renderGroups()

	expect(document.querySelector('.is-root-container')).toHaveClass('is-layout-constrained', 'has-global-padding')
	expect(CANVAS_ROOT.layout).toEqual({ ...CANVAS_LAYOUT, type: 'constrained' })
})

test('aligns a group full at the root and inside a constrained group', () => {
	renderGroups()

	for (const words of [GROUPS.full, GROUPS.fullInFull, GROUPS.fullInPlain]) {
		expect(groupOf(words)).toHaveClass('alignfull')
	}
})

test('aligns no group full inside a flow group, as a WordPress block theme offers none there', () => {
	renderGroups()

	expect(groupOf(GROUPS.fullInFlow)).not.toHaveClass('alignfull')
})

test('pads every constrained group and leaves a full flow group unpadded, as WordPress does', () => {
	renderGroups()

	for (const words of [GROUPS.full, GROUPS.fullInFull, GROUPS.plain, GROUPS.fullInPlain, GROUPS.fullInFlow]) {
		expect(groupOf(words)).toHaveClass('has-global-padding')
	}
	expect(groupOf(GROUPS.flow)).toHaveClass('alignfull', 'is-layout-flow')
	expect(groupOf(GROUPS.flow)).not.toHaveClass('has-global-padding')
})
