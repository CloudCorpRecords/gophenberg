// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { cutTarget, statementStarts, targetFile } from '../../scripts/pieces.ts'

test('gives the line of every top-level statement after the first', () => {
	const source = ['const a = 1', '', 'function b() {', '\treturn a', '}', 'export const c = b()'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.ts')).toEqual([3, 6])
})

test('gives the line a statement starts on below the doc comment above it', () => {
	const source = ['const a = 1', '/** The b. */', 'const b = 2'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.ts')).toEqual([3])
})

test('skips a statement that starts on the line the statement before it ends on', () => {
	const source = ['const a = [', '\t1,', ']; const b = 2', 'const c = 3'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.ts')).toEqual([4])
})

test('leaves nested statements out', () => {
	const source = ['function a() {', '\tconst b = 1', '\treturn b', '}'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.ts')).toEqual([])
})

test('keeps no cut inside a template string a line at the left edge would fool', () => {
	const source = ['const a = `one', 'const b = 2`', 'const c = 3'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.ts')).toEqual([3])
})

test('reads markup in a TSX file', () => {
	const source = ['const a = <p>one</p>', 'const b = <p>two</p>'].join('\n')

	expect(statementStarts(source, 'frontend/src/a.tsx')).toEqual([2])
})

test('cuts a range at every statement start inside it', () => {
	expect(cutTarget('frontend/src/a.ts:10-30', [5, 12, 20, 31])).toEqual([
		'frontend/src/a.ts:10-11',
		'frontend/src/a.ts:12-19',
		'frontend/src/a.ts:20-30',
	])
})

test('keeps a range whole when no statement starts inside it', () => {
	expect(cutTarget('frontend/src/a.ts:10-12', [9, 13])).toEqual(['frontend/src/a.ts:10-12'])
})

test('makes no empty piece for a statement starting on the first line of the range', () => {
	expect(cutTarget('frontend/src/a.ts:10-12', [10, 12])).toEqual(['frontend/src/a.ts:10-11', 'frontend/src/a.ts:12-12'])
})

test('leaves a whole file uncut', () => {
	expect(cutTarget('frontend/src/new.ts', [3, 8])).toEqual(['frontend/src/new.ts'])
})

test('keeps a colon in the file name when it cuts', () => {
	expect(cutTarget('frontend/src/c:1-2.ts:4-6', [5])).toEqual(['frontend/src/c:1-2.ts:4-4', 'frontend/src/c:1-2.ts:5-6'])
})

test('names the file of a range', () => {
	expect(targetFile('frontend/src/a.ts:10-12')).toBe('frontend/src/a.ts')
})

test('names a whole file as itself', () => {
	expect(targetFile('frontend/src/new.ts')).toBe('frontend/src/new.ts')
})
