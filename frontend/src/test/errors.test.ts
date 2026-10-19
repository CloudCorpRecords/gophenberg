// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { execFileSync } from 'node:child_process'
import { join } from 'node:path'

import { errorText } from '../i18n/errors'
import { errorTemplates, updateTemplates } from '../i18n/errorTemplates'
import { repositoryRoot } from '../../scripts/config.ts'

test('reads the message a code stands for', () => {
	const held = errorText({ error: 'content: type still holds content', code: 'type_in_use' })

	expect(held).not.toBe('content: type still holds content')
	expect(held).toMatch(/content/i)
})

test('puts the data an error carries into its message', () => {
	const held = errorText({
		error: 'content: unknown field: colour',
		code: 'field_unknown',
		meta: { field: 'colour' },
	})

	expect(held).toContain('colour')
})

test('names the capability a refused request lacks', () => {
	const held = errorText({
		error: 'capability missing',
		code: 'capability_missing',
		meta: { capability: 'manage_themes' },
	})

	expect(held).toBe('Your role does not allow manage_themes.')
})

test('asks a reader to wait after an update check run moments ago', () => {
	const held = errorText({ error: 'the last check is too recent, so try again later', code: 'check_too_soon' })

	expect(held).toBe('Gophenberg checked for updates moments ago. Try again later.')
})

test('points a reader at an address that holds nothing back to the link, never to a screen the admin lacks', () => {
	const held = errorText({ error: 'not found', code: 'route_not_found' })

	expect(held).toBe('There is nothing at this address. Check the link and try again.')
})

test('falls back to the server message for a code it does not know', () => {
	const held = errorText({ error: 'content: something new', code: 'a_code_from_the_future' })

	expect(held).toBe('content: something new')
})

test('falls back to the server message when no code rides at all', () => {
	expect(errorText({ error: 'the server answered 500' })).toBe('the server answered 500')
})

test('answers something readable when the server said nothing', () => {
	expect(errorText({ error: '' })).not.toBe('')
})

test('leaves a placeholder alone when the data it names is absent', () => {
	const held = errorText({ error: 'content: unknown field: colour', code: 'field_unknown' })

	expect(held).toBe('content: unknown field: colour')
})

test('names the count a gallery bound refuses with', () => {
	const fewest = errorText({ error: '', code: 'field_items_min', meta: { field: 'Gallery', limit: 2 } })
	const most = errorText({ error: '', code: 'field_items_max', meta: { field: 'Gallery', limit: 4 } })

	expect(fewest).toContain('Gallery')
	expect(fewest).toContain('2')
	expect(most).toContain('4')
})

test('writes a number an error carries in the site format', () => {
	const held = errorText({ error: '', code: 'per_page_invalid', meta: { value: '5000', max: 1000 } })

	expect(held).toContain('between 1 and 1.000 items, so 5000 is not')
})

test('writes a limit an error carries with every decimal it holds', () => {
	const held = errorText({ error: '', code: 'field_min', meta: { field: 'Rate', limit: 0.0001 } })

	expect(held).toContain('0,0001')
})

test('names the reason an update could not start', () => {
	const held = errorText({
		error: 'refused',
		code: 'update_refused',
		meta: { reason: 'Gophenberg 0.21.0 needs PostgreSQL 16 or newer' },
	})

	expect(held).toBe('Gophenberg cannot start this update: Gophenberg 0.21.0 needs PostgreSQL 16 or newer')
})

test('carries a message for every code the server can answer with', () => {
	const script = join(repositoryRoot(), 'frontend', 'scripts', 'emittedCodes.sh')
	const emitted = execFileSync('sh', [script], { cwd: repositoryRoot(), encoding: 'utf8' })
		.split('\n')
		.filter((held) => held !== '')
	const held = { ...errorTemplates(), ...updateTemplates() }

	expect(emitted.filter((code) => held[code] === undefined)).toEqual([])
})

test('names the label a root hand-over trips on', () => {
	const held = errorText({
		error: 'content: route word reserved: the label "Page" would become the route word "page"',
		code: 'route_word_reserved_by_label',
		meta: { label: 'Page', word: 'page' },
	})

	expect(held).toContain('Page')
	expect(held).toContain('page')
	expect(held).toMatch(/plural name/i)
})

test('names the address a root hand-over trips on', () => {
	const held = errorText({
		error: 'content: the address is reserved: "page/2"',
		code: 'pagination_address_reserved',
		meta: { path: 'page/2' },
	})

	expect(held).toContain('page/2')
	expect(held).toMatch(/slug/i)
})
