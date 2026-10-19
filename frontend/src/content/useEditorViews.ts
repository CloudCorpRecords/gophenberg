// SPDX-License-Identifier: Apache-2.0

import { useCallback, useId, useState } from 'react'

import type { PreviewWidth } from './editorSetup'

export interface EditorViews {
	listOpen: boolean
	toggleList: () => void
	preview: PreviewWidth
	setPreview: (width: PreviewWidth) => void
	sidebarOpen: boolean
	sidebarMoved: boolean
	toggleSidebar: () => void
	closeSidebar: () => void
	sidebarId: string
}

/** Whether the sidebar shows, whether a narrowing closed it, the window it fits, and whether it moved since load. */
interface SidebarState {
	open: boolean
	reopen: boolean
	narrow: boolean
	moved: boolean
}

/**
 * Returns the sidebar once the window crosses the editor breakpoint, closed on narrowing and reopened on widening.
 * @param state - The sidebar before the crossing.
 * @param narrow - Whether the window is now below the breakpoint.
 * @returns The sidebar after the crossing.
 */
function crossed(state: SidebarState, narrow: boolean): SidebarState {
	if (narrow) {
		return { open: false, reopen: state.open, narrow, moved: true }
	}
	return { open: state.open || state.reopen, reopen: false, narrow, moved: true }
}

/**
 * Holds the views the editor chrome offers over the post being edited.
 * @param narrow - Whether the window is below the editor breakpoint.
 * @returns The state of the block list, of the preview width and of the settings sidebar.
 */
export function useEditorViews(narrow: boolean): EditorViews {
	const [listOpen, setListOpen] = useState(false)
	const [preview, setPreview] = useState<PreviewWidth>('desktop')
	const [sidebar, setSidebar] = useState<SidebarState>({ open: !narrow, reopen: false, narrow, moved: false })
	const sidebarId = useId()
	if (sidebar.narrow !== narrow) {
		setSidebar(crossed(sidebar, narrow))
	}
	const toggleList = useCallback(() => setListOpen((open) => !open), [])
	const toggleSidebar = useCallback(() => setSidebar((held) => ({ ...held, open: !held.open, moved: true })), [])
	const closeSidebar = useCallback(() => setSidebar((held) => ({ ...held, open: false, moved: true })), [])
	return {
		listOpen,
		toggleList,
		preview,
		setPreview,
		sidebarOpen: sidebar.open,
		sidebarMoved: sidebar.moved,
		toggleSidebar,
		closeSidebar,
		sidebarId,
	}
}
