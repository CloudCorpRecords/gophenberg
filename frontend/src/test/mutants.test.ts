// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import {
	mutateTargets,
	mutatedFiles,
	parseShard,
	shardCount,
	shardTargets,
	strykerConfig,
	wholeSetting,
} from '../../scripts/mutants.ts'

const PATTERNS = ['frontend/src/**/*.{ts,tsx}', '!**/*.test.{ts,tsx}', '!frontend/src/main.tsx']

const MUTANTS = new Map([
	['frontend/src/a.ts:12-13', 2],
	['frontend/src/b.ts:41-50', 10],
	['frontend/src/c.ts:30-30', 1],
	['frontend/src/d.ts:3-9', 7],
	['frontend/src/e.ts:7-9', 3],
	['frontend/src/f.ts:100-104', 5],
])

const RANGES = [...MUTANTS.keys()]

/**
 * Returns the share of every shard of a run split into the given number of shards.
 * @param targets - The mutate entries to split.
 * @param count - How many shards the run has.
 * @param weight - Weighs a mutate entry, one for any entry the mutant table does not name.
 * @returns The share of each shard, the first shard first.
 */
function sharesOf(targets: string[], count: number, weight = (target: string) => MUTANTS.get(target) ?? 1): string[][] {
	return Array.from({ length: count }, (_, place) => shardTargets(targets, { index: place + 1, count }, weight))
}

/**
 * Returns a diff with no context lines touching one file with the given hunk headers.
 * @param file - The file the diff changes.
 * @param hunks - The hunk headers, as git writes them.
 * @returns The diff text.
 */
function diffOf(file: string, ...hunks: string[]): string {
	const body = hunks.flatMap((hunk) => [hunk, '+changed'])
	return [`diff --git a/${file} b/${file}`, `--- a/${file}`, `+++ b/${file}`, ...body].join('\n')
}

test('names the lines a hunk adds as a range', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -10,0 +11,3 @@ export function Screen() {')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:11-13'])
})

test('names a single changed line as a range of one', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -5 +5 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:5-5'])
})

test('skips a hunk that only removes lines', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -5,2 +4,0 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([])
})

test('keeps every hunk of a file as its own range', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -2 +2 @@', '@@ -20,0 +21,2 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:2-2',
		'frontend/src/users/screen.tsx:21-22',
	])
})

test('mutates a file git does not track yet whole', () => {
	expect(mutateTargets('', ['frontend/src/users/new.tsx', 'notes.md'], PATTERNS)).toEqual([
		'frontend/src/users/new.tsx',
	])
})

test('leaves out the changed lines of a file the patterns do not name', () => {
	const diff = [
		diffOf('frontend/src/test/users.test.tsx', '@@ -3 +3 @@'),
		diffOf('frontend/src/users/screen.tsx', '@@ -7 +7 @@'),
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:7-7'])
})

test('reads an added line starting with plus signs as content, not as a file header', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1,0 +1,2 @@'),
		'+++ b/frontend/src/users/other.tsx',
		'@@ -9 +10 @@',
		'+later',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:1-2',
		'frontend/src/users/screen.tsx:10-10',
	])
})

test('reads a removed line starting with dashes as content, not as a file header', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1 +0,0 @@'),
		'--- a removed line of dashes',
		'@@ -3 +2 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:2-2'])
})

test('reads a removed line and an added line shaped like file headers as content', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1 +1,2 @@'),
		'--- a/frontend/src/users/other.tsx',
		'+++ b/frontend/src/users/other.tsx',
		'@@ -9 +10 @@',
		'+later',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:1-2',
		'frontend/src/users/screen.tsx:10-10',
	])
})

test('reads a hunk header with counts of several digits', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -40,12 +40,15 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:40-54'])
})

test('reads an added line holding hunk header text as content', () => {
	const diff = [diffOf('frontend/src/users/screen.tsx', '@@ -1 +1 @@'), "+const hunk = '@@ -9 +10 @@'"].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:1-1'])
})

