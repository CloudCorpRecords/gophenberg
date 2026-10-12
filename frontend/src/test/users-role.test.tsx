// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { roleLabel, startingRole } from '../users/roles'
import { adminUser, renderAt } from './render'

const SIGNED_IN = '0198b2f0-0000-7000-8000-000000000001'

const OTHER = '0198b2f0-0000-7000-8000-000000000002'

/**
 * Returns a stored account row holding a role.
 * @param id - The account identifier.
 * @param name - The account holder's name.
 * @param role - The role the account holds.
 * @returns The account row.
 */
function withRole(id: string, name: string, role: string) {
	return {
		id,
		email: `${name.split(' ')[0]?.toLowerCase()}@example.com`,
		name,
		disabled: false,
		role,
		created_at: '2026-07-06T10:00:00Z',
	}
}

/**
 * Opens the role dialog of the other account and returns it.
 * @returns The dialog element.
 */
async function openRoleDialog(): Promise<HTMLElement> {
	await userEvent.click(await screen.findByRole('button', { name: 'Change the role of Maria Perez' }))
	return screen.findByRole('dialog', { name: 'Change role' })
}

/**
 * Picks a role in the open role dialog.
 * @param dialog - The role dialog.
 * @param label - The label of the role to pick.
 */
async function pickRole(dialog: HTMLElement, label: string) {
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: label }))
}

/**
 * Answers the next role write with a refusal carrying the code.
 * @param status - The status the refusal answers with.
 * @param code - The error code the refusal names.
 */
function refuseRoleWrite(status: number, code: string) {
	server.use(
		http.put(`/api/users/${OTHER}/role`, () => HttpResponse.json({ error: code.replaceAll('_', ' '), code }, { status })),
	)
}

beforeEach(() =>
	server.use(
		http.get('/api/users', () => HttpResponse.json([withRole(OTHER, 'Maria Perez', 'editor')])),
	),
)

test('shows the role each account holds', async () => {
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Maria Perez/ })

	expect(within(row).getByText('Editor')).toBeInTheDocument()
})

test('offers exactly the roles the session may give, in the order the server sent them', async () => {
	renderAt('/users', { ...adminUser, grantable: ['admin', 'moderator', 'editor', 'author'] })

	const dialog = await openRoleDialog()
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Role' }))
	const offered = (await screen.findAllByRole('option')).map((option) => option.textContent)

	expect(offered).toEqual(['Administrator', 'Moderator', 'Editor', 'Author'])
})

test('offers no role past the reach of the session', async () => {
	renderAt('/users', { ...adminUser, grantable: ['author'] })

	const dialog = await openRoleDialog()
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Role' }))
	const offered = (await screen.findAllByRole('option')).map((option) => option.textContent)

	expect(offered).toEqual(['Author'])
})

test('asks which role the account should hold, and waits for another one to be picked', async () => {
	renderAt('/users')

	const dialog = await openRoleDialog()

	expect(within(dialog).getByText('Choose the role of Maria Perez.')).toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Change role' })).toHaveAttribute('aria-disabled', 'true')
	await pickRole(dialog, 'Administrator')
	expect(within(dialog).getByRole('button', { name: 'Change role' })).not.toHaveAttribute('aria-disabled', 'true')
})

test('keeps the dialog body beside its header so the dialog spacing applies', async () => {
	renderAt('/users')

	const dialog = await openRoleDialog()
	const header = within(dialog).getByRole('banner')
	const footer = within(dialog).getByRole('contentinfo')

	expect(footer.parentElement).toBe(header.parentElement)
	expect(header.nextElementSibling).toContainElement(within(dialog).getByText('Choose the role of Maria Perez.'))
})

