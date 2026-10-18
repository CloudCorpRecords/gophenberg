// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, renderHook, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../content/editor.css'
import { chosenPreview } from '../content/EditorHeader'
import { useEditorViews } from '../content/useEditorViews'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

/**
 * Renders the editor views over a window of the given width.
 * @param narrow - Whether the window starts below the editor breakpoint.
 * @returns The rendered hook, its views and a way to change the window.
 */
function viewsOver(narrow: boolean) {
	return renderHook(({ below }) => useEditorViews(below), { initialProps: { below: narrow } })
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('keeps the block list out of the way until it is asked for', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.getByRole('button', { name: 'List view' })).toBeInTheDocument()
	expect(document.querySelector('.gophenberg-editor__outline')).toBeNull()
})

test('opens the block list beside the canvas', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	const outline = document.querySelector('.gophenberg-editor__outline') as HTMLElement
	expect(within(outline).getByRole('treegrid')).toBeInTheDocument()
	expect(within(outline).getByText('Paragraph')).toBeInTheDocument()
})

test('closes the block list when it is asked for again', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	expect(document.querySelector('.gophenberg-editor__outline')).toBeNull()
})

test('marks the list view control as pressed while it is open', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	const control = screen.getByRole('button', { name: 'List view' })

	await userEvent.click(control)

	expect(screen.getByRole('button', { name: 'List view' })).toHaveAttribute('aria-pressed', 'true')
})

test('opens the canvas at the width of a desktop', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const canvas = document.querySelector('.gophenberg-editor__canvas') as HTMLElement

	expect(canvas).not.toHaveClass('gophenberg-editor__canvas--tablet')
	expect(canvas).not.toHaveClass('gophenberg-editor__canvas--mobile')
})

test('narrows the canvas to the width asked for', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('combobox', { name: 'Preview' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Mobile' }))

	const canvas = document.querySelector('.gophenberg-editor__canvas') as HTMLElement
	expect(canvas).toHaveClass('gophenberg-editor__canvas--mobile')
})

test('keeps the preview width when the select reports nothing', () => {
	expect(chosenPreview(null, 'tablet')).toBe('tablet')
	expect(chosenPreview({ value: null }, 'tablet')).toBe('tablet')
})

test('opens the settings on a wide window', () => {
	const { result } = viewsOver(false)

	expect(result.current.sidebarOpen).toBe(true)
})

test('keeps the settings closed on a narrow window', () => {
	const { result } = viewsOver(true)

	expect(result.current.sidebarOpen).toBe(false)
})

test('closes the settings when the window narrows', () => {
	const { result, rerender } = viewsOver(false)

	rerender({ below: true })

	expect(result.current.sidebarOpen).toBe(false)
})

test('opens the settings again when the window widens', () => {
	const { result, rerender } = viewsOver(false)
	rerender({ below: true })

	rerender({ below: false })

	expect(result.current.sidebarOpen).toBe(true)
})

test('keeps settings closed by hand closed through a narrow window and back', () => {
	const { result, rerender } = viewsOver(false)
	act(() => result.current.closeSidebar())

	rerender({ below: true })
	rerender({ below: false })

	expect(result.current.sidebarOpen).toBe(false)
})

test('keeps settings opened on a narrow window open as it widens', () => {
	const { result, rerender } = viewsOver(true)
	act(() => result.current.toggleSidebar())

	rerender({ below: false })

	expect(result.current.sidebarOpen).toBe(true)
})

test('reopens the settings a narrowing closed, even after they were shown and shut on the narrow window', () => {
	const { result, rerender } = viewsOver(false)
	rerender({ below: true })
	act(() => result.current.toggleSidebar())
	act(() => result.current.closeSidebar())

	rerender({ below: false })

	expect(result.current.sidebarOpen).toBe(true)
})

test('flips the settings each time they are toggled', () => {
	const { result } = viewsOver(false)

	act(() => result.current.toggleSidebar())
	expect(result.current.sidebarOpen).toBe(false)

	act(() => result.current.toggleSidebar())
	expect(result.current.sidebarOpen).toBe(true)
})

test('holds the settings still on load, so settings open from the start never slide in', () => {
	const { result } = viewsOver(false)

	expect(result.current.sidebarMoved).toBe(false)
})

test('lets the settings slide once they are toggled', () => {
	const { result } = viewsOver(false)

	act(() => result.current.toggleSidebar())

	expect(result.current.sidebarMoved).toBe(true)
})

test('lets the settings slide once they are closed', () => {
	const { result } = viewsOver(false)

	act(() => result.current.closeSidebar())

	expect(result.current.sidebarMoved).toBe(true)
})

test('lets the settings slide once the window crosses the breakpoint', () => {
	const { result, rerender } = viewsOver(false)

	rerender({ below: true })
	rerender({ below: false })

	expect(result.current.sidebarMoved).toBe(true)
})

test('names the settings panel by an id that holds across renders', () => {
	const { result, rerender } = viewsOver(false)
	const named = result.current.sidebarId

	rerender({ below: true })

	expect(named).toEqual(expect.stringMatching(/\S/))
	expect(result.current.sidebarId).toBe(named)
})

test('gives each preview width a size to render at', () => {
	const styles = Array.from(document.styleSheets)
		.flatMap((sheet) => Array.from(sheet.cssRules ?? []))
		.map((rule) => rule.cssText)
		.join('\n')

	expect(styles).toMatch(/canvas--tablet[^}]*max-width/)
	expect(styles).toMatch(/canvas--mobile[^}]*max-width/)
})
