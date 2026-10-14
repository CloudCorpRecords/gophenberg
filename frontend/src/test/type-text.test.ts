// SPDX-License-Identifier: Apache-2.0

import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { afterEach, expect, test } from 'vitest'

import { pluralOf, singularOf, typeText } from '../content/typeText'
import type { TypeField } from '../content/typeText'
import { catalogFor } from '../i18n/catalog'

/** Every text Gophenberg ships for a built-in type, with its Spanish and French catalogue words. */
const SHIPPED: { key: string, field: TypeField, english: string, spanish: string, french: string }[] = [
	{ key: 'post', field: 'singular', english: 'Post', spanish: 'Entrada', french: 'Article' },
	{ key: 'post', field: 'plural', english: 'Posts', spanish: 'Entradas', french: 'Articles' },
	{
		key: 'post',
		field: 'description',
		english: 'Manage the posts on this site.',
		spanish: 'Gestiona las entradas de este sitio.',
		french: 'Gérez les articles de ce site.',
	},
	{ key: 'page', field: 'singular', english: 'Page', spanish: 'Página', french: 'Page' },
	{ key: 'page', field: 'plural', english: 'Pages', spanish: 'Páginas', french: 'Pages' },
	{
		key: 'page',
		field: 'description',
		english: 'Manage the pages on this site.',
		spanish: 'Gestiona las páginas de este sitio.',
		french: 'Gérez les pages de ce site.',
	},
	{ key: 'category', field: 'singular', english: 'Category', spanish: 'Categoría', french: 'Catégorie' },
	{ key: 'category', field: 'plural', english: 'Categories', spanish: 'Categorías', french: 'Catégories' },
	{
		key: 'category',
		field: 'description',
		english: 'Manage the categories on this site.',
		spanish: 'Gestiona las categorías de este sitio.',
		french: 'Gérez les catégories de ce site.',
	},
]

/**
 * Loads the catalogue of a language for the rest of the test.
 * @param locale - The language the reader reads the admin in.
 */
async function readIn(locale: string) {
	setLocaleData(await catalogFor(locale), 'gophenberg')
}

afterEach(() => {
	resetLocaleData({}, 'gophenberg')
})

test.each(SHIPPED)('shows the $field of $key in English to an English reader', ({ key, field, english }) => {
	expect(typeText({ key }, field, english)).toBe(english)
})

test.each(SHIPPED)('shows the $field of $key in Spanish while it holds the shipped English', async (shipped) => {
	await readIn('es-ES')

	expect(typeText({ key: shipped.key }, shipped.field, shipped.english)).toBe(shipped.spanish)
})

test.each(SHIPPED)('shows the $field of $key in French while it holds the shipped English', async (shipped) => {
	await readIn('fr-FR')

	expect(typeText({ key: shipped.key }, shipped.field, shipped.english)).toBe(shipped.french)
})

test('shows a name a person changed exactly as they typed it', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'post' }, 'plural', 'Articles')).toBe('Articles')
})

test('shows a description a person changed exactly as they typed it', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'post' }, 'description', 'Everything the team writes.')).toBe('Everything the team writes.')
})

test('shows the shipped English of another field as typed', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'post' }, 'plural', 'Post')).toBe('Post')
})

test('shows the text of a type Gophenberg does not ship as typed, though it reads like a built-in name', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'guide' }, 'plural', 'Posts')).toBe('Posts')
})

test('shows the text of a key that names no built-in type as typed', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'constructor' }, 'singular', 'Post')).toBe('Post')
})

test('reads the shipped English of a type no plugin declared through the catalogue', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'category', origin: '' }, 'plural', 'Categories')).toBe('Categorías')
})

test('shows the name a plugin gave its type as declared, though it is the shipped English', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'category', origin: 'shop' }, 'plural', 'Categories')).toBe('Categories')
})

test('shows the description a plugin gave its type as declared, though it is the shipped English', async () => {
	await readIn('es-ES')

	expect(typeText({ key: 'page', origin: 'shop' }, 'description', 'Manage the pages on this site.')).toBe(
		'Manage the pages on this site.',
	)
})

test('names a type a plugin declared by both its names as declared', async () => {
	await readIn('es-ES')
	const declared = { key: 'category', singularLabel: 'Category', pluralLabel: 'Categories', origin: 'shop' }

	expect([singularOf(declared), pluralOf(declared)]).toEqual(['Category', 'Categories'])
})
