// SPDX-License-Identifier: Apache-2.0

import ts from 'typescript'

/** A mutate entry naming a range, capturing its first and last line. */
export const RANGE = /:(\d+)-(\d+)$/

/**
 * Returns the file a mutate entry names.
 * @param target - The mutate entry, as file:start-end or a whole file.
 * @returns The file, relative to the repository root.
 */
export function targetFile(target: string): string {
	return target.replace(RANGE, '')
}

/**
 * Returns the lines where a top-level statement starts below the line the statement before it ends on.
 * @param source - The text of the file.
 * @param fileName - The file name, whose extension picks the TypeScript or TSX grammar.
 * @returns The lines, counted from 1, in file order.
 */
export function statementStarts(source: string, fileName: string): number[] {
	const file = ts.createSourceFile(fileName, source, ts.ScriptTarget.Latest, true)
	const lineOf = (position: number) => file.getLineAndCharacterOfPosition(position).line + 1
	return file.statements.slice(1).flatMap((statement, place) => {
		const start = lineOf(statement.getStart(file))
		return start > lineOf(file.statements[place].getEnd()) ? [start] : []
	})
}

/**
 * Returns a mutate entry cut into pieces at every statement start inside its range.
 * @param target - The mutate entry, as file:start-end or a whole file.
 * @param starts - The lines where a top-level statement of the file starts.
 * @returns The pieces in line order, or the entry itself when it names no range.
 */
export function cutTarget(target: string, starts: number[]): string[] {
	const range = RANGE.exec(target)
	if (range === null) {
		return [target]
	}
	const file = targetFile(target)
	const first = Number(range[1])
	const last = Number(range[2])
	const bounds = [first, ...starts.filter((line) => line > first && line <= last)]
	return bounds.map((start, place) => `${file}:${start}-${(bounds[place + 1] ?? last + 1) - 1}`)
}