test('writes the role the reader picked, says so and closes', async () => {
	const asked: unknown[] = []
	server.use(
		http.put(`/api/users/${OTHER}/role`, async ({ request }) => {
			asked.push(await request.json())
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/users')

	const dialog = await openRoleDialog()
	await pickRole(dialog, 'Administrator')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Change role' }))

	expect(await screen.findByText('Role changed.')).toBeInTheDocument()
	expect(asked).toEqual([{ role: 'admin' }])
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test('closes on cancel without writing', async () => {
	const asked: unknown[] = []
	server.use(
		http.put(`/api/users/${OTHER}/role`, () => {
			asked.push('written')
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/users')

	const dialog = await openRoleDialog()
	await pickRole(dialog, 'Administrator')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
	expect(asked).toEqual([])
})

test('leaves the signed in account without a way to change its own role', async () => {
	server.use(
		http.get('/api/users', () =>
			HttpResponse.json([withRole(SIGNED_IN, 'Grace Hopper', 'admin')]),
		),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Grace Hopper/ })

	expect(within(row).queryByRole('button', { name: /Change the role/ })).toBeNull()
	expect(within(row).getByText('Administrator')).toBeInTheDocument()
})

test.each([
	[422, 'self_role_refused', 'You cannot change your own role.'],
	[422, 'last_privileged_refused', 'Somebody must be able to manage users.'],
	[403, 'role_beyond_reach', 'That role holds more than your own.'],
	[422, 'role_unknown', 'Nothing declares that role.'],
	[403, 'role_insufficient', 'The role could not be changed.'],
	[409, 'role_changed', 'The role could not be changed.'],
	[500, 'internal', 'The role could not be changed.'],
])('explains a role write answered %i %s', async (status, code, words) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	refuseRoleWrite(status, code)
	renderAt('/users')

	const dialog = await openRoleDialog()
	await pickRole(dialog, 'Administrator')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Change role' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(words)
})

test('explains a role past the reach of the session in the reader s language', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	refuseRoleWrite(403, 'role_beyond_reach')
	renderAt('/users')

	await userEvent.click(await screen.findByRole('button', { name: 'Cambiar el rol de Maria Perez' }))
	const dialog = await screen.findByRole('dialog', { name: 'Cambiar el rol' })
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Rol' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Administrador' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cambiar el rol' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent('Ese rol abarca más que el tuyo.')
})

test('creates an account under the role the form picked', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/users', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(withRole(OTHER, 'Maria Perez', 'editor'), { status: 201 })
		}),
	)
	renderAt('/users/new')

	await userEvent.type(await screen.findByLabelText('Email'), 'maria@example.com')
	await userEvent.type(screen.getByLabelText('Name'), 'Maria Perez')
	await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery')
	await userEvent.click(screen.getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Editor' }))
	await userEvent.click(screen.getByRole('button', { name: 'Create user' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ role: 'editor' })
})

test('offers a new account exactly the roles the session may give', async () => {
	renderAt('/users/new', { ...adminUser, grantable: ['moderator', 'author'] })

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role' }))
	const offered = (await screen.findAllByRole('option')).map((option) => option.textContent)

	expect(offered).toEqual(['Moderator', 'Author'])
})

test('starts a new account at the author role', async () => {
	renderAt('/users/new')

	expect(await screen.findByRole('combobox', { name: 'Role' })).toHaveTextContent('Author')
})

test('offers a new account no role when the session names none it may give', async () => {
	renderAt('/users/new', { ...adminUser, grantable: undefined })

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role' }))

	expect(screen.queryAllByRole('option')).toEqual([])
})

test('starts a new account at author when the session names no role it may give', () => {
	expect(startingRole([])).toBe('author')
})

test('starts a new account at the narrowest role the session may give when author is not one', async () => {
	renderAt('/users/new', { ...adminUser, grantable: ['moderator', 'reviewer'] })

	expect(await screen.findByRole('combobox', { name: 'Role' })).toHaveTextContent('Reviewer')
})

test.each([
	[422, 'role_unknown', 'Nothing declares that role.'],
	[403, 'role_beyond_reach', 'That role holds more than your own.'],
])('explains a new account answered %i %s', async (status, code, words) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post('/api/users', () => HttpResponse.json({ error: code.replaceAll('_', ' '), code }, { status })),
	)
	renderAt('/users/new')

	await userEvent.type(await screen.findByLabelText('Email'), 'maria@example.com')
	await userEvent.type(screen.getByLabelText('Name'), 'Maria Perez')
	await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery')
	await userEvent.click(screen.getByRole('button', { name: 'Create user' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(words)
})

test('names an account holding no role', async () => {
	server.use(
		http.get('/api/users', () => HttpResponse.json([withRole(OTHER, 'Maria Perez', '')])),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Maria Perez/ })

	expect(within(row).getByText('No role')).toBeInTheDocument()
})

test('reads a role the screen does not know with a capital first letter', () => {
	expect(roleLabel('archivist')).toBe('Archivist')
	expect(roleLabel('')).toBe('No role')
	expect(roleLabel('editor')).toBe('Editor')
})
