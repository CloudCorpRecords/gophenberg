// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { countMutants, instrumenterOptions } from '../../scripts/mutantCount.ts'
import { cutTarget, statementStarts } from '../../scripts/pieces.ts'

const FILE = 'frontend/src/above.ts'

const SOURCE = [
	'export function above(value: number) {',
	'\treturn value > 1',
	'}',
	'',
	"export const greeting = __('Hello', 'gophenberg')",
	'',
	'export type Name = string',
].join('\n')

const OTHER = 'frontend/src/other.ts'

const WITH_DOMAIN = instrumenterOptions({ ignorers: ['text-domain'] })

const sourceOf = () => SOURCE

const readEither = (file: string) => (file === OTHER ? 'export const b = !a' : SOURCE)

test('counts the mutants Stryker makes inside each range', async () => {
	const counts = await countMutants([`${FILE}:1-3`, `${FILE}:5-5`], sourceOf, WITH_DOMAIN)

	expect(Object.fromEntries(counts)).toEqual({ [`${FILE}:1-3`]: 5, [`${FILE}:5-5`]: 1 })
})

test('leaves out the text domain argument the config ignores', async () => {
	const counts = await countMutants([`${FILE}:5-5`], sourceOf, instrumenterOptions({}))

	expect(counts.get(`${FILE}:5-5`)).toBe(2)
})

test('counts a whole file over all its lines', async () => {
	const counts = await countMutants([FILE], sourceOf, WITH_DOMAIN)

	expect(counts.get(FILE)).toBe(6)
})

test('counts no mutant in a range holding only a type', async () => {
	const counts = await countMutants([`${FILE}:7-7`], sourceOf, WITH_DOMAIN)

	expect(counts.get(`${FILE}:7-7`)).toBe(0)
})

test('loses no mutant when a range is cut at its statement starts', async () => {
	const pieces = cutTarget(`${FILE}:1-7`, statementStarts(SOURCE, FILE))
	const whole = await countMutants([`${FILE}:1-7`], sourceOf, WITH_DOMAIN)
	const cut = await countMutants(pieces, sourceOf, WITH_DOMAIN)

	expect(pieces).toHaveLength(3)
	expect([...cut.values()].reduce((sum, count) => sum + count, 0)).toBe(whole.get(`${FILE}:1-7`))
})

test('counts the ranges of several files apart', async () => {
	const counts = await countMutants([`${FILE}:1-3`, `${OTHER}:1-1`], readEither, WITH_DOMAIN)

	expect(Object.fromEntries(counts)).toEqual({ [`${FILE}:1-3`]: 5, [`${OTHER}:1-1`]: 1 })
})

test('counts a range and the whole of another file in one call', async () => {
	const counts = await countMutants([`${FILE}:1-3`, OTHER], readEither, WITH_DOMAIN)

	expect(Object.fromEntries(counts)).toEqual({ [`${FILE}:1-3`]: 5, [OTHER]: 1 })
})

test('leaves out the mutators the config excludes', async () => {
	const mutator = { excludedMutations: ['EqualityOperator'] }
	const options = instrumenterOptions({ ignorers: ['text-domain'], mutator })

	const counts = await countMutants([`${FILE}:1-3`], sourceOf, options)

	expect(counts.get(`${FILE}:1-3`)).toBe(3)
})

test('refuses a config naming an ignorer no local plugin declares', () => {
	expect(() => instrumenterOptions({ ignorers: ['nowhere'] })).toThrow('the ignorer nowhere')
})

test('reads the parser plugins the config names', () => {
	expect(instrumenterOptions({ mutator: { plugins: ['jsx'] } }).plugins).toEqual(['jsx'])
})

test('uses the instrumenter defaults when the config sets no mutator', () => {
	expect(instrumenterOptions({})).toEqual({ plugins: null, excludedMutations: [], ignorers: [] })
})
