// SPDX-License-Identifier: Apache-2.0

import { CANVAS_STYLES } from '@gophenberg/frontend-sdk/editor'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../content/editor.css'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const EDITOR_SHEET = readFileSync(join(import.meta.dirname, '..', 'content', 'editor.css'), 'utf8')

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

/**
 * Returns the editor element carrying the given chrome class.
 * @param part - The chrome part to look for.
 * @returns The element.
 */
function chrome(part: string): HTMLElement {
	return document.querySelector(`.gophenberg-editor__${part}`) as HTMLElement
}

/**
 * Returns every rule the document's stylesheets hold, as text.
 * @returns The rules, one after another.
 */
function sheetText(): string {
	return Array.from(document.styleSheets)
		.flatMap((sheet) => Array.from(sheet.cssRules ?? []))
		.map((rule) => rule.cssText)
		.join('\n')
}

test('stands the editor up as a column filling the screen', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const editor = document.querySelector('.gophenberg-editor') as HTMLElement

	expect(getComputedStyle(editor).flexDirection).toBe('column')
	expect(getComputedStyle(editor).height).toBe(`${window.innerHeight}px`)
})

test('holds the header to one row under a rule', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const header = chrome('header')

	expect(getComputedStyle(header).height).toBe('46px')
	expect(getComputedStyle(header).display).toBe('grid')
	expect(getComputedStyle(header).borderBottomWidth).toBe('1px')
	expect(getComputedStyle(header).maxWidth).toBe(`${window.innerWidth}px`)
})

test('gives the writing area the room the sidebar does not take', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(getComputedStyle(chrome('content')).flexGrow).toBe('1')
	expect(getComputedStyle(chrome('content')).minWidth).toBe('0px')
})

test('holds the sidebar to its own column beside the writing area', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const sidebar = chrome('sidebar')

	expect(getComputedStyle(sidebar).flexBasis).toBe('var(--gophenberg-editor-sidebar-width)')
	expect(getComputedStyle(document.documentElement).getPropertyValue('--gophenberg-editor-sidebar-width')).toBe('280px')
	expect(getComputedStyle(sidebar).flexGrow).toBe('0')
	expect(getComputedStyle(sidebar).flexShrink).toBe('0')
	expect(getComputedStyle(sidebar).borderLeftWidth).toBe('1px')
})

test('holds the settings tabs and their close button in one sticky row under a rule', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('button', { name: 'Close Settings' })

	const row = chrome('sidebar-header')

	expect(getComputedStyle(row).position).toBe('sticky')
	expect(getComputedStyle(row).display).toBe('flex')
	expect(getComputedStyle(row).borderBottomWidth).toBe('1px')
	expect(getComputedStyle(chrome('sidebar-tabs')).borderBottomStyle).not.toBe('solid')
})

test('takes closed settings out of the layout', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Close Settings' }))

	expect(getComputedStyle(chrome('sidebar')).display).toBe('none')
})

test('slides the settings in and out over 300ms with the WordPress ease, and at once under reduced motion', () => {
	const styles = sheetText()
	const slide = /\.gophenberg-editor__sidebar--moved \{[^}]*transition:[^}]*300ms cubic-bezier\(0\.6, 0, 0\.4, 1\)/

	expect(styles).toMatch(slide)
	expect(styles).toMatch(/prefers-reduced-motion: reduce[^@]*\.gophenberg-editor__sidebar--moved[^}]*transition: none/)
})

test('writes the width of each side panel once, for its column, its slide and the toasts beside it to share', () => {
	expect(EDITOR_SHEET.match(/\d+px/g)?.filter((width) => width === '280px')).toEqual(['280px', '280px'])
	expect(EDITOR_SHEET).toMatch(/--gophenberg-editor-sidebar-width: 280px/)
	expect(EDITOR_SHEET).toMatch(/--gophenberg-editor-outline-width: 280px/)
})

test('slides the settings only once they have moved, never as the editor loads', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(chrome('sidebar')).not.toHaveClass('gophenberg-editor__sidebar--moved')
	expect(sheetText()).not.toMatch(/^\.gophenberg-editor__sidebar \{[^}]*transition/m)

	await userEvent.click(screen.getByRole('button', { name: 'Settings' }))

	expect(chrome('sidebar')).toHaveClass('gophenberg-editor__sidebar--moved')
})

test('lets the main area fill what the header leaves', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const main = chrome('main')

	expect(getComputedStyle(main).display).toBe('flex')
	expect(getComputedStyle(main).flexGrow).toBe('1')
	expect(getComputedStyle(main).minHeight).toBe('0px')
})

test('writes the title over the column the blocks sit in', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const title = chrome('title')

	expect(getComputedStyle(title).marginLeft).toBe('auto')
	expect(sheetText()).toMatch(
		/\.gophenberg-editor__title \{[^}]*max-width: calc\(700px \+ 2 \* var\(--gophenberg-canvas-gutter\)\)/,
	)
})

test('pads the title by the gutter the canvas keeps, measured on the writing area', () => {
	const styles = sheetText()

	expect(styles).toMatch(/\.gophenberg-editor__content \{[^}]*container-type: inline-size/)
	expect(styles).toMatch(/\.gophenberg-editor__content \{[^}]*--gophenberg-canvas-gutter: clamp\(30px,\s*5cqi,\s*50px\)/)
	expect(styles).toMatch(/\.gophenberg-editor__title \{[^}]*padding: 32px var\(--gophenberg-canvas-gutter\) 0/)
})

test('pads the title by the same gutter the canvas body keeps, so title and blocks line up', () => {
	const gutter = /--gophenberg-canvas-gutter:\s*clamp\(([^)]*)\)/
	const canvas = CANVAS_STYLES.map((entry) => entry.css).join('\n').match(gutter)

	expect(canvas?.[1]).toMatch(/vw/)
	expect(EDITOR_SHEET.match(gutter)?.[1].replace('cqi', 'vw')).toBe(canvas?.[1])
})

test('leaves the block toolbar to float by its block', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(chrome('toolbar')).toBeNull()
	expect(chrome('header').querySelector('.block-editor-block-toolbar')).toBeNull()
	expect(within(chrome('content')).queryByRole('toolbar', { name: 'Block tools' })).toBeNull()
})
