// SPDX-License-Identifier: Apache-2.0

import type { BulkFailure, BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

/** Cuts a title to the length a toast shows. */
export type NameCut = (title: string) => string

/** The sentences a run over items reads as. */
interface RunSentences {
	/** The toast for the one item a run finished, given its cut name. */
	named: (name: string) => string
	/** The toast counting the items a run finished. */
	finished: (count: number) => string
	/** The notice counting the items a run could not finish. */
	unfinished: (count: number) => string
}

/**
 * Returns the reason the one failed call gave, already in the reader's words.
 * @param failures - The failures of a run over one item.
 * @returns The reason.
 */
function reasonOf<Item>(failures: BulkFailure<Item>[]): string {
	return (failures[0].error as Error).message
}

/**
 * Returns the words a run over items reads as.
 * @param name - Cuts a name to the length a toast shows.
 * @param nameOf - Returns the name an item is listed under.
 * @param sentences - The toasts and the notice of the run.
 * @returns The words for the items that finished and for the ones that failed, the reason itself for a run over one.
 */
export function itemWords<Item>(
	name: NameCut,
	nameOf: (item: Item) => string,
	sentences: RunSentences,
): BulkWords<Item> {
	return {
		done: (count, only) => (only === undefined ? sentences.finished(count) : sentences.named(name(nameOf(only)))),
		failed: (failures, asked) => (asked === 1 ? reasonOf(failures) : sentences.unfinished(failures.length)),
	}
}

/**
 * Returns the question asked before the given items are deleted for good.
 * @param items - The items acted on.
 * @param name - Cuts a name to the length a toast shows.
 * @param nameOf - Returns the name an item is listed under.
 * @returns The question naming the item when only one was picked.
 */
export function deleteQuestion<Item>(items: Item[], name: NameCut, nameOf: (item: Item) => string): string {
	if (items.length === 1) {
		const one = __('Delete "%(title)s" for good? This cannot be undone.', 'gophenberg')
		return sprintf(one, { title: name(nameOf(items[0])) })
	}
	const many = _n(
		'Delete %s item for good? This cannot be undone.',
		'Delete %s items for good? This cannot be undone.',
		items.length,
		'gophenberg',
	)
	return sprintf(many, formatNumber(items.length))
}

/**
 * Returns the words a permanent delete over items reads as.
 * @param name - Cuts a name to the length a toast shows.
 * @param nameOf - Returns the name an item is listed under.
 * @returns The words for the items deleted and for the ones that stayed.
 */
export function deleteWords<Item>(name: NameCut, nameOf: (item: Item) => string): BulkWords<Item> {
	return itemWords(name, nameOf, {
		named: (title) => sprintf(__('"%s" permanently deleted.', 'gophenberg'), title),
		finished: (count) =>
			sprintf(
				_n('%s item permanently deleted.', '%s items permanently deleted.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				_n(
					'%s item could not be permanently deleted.',
					'%s items could not be permanently deleted.',
					count,
					'gophenberg',
				),
				formatNumber(count),
			),
	})
}
