// SPDX-License-Identifier: Apache-2.0

import { EDGE_BREAKPOINT, SHEET_BREAKPOINT, TITLE_BREAKPOINT } from '@gopherium/godmin'
import { setViewport } from '@gopherium/godmin/testing'
import { act, renderHook } from '@testing-library/react'
import { expect, onTestFinished, test, vi } from 'vitest'

import { useEditorWidth } from '../content/useEditorWidth'

/** What the hook reads at each width either side of a breakpoint. */
const BANDS = [
	{ width: 403, narrow: true, iconSave: true, shortPill: true, noPill: true },
	{ width: 404, narrow: true, iconSave: true, shortPill: true, noPill: false },
	{ width: 599, narrow: true, iconSave: true, shortPill: true, noPill: false },
	{ width: 600, narrow: true, iconSave: false, shortPill: true, noPill: false },
	{ width: 601, narrow: true, iconSave: false, shortPill: false, noPill: false },
	{ width: 781, narrow: true, iconSave: false, shortPill: false, noPill: false },
	{ width: 782, narrow: false, iconSave: false, shortPill: false, noPill: false },
]

test('reads the window as a desktop by default', () => {
	const { result } = renderHook(() => useEditorWidth())

	expect(result.current).toEqual({ narrow: false, iconSave: false, shortPill: false, noPill: false })
})

test('reads the window as the narrowest phone when every query matches', () => {
	setViewport({ matches: true })

	const { result } = renderHook(() => useEditorWidth())

	expect(result.current).toEqual({ narrow: true, iconSave: true, shortPill: true, noPill: true })
})

test('asks the queries the godmin breakpoints build', () => {
	const asked = vi.spyOn(window, 'matchMedia')
	onTestFinished(() => asked.mockRestore())

	renderHook(() => useEditorWidth())

	expect(new Set(asked.mock.calls.map(([query]) => query))).toEqual(
		new Set([
			`(max-width: ${EDGE_BREAKPOINT - 1}px)`,
			`(max-width: ${SHEET_BREAKPOINT - 1}px)`,
			`(max-width: ${SHEET_BREAKPOINT}px)`,
			`(max-width: ${TITLE_BREAKPOINT - 1}px)`,
		]),
	)
})

test.each(BANDS)('reads a window $width pixels wide', ({ width, ...flags }) => {
	setViewport({ width })

	const { result } = renderHook(() => useEditorWidth())

	expect(result.current).toEqual(flags)
})

test('follows the window as it narrows past the editor breakpoint', () => {
	setViewport({ width: 1280 })
	const { result } = renderHook(() => useEditorWidth())

	act(() => setViewport({ width: 390 }))

	expect(result.current.narrow).toBe(true)
})
