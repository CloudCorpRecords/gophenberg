// SPDX-License-Identifier: Apache-2.0

import { ACCESS_ADMIN } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { createAuthQueryClient, sessionQueryKey } from '@gopherium/react-auth'
import type { User } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClientProvider, useQuery } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { AdminGate } from '../AdminGate'

/**
 * Renders what the admin reads first once it opens.
 * @returns The admin stand-in.
 */
function TheAdmin() {
	useQuery({ queryKey: ['settings'], queryFn: () => fetch('/api/settings').then((answer) => answer.json()) })
	return <p>the admin</p>
}

/**
 * Renders the gate over the admin stand-in with the session signed in.
 * @param session - The signed in account.
 * @returns The query client behind the gate.
 */
function renderGate(session: User) {
	const client = createAuthQueryClient({ queries: { retry: false, staleTime: Infinity } })
	seedSession(client, session)
	render(
		<QueryClientProvider client={client}>
			<AdminGate>
				<TheAdmin />
			</AdminGate>
		</QueryClientProvider>,
	)
	return client
}

/** A member of the site, who never enters the admin. */
const member: User = { ...defaultUser, role: 'member', capabilities: [] }

test('opens the admin for a session holding the admin capability', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({})))

	renderGate({ ...defaultUser, role: 'author', capabilities: [ACCESS_ADMIN] })

	expect(await screen.findByText('the admin')).toBeInTheDocument()
})

test('turns a member away with the WordPress words, a link to the site and a way out', async () => {
	renderGate(member)

	expect(await screen.findByText('Sorry, you are not allowed to access this page.')).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'Visit Site' })).toHaveAttribute('href', '/')
	expect(screen.getByRole('button', { name: 'Log out' })).toBeInTheDocument()
	expect(screen.queryByText('the admin')).toBeNull()
})

test('turns away a role named admin that carries no capability', async () => {
	renderGate({ ...defaultUser, role: 'admin', capabilities: [] })

	expect(await screen.findByText('Sorry, you are not allowed to access this page.')).toBeInTheDocument()
})

test('reads nothing the admin reads for a member', async () => {
	const asked: string[] = []
	server.use(
		http.get('/api/settings', () => {
			asked.push('settings')
			return HttpResponse.json({})
		}),
	)

	renderGate(member)
	await screen.findByText('Sorry, you are not allowed to access this page.')

	expect(asked).toEqual([])
})

test('signs a member out from the screen turning it away', async () => {
	server.use(http.post('/api/auth/logout', () => new HttpResponse(null, { status: 204 })))
	const client = renderGate(member)

	await userEvent.click(await screen.findByRole('button', { name: 'Log out' }))

	await waitFor(() => expect(client.getQueryData(sessionQueryKey)).toBeNull())
})
