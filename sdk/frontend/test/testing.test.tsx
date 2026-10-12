// SPDX-License-Identifier: Apache-2.0

import { defaultUser } from '@gopherium/react-auth/testing'
import { createRoute } from '@tanstack/react-router'
import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { can, useSession } from '../index'
import type { FrontendPlugin } from '../index'
import { renderPluginAt } from '../testing'

/**
 * Renders whether the signed in account may manage events.
 * @returns The reported reading.
 */
function EventsCanvas() {
	const session = useSession().data
	return <p>{`manages events:${String(can(session, 'events.manage'))}`}</p>
}

const eventsPlugin: FrontendPlugin = {
	id: 'events',
	nav: [],
	routes: (parent) => [
		createRoute({
			getParentRoute: () => parent,
			path: '/events',
			component: EventsCanvas,
		}),
	],
}

const sectionPlugin: FrontendPlugin = {
	id: 'section',
	nav: [
		{
			label: 'Section',
			to: '/section',
			icon: <svg viewBox="0 0 24 24" />,
		},
	],
	routes: (parent) => [
		createRoute({
			getParentRoute: () => parent,
			path: '/section',
			staticData: {
				Sidebar: function SectionSidebar() {
					return <p>section sidebar</p>
				},
			},
			component: function SectionCanvas() {
				return <p>section canvas</p>
			},
		}),
	],
}

test('renders the plugin nav on routes without a section screen', async () => {
	renderPluginAt(sectionPlugin, '/')

	expect(await screen.findByText('Test host home')).toBeInTheDocument()
	expect(
		screen.getByRole('link', { name: 'Section' }),
	).toBeInTheDocument()
})

test('renders the section sidebar and canvas on a section route', async () => {
	renderPluginAt(sectionPlugin, '/section')

	expect(await screen.findByText('section sidebar')).toBeInTheDocument()
	expect(screen.getByText('section canvas')).toBeInTheDocument()
})

test('renders a plugin screen with the session the test seeds', async () => {
	const session = { ...defaultUser, role: 'moderator', capabilities: ['events.manage'] }

	renderPluginAt(eventsPlugin, '/events', { session })

	expect(await screen.findByText('manages events:true')).toBeInTheDocument()
})

test('renders a plugin screen signed out when the test seeds no account', async () => {
	renderPluginAt(eventsPlugin, '/events', { session: null })

	expect(await screen.findByText('manages events:false')).toBeInTheDocument()
})
