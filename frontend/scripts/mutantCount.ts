// SPDX-License-Identifier: Apache-2.0

import { type File, Instrumenter, type InstrumenterOptions } from '@stryker-mutator/instrumenter'

import { type Piece, pieceOf } from './pieces.ts'
import { strykerPlugins } from './textDomain.ts'

/**
 * Drops a log message.
 * @returns Nothing.
 */
const drop = (): undefined => undefined

/**
 * Answers that a log level is off.
 * @returns False.
 */
const off = (): boolean => false

/** A logger that drops every message. */
const QUIET: ConstructorParameters<typeof Instrumenter>[0] = {
	isTraceEnabled: off,
	isDebugEnabled: off,
	isInfoEnabled: off,
	isWarnEnabled: off,
	isErrorEnabled: off,
	isFatalEnabled: off,
	trace: drop,
	debug: drop,
	info: drop,
	warn: drop,
	error: drop,
	fatal: drop,
}

/** The part of the Stryker config that shapes which mutants a run makes. */
export interface MutationConfig {
	ignorers?: string[]
	mutator?: { plugins?: unknown[] | null; excludedMutations?: string[] }
}

/**
 * Returns the instrumenter options a Stryker run with the config uses.
 * @param config - The Stryker config.
 * @returns The options, holding every ignorer the config names.
 */
export function instrumenterOptions(config: MutationConfig): InstrumenterOptions {
	const ignorers = (config.ignorers ?? []).map((name) => {
		const plugin = strykerPlugins.find((found) => found.kind === 'Ignore' && found.name === name)
		if (plugin === undefined) {
			throw new Error(`the mutation config names the ignorer ${name}, which no local plugin declares`)
		}
		return plugin.value
	})
	return {
		plugins: config.mutator?.plugins ?? null,
		excludedMutations: config.mutator?.excludedMutations ?? [],
		ignorers,
	}
}

/**
 * Returns how many mutants Stryker tests in each mutate entry, leaving out the ones an ignorer drops.
 * @param targets - The mutate entries, as file:start-end or a whole file.
 * @param sourceOf - Reads the text of a file.
 * @param options - The options the Stryker run instruments with.
 * @returns The mutant count of each entry, by entry.
 */
export async function countMutants(
	targets: string[],
	sourceOf: (file: string) => string,
	options: InstrumenterOptions,
): Promise<Map<string, number>> {
	const pieces = targets.map(pieceOf)
	const files = new Map<string, Piece[]>()
	for (const piece of pieces) {
		files.set(piece.file, [...(files.get(piece.file) ?? []), piece])
	}
	const { mutants } = await new Instrumenter(QUIET).instrument(
		[...files].map(([name, held]) => ({ name, content: sourceOf(name), mutate: mutateOf(held) })),
		options,
	)
	const tested = mutants.filter((mutant) => mutant.status !== 'Ignored')
	return new Map(
		pieces.map((piece) => [
			piece.target,
			tested.filter((mutant) => mutant.fileName === piece.file && holds(piece, mutant.location.start.line + 1))
				.length,
		]),
	)
}

/**
 * Returns the mutate description Stryker builds for the pieces of one file, its lines counted from 0.
 * @param pieces - The pieces of the file.
 * @returns True when a piece is the whole file, otherwise every range from its first column to its last line end.
 */
function mutateOf(pieces: Piece[]): File['mutate'] {
	const ranges = pieces.flatMap(({ lines }) =>
		lines === undefined
			? []
			: [{ start: { line: lines.first - 1, column: 0 }, end: { line: lines.last - 1, column: Number.MAX_SAFE_INTEGER } }],
	)
	return ranges.length === pieces.length ? ranges : true
}

/**
 * Reports whether a piece holds a line.
 * @param piece - The piece.
 * @param line - The line, counted from 1.
 * @returns Whether the line sits inside the piece, always so for a whole file.
 */
function holds(piece: Piece, line: number): boolean {
	return piece.lines === undefined || (line >= piece.lines.first && line <= piece.lines.last)
}
