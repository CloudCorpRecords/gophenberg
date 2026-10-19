// SPDX-License-Identifier: Apache-2.0

import { EDGE_BREAKPOINT, SHEET_BREAKPOINT, TITLE_BREAKPOINT, useMediaQuery } from '@gopherium/godmin'

export interface EditorWidth {
	narrow: boolean
	iconSave: boolean
	shortPill: boolean
	noPill: boolean
}

/**
 * Reads which of the WordPress editor breakpoints the window sits below.
 * @returns Whether the editor is narrow, saves through an icon, shortens its document pill or drops it.
 */
export function useEditorWidth(): EditorWidth {
	return {
		narrow: useMediaQuery(`(max-width: ${EDGE_BREAKPOINT - 1}px)`),
		iconSave: useMediaQuery(`(max-width: ${SHEET_BREAKPOINT - 1}px)`),
		shortPill: useMediaQuery(`(max-width: ${SHEET_BREAKPOINT}px)`),
		noPill: useMediaQuery(`(max-width: ${TITLE_BREAKPOINT - 1}px)`),
	}
}
