// SPDX-License-Identifier: Apache-2.0

import type { BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, _x, sprintf } from '@wordpress/i18n'

import { deleteQuestion, itemWords } from '../lists/itemWords'
import type { NameCut } from '../lists/itemWords'
import { pluralOf } from './typeText'
import type { ContentType } from './types'

/**
 * Returns the words a switch on or off over types reads as.
 * @param name - Cuts a name to the length a toast shows.
 * @param on - Whether the switch turns the types on.
 * @returns The words for the types switched and for the ones that stayed.
 */
export function switchWords(name: NameCut, on: boolean): BulkWords<ContentType> {
	return itemWords(name, pluralOf, {
		named: (type) =>
			sprintf(on ? __('"%s" activated.', 'gophenberg') : __('"%s" deactivated.', 'gophenberg'), type),
		finished: (count) =>
			sprintf(
				on
					? _n('%s content type activated.', '%s content types activated.', count, 'gophenberg')
					: _n('%s content type deactivated.', '%s content types deactivated.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				on
					? _n('%s content type could not be activated.', '%s content types could not be activated.', count, 'gophenberg')
					: _n(
							'%s content type could not be deactivated.',
							'%s content types could not be deactivated.',
							count,
							'gophenberg',
						),
				formatNumber(count),
			),
	})
}

/**
 * Returns the words deleting types for good reads as, counting content types where the posts list counts items.
 * @param name - Cuts a name to the length a toast shows.
 * @returns The words for the types deleted and for the ones that stayed.
 */
export function removalWords(name: NameCut): BulkWords<ContentType> {
	return itemWords(name, pluralOf, {
		named: (type) => sprintf(_x('"%s" permanently deleted.', 'content type', 'gophenberg'), type),
		finished: (count) =>
			sprintf(
				_n('%s content type permanently deleted.', '%s content types permanently deleted.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				_n(
					'%s content type could not be permanently deleted.',
					'%s content types could not be permanently deleted.',
					count,
					'gophenberg',
				),
				formatNumber(count),
			),
	})
}

/**
 * Returns the question asked before types are deleted for good.
 * @param types - The types picked.
 * @param name - Cuts a name to the length a toast shows.
 * @returns The question naming the type when only one was picked, else counting content types.
 */
export function removalQuestion(types: ContentType[], name: NameCut): string {
	if (types.length === 1) {
		return deleteQuestion(types, name, pluralOf)
	}
	const many = _n(
		'Delete %s content type for good? This cannot be undone.',
		'Delete %s content types for good? This cannot be undone.',
		types.length,
		'gophenberg',
	)
	return sprintf(many, formatNumber(types.length))
}
