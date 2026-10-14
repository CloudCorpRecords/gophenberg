// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import type { ComponentProps, ComponentType, ReactElement } from 'react'

import type { Capability } from './capabilities'

export type { CanvasMode } from '@gopherium/godmin'

declare module '@tanstack/react-router' {
	interface StaticDataRouteOption {
		Sidebar?: ComponentType
		capability?: Capability
	}
}

export interface NavItem {
	label: string
	to: string
	icon: ReactElement<ComponentProps<'svg'>>
	capability?: Capability
}

export interface FrontendPlugin {
	id: string
	routes: (parent: AnyRoute) => AnyRoute[]
	nav: NavItem[]
}

export {
	AlertDialog,
	Badge,
	Button,
	Icon,
	IconButton,
	InputControl,
	Link,
	Notice,
	SelectControl,
	Skeleton,
	Stack,
	Tabs,
	Text,
	VisuallyHidden,
} from '@wordpress/ui'
export {
	chevronLeft as backIcon,
	backup as backupIcon,
	chevronDown as downIcon,
	listView as listViewIcon,
	media as mediaIcon,
	post as postIcon,
	redo as redoIcon,
	trash as trashIcon,
	undo as undoIcon,
	chevronUp as upIcon,
	upload as uploadIcon,
} from '@wordpress/icons'
export { Dialog } from './dialog'
export { TextareaControl } from './TextareaControl'
export {
	CheckboxControl,
	DropZone,
	RadioControl,
	RangeControl,
	__experimentalStyleProvider as StyleProvider,
} from '@wordpress/components'
export { speak } from '@wordpress/a11y'
export {
	ACCESS_ADMIN,
	ADMIN,
	AUTHOR,
	CHANGE_OTHERS_WORK,
	EDITOR,
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	MEMBER,
	sessionMayChange,
} from './capabilities'
export { can, useSession } from '@gopherium/react-auth'
export type { User as Session } from '@gopherium/react-auth'
