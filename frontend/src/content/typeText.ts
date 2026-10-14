// SPDX-License-Identifier: Apache-2.0

import { __, _x } from '@wordpress/i18n'

/** A text of a content type the admin shows. */
export type TypeField = 'singular' | 'plural' | 'description'

/** One text Gophenberg ships for a built-in type, its English beside the call reading it in the reader's language. */
interface Shipped {
	english: string
	read: () => string
}

/** The texts Gophenberg ships for the types it registers itself, by type key and field. */
const SHIPPED = new Map<string, Record<TypeField, Shipped>>([
	[
		'post',
		{
			singular: { english: 'Post', read: () => _x('Post', 'post type singular name', 'gophenberg') },
			plural: { english: 'Posts', read: () => _x('Posts', 'post type general name', 'gophenberg') },
			description: {
				english: 'Manage the posts on this site.',
				read: () => __('Manage the posts on this site.', 'gophenberg'),
			},
		},
	],
	[
		'page',
		{
			singular: { english: 'Page', read: () => _x('Page', 'post type singular name', 'gophenberg') },
			plural: { english: 'Pages', read: () => _x('Pages', 'post type general name', 'gophenberg') },
			description: {
				english: 'Manage the pages on this site.',
				read: () => __('Manage the pages on this site.', 'gophenberg'),
			},
		},
	],
	[
		'category',
		{
			singular: { english: 'Category', read: () => _x('Category', 'taxonomy singular name', 'gophenberg') },
			plural: { english: 'Categories', read: () => _x('Categories', 'taxonomy general name', 'gophenberg') },
			description: {
				english: 'Manage the categories on this site.',
				read: () => __('Manage the categories on this site.', 'gophenberg'),
			},
		},
	],
])

/** What a type is known by: its key, and the plugin that declared it when one did. */
interface TypeIdentity {
	key: string
	origin?: string
}

/**
 * Returns a type's text in the reader's language while it holds the shipped English and no plugin declared it.
 * @param type - The type the text belongs to.
 * @param field - Which text of the type it is.
 * @param stored - The text the type holds.
 * @returns The catalogue's text for a shipped English, the stored text otherwise.
 */
export function typeText(type: TypeIdentity, field: TypeField, stored: string): string {
	if ((type.origin ?? '') !== '') {
		return stored
	}
	const shipped = SHIPPED.get(type.key)?.[field]
	return shipped?.english === stored ? shipped.read() : stored
}

/**
 * Returns the plural name of a type as the reader reads it.
 * @param named - The type and the plural name it holds.
 * @returns The name to show.
 */
export function pluralOf(named: TypeIdentity & { pluralLabel: string }): string {
	return typeText(named, 'plural', named.pluralLabel)
}

/**
 * Returns the singular name of a type as the reader reads it.
 * @param named - The type and the singular name it holds.
 * @returns The name to show.
 */
export function singularOf(named: TypeIdentity & { singularLabel: string }): string {
	return typeText(named, 'singular', named.singularLabel)
}
