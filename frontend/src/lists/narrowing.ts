// SPDX-License-Identifier: Apache-2.0

import type { Filter } from '@gophenberg/frontend-sdk/dataviews'

/** Returns the part of a listing request a filter of one column narrows it by. */
export type Narrowing<Terms> = (filter: Filter) => Terms

/**
 * Reports whether a filter value is a moment the server reads, one falling in a year of four digits.
 * @param value - The value the filter holds.
 * @returns Whether the value is text a date can be read from, in the years 0 to 9999.
 */
export function isMoment(value: unknown): boolean {
	const year = typeof value === 'string' ? new Date(value).getUTCFullYear() : Number.NaN
	return year >= 0 && year <= 9999
}

/**
 * Returns the narrowing of a date filter, writing its moment under the request key of its operator.
 * @param before - The request key a before filter writes.
 * @param after - The request key an after filter writes.
 * @returns The narrowing.
 */
export function dateNarrowing<Key extends string>(before: Key, after: Key): Narrowing<Partial<Record<Key, string>>> {
	return (filter) => {
		const terms: Partial<Record<Key, string>> = {}
		terms[filter.operator === 'before' ? before : after] = new Date(String(filter.value)).toISOString()
		return terms
	}
}

/**
 * Returns the part of a listing request the filters holding a value narrow it by.
 * @param filters - The filters the view carries, each one the list offers.
 * @param narrowings - The narrowing of each column a filter may name, keyed by the column.
 * @returns What the narrowings wrote, merged in the order the filters come.
 */
export function narrowedBy<Terms extends object>(
	filters: Filter[],
	narrowings: Record<string, Narrowing<Terms> | undefined>,
): Partial<Terms> {
	let narrowed: Partial<Terms> = {}
	for (const filter of filters) {
		const narrow = narrowings[filter.field]
		if (narrow !== undefined && filter.value !== undefined) {
			narrowed = { ...narrowed, ...narrow(filter) }
		}
	}
	return narrowed
}
