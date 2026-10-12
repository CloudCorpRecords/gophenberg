// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import { screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { adminUser, renderAt } from './render'

vi.mock('../plugins', async () => {
	const { createRoute } = await import('@tanstack/react-router')
	const { createElement } = await import('react')
	const plugin = {
		id: 'demo',
		nav: [
			{
				label: 'Demo',
				to: '/demo',
				icon: createElement('svg', { viewBox: '0 0 24 24' }),
			},
			{
				label: 'Demo settings',
				to: '/demo/settings',
				icon: createElement('svg', { viewBox: '0 0 24 24' }),
				capability: 'demo.manage',
			},
		],
		routes: (parent: AnyRoute) => [
			createRoute({
				getParentRoute: () => parent,
				path: '/demo',
				component: function DemoCanvas() {
					return createElement('p', null, 'demo canvas')
				},
			}),
			createRoute({
				getParentRoute: () => parent,
				path: '/demo/settings',
				component: function DemoSettings() {
					return createElement('p', null, 'demo settings')
				},
			}),
		],
	}
	return { plugins: [plugin] }
})

test('mounts plugin routes in the app router', async () => {
	renderAt('/demo')

	expect(await screen.findByText('demo canvas')).toBeInTheDocument()
})

test('renders plugin nav entries in the menu', async () => {
	renderAt('/')

	expect(
		await screen.findByRole('link', { name: 'Demo' }),
	).toBeInTheDocument()
})

test('hides a plugin nav entry from a session lacking the capability it names', async () => {
	renderAt('/', adminUser)

	expect(await screen.findByRole('link', { name: 'Demo' })).toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Demo settings' })).toBeNull()
})

test('shows a plugin nav entry to a session carrying the capability it names', async () => {
	renderAt('/', { ...adminUser, capabilities: ['demo.manage'] })

	expect(await screen.findByRole('link', { name: 'Demo settings' })).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'Demo' })).toBeInTheDocument()
})