test('reads a file name with a space without the tab git ends it with', () => {
	const diff = [
		'diff --git a/frontend/src/users/my screen.tsx b/frontend/src/users/my screen.tsx',
		'--- a/frontend/src/users/my screen.tsx\t',
		'+++ b/frontend/src/users/my screen.tsx\t',
		'@@ -5 +5 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/my screen.tsx:5-5'])
})

test('gives no lines of a file whose name git quotes to the file before it', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -2 +2 @@'),
		'diff --git "a/frontend/src/users/tab\\tname.tsx" "b/frontend/src/users/tab\\tname.tsx"',
		'--- "a/frontend/src/users/tab\\tname.tsx"',
		'+++ "b/frontend/src/users/tab\\tname.tsx"',
		'@@ -7 +7 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:2-2'])
})

test('keeps a changed source file a pattern names', () => {
	expect(mutatedFiles(['frontend/src/users/screen.tsx'], PATTERNS)).toEqual(['frontend/src/users/screen.tsx'])
})

test('drops a changed file no pattern names', () => {
	expect(mutatedFiles(['frontend/vite.config.ts', 'README.md'], PATTERNS)).toEqual([])
})

test('drops a changed file a later negated pattern leaves out', () => {
	expect(mutatedFiles(['frontend/src/test/users.test.tsx', 'frontend/src/main.tsx'], PATTERNS)).toEqual([])
})

test('keeps a file a pattern names again after a negated one', () => {
	const patterns = [...PATTERNS, 'frontend/src/main.tsx']

	expect(mutatedFiles(['frontend/src/main.tsx'], patterns)).toEqual(['frontend/src/main.tsx'])
})

test('keeps the changed order of the files it keeps', () => {
	const changed = ['frontend/src/b.ts', 'frontend/vite.config.ts', 'frontend/src/a.ts']

	expect(mutatedFiles(changed, PATTERNS)).toEqual(['frontend/src/b.ts', 'frontend/src/a.ts'])
})

test('reads the patterns the mutation config holds', () => {
	const changed = ['frontend/src/users/screen.tsx', 'frontend/src/test/users.test.tsx', 'frontend/src/i18n/catalog.ts']

	expect(mutatedFiles(changed, strykerConfig().mutate)).toEqual(['frontend/src/users/screen.tsx'])
})

test('gives the initial test run longer than a mutation job may take', () => {
	expect(strykerConfig().dryRunTimeoutMinutes).toBeGreaterThan(15)
})

test('reads a shard named as its index over the shard count', () => {
	expect(parseShard('2/6')).toEqual({ index: 2, count: 6 })
})

test('reads a shard index and count of several digits', () => {
	expect(parseShard('12/16')).toEqual({ index: 12, count: 16 })
})

test('reads the only shard of a run in one piece', () => {
	expect(parseShard('1/1')).toEqual({ index: 1, count: 1 })
})

test('reads the last shard of a run', () => {
	expect(parseShard('6/6')).toEqual({ index: 6, count: 6 })
})

test.each(['0/6', '7/6', '1/0', '0/0'])('reads no shard from %s, its index outside the count', (text) => {
	expect(parseShard(text)).toBeUndefined()
})

test.each(['', '2', '2/', '/6', 'a/6', 'x2/6', '2/6x', '2/6/1', '-1/6', '1.5/6', ' 2/6'])(
	'reads no shard from %j, not written as index over count',
	(text) => {
		expect(parseShard(text)).toBeUndefined()
	},
)

test('gives one shard for every so many mutants, rounding up', () => {
	expect(shardCount([30, 30, 31], 45, 20)).toBe(3)
})

test('gives one shard to a run holding fewer mutants than a shard tests', () => {
	expect(shardCount([1], 45, 20)).toBe(1)
})

test('gives one shard to a run holding no mutant', () => {
	expect(shardCount([], 45, 20)).toBe(1)
})

test('gives no more shards than a run may use', () => {
	expect(shardCount(Array.from({ length: 100 }, () => 50), 45, 20)).toBe(20)
})

test('gives no more shards than there are pieces to deal', () => {
	expect(shardCount([96], 45, 20)).toBe(1)
})

test('reads a whole number setting the workflow sets', () => {
	expect(wholeSetting({ MAX_SHARDS: '20' }, 'MAX_SHARDS')).toBe(20)
})

test.each([undefined, '', '0', '-3', '1.5', 'x'])('refuses the setting %j, not a whole number from 1', (value) => {
	expect(() => wholeSetting({ MAX_SHARDS: value }, 'MAX_SHARDS')).toThrow('MAX_SHARDS')
})

test.each([1, 2, 3, 4, 6, 8])('gives every target to exactly one shard of a run in %i shards', (count) => {
	expect(sharesOf(RANGES, count).flat().toSorted()).toEqual(RANGES)
})

test('spreads the mutants evenly over two shards', () => {
	expect(sharesOf(RANGES, 2)).toEqual([
		['frontend/src/b.ts:41-50', 'frontend/src/c.ts:30-30', 'frontend/src/e.ts:7-9'],
		['frontend/src/a.ts:12-13', 'frontend/src/d.ts:3-9', 'frontend/src/f.ts:100-104'],
	])
})

test('spreads the mutants evenly over three shards', () => {
	expect(sharesOf(RANGES, 3)).toEqual([
		['frontend/src/b.ts:41-50'],
		['frontend/src/a.ts:12-13', 'frontend/src/d.ts:3-9'],
		['frontend/src/c.ts:30-30', 'frontend/src/e.ts:7-9', 'frontend/src/f.ts:100-104'],
	])
})

test.each<{ name: string; weights: [string, number][]; shares: string[][] }>([
	{
		name: 'a long range holding few mutants below a short range holding many',
		weights: [
			['frontend/src/long.ts:1-90', 2],
			['frontend/src/short.ts:1-3', 9],
			['frontend/src/mid.ts:1-20', 8],
		],
		shares: [['frontend/src/short.ts:1-3'], ['frontend/src/long.ts:1-90', 'frontend/src/mid.ts:1-20']],
	},
	{
		name: 'a whole file by the mutants it holds',
		weights: [
			['frontend/src/a.ts:1-3', 3],
			['frontend/src/b.ts:1-3', 3],
			['frontend/src/new.tsx', 6],
		],
		shares: [['frontend/src/new.tsx'], ['frontend/src/a.ts:1-3', 'frontend/src/b.ts:1-3']],
	},
])('weighs $name', ({ weights, shares }) => {
	const table = new Map(weights)

	expect(sharesOf([...table.keys()], 2, (target) => table.get(target) ?? 0)).toEqual(shares)
})

test('puts targets of the same size in name order', () => {
	expect(sharesOf(['frontend/src/b.ts:1-1', 'frontend/src/a.ts:1-1'], 2)).toEqual([
		['frontend/src/a.ts:1-1'],
		['frontend/src/b.ts:1-1'],
	])
})

test('keeps the given order of the targets in a share', () => {
	expect(sharesOf(RANGES.toReversed(), 2)[0]).toEqual([
		'frontend/src/e.ts:7-9',
		'frontend/src/c.ts:30-30',
		'frontend/src/b.ts:41-50',
	])
})

test('gives the same shares whatever order the targets come in', () => {
	const shares = sharesOf(RANGES.toReversed(), 3).map((share) => share.toSorted())

	expect(shares).toEqual(sharesOf(RANGES, 3))
})

test('leaves a shard empty when the run has fewer targets than shards', () => {
	expect(sharesOf(['frontend/src/a.ts:1-1', 'frontend/src/b.ts:1-1'], 3)).toEqual([
		['frontend/src/a.ts:1-1'],
		['frontend/src/b.ts:1-1'],
		[],
	])
})
