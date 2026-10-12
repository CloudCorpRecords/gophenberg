// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join, matchesGlob } from 'node:path'

import { repositoryRoot } from './config.ts'
import type { MutationConfig } from './mutantCount.ts'

/**
 * Returns the changed files a mutation run over the patterns would mutate.
 * @param changed - The changed files, relative to the repository root.
 * @param patterns - The mutate patterns in order, a leading ! leaving files out.
 * @returns The changed files the patterns name, in their changed order.
 */
export function mutatedFiles(changed: string[], patterns: string[]): string[] {
	return changed.filter((file) =>
		patterns.reduce(
			(kept, pattern) =>
				pattern.startsWith('!') ? kept && !matchesGlob(file, pattern.slice(1)) : kept || matchesGlob(file, pattern),
			false,
		),
	)
}

/** A hunk header, capturing where its lines start on the new side and how many there are. */
const HUNK = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/

/**
 * Returns the mutate entries for the lines a change touched, as file:start-end, and each new file whole.
 * @param diff - The output of git diff --unified=0 with a/ and b/ prefixes.
 * @param untracked - The files git does not track yet, relative to the repository root.
 * @param patterns - The mutate patterns in order, a leading ! leaving files out.
 * @returns The entries the patterns name, in the order the change lists them.
 */
export function mutateTargets(diff: string, untracked: string[], patterns: string[]): string[] {
	const tracked = [...changedLines(diff)]
		.filter(([file]) => mutatedFiles([file], patterns).length > 0)
		.flatMap(([file, ranges]) => ranges.map((range) => `${file}:${range}`))
	return [...tracked, ...mutatedFiles(untracked, patterns)]
}

/** A shard argument, capturing its index and its count. */
const SHARD = /^(\d+)\/(\d+)$/

/** One shard of a mutation run, as its index counted from 1 and how many shards the run has. */
interface Shard {
	index: number
	count: number
}

/**
 * Returns the shard an argument names as index/count, or undefined when it names none.
 * @param text - The argument, as in 2/6.
 * @returns The shard, or undefined when the text is not an index from 1 to the count.
 */
export function parseShard(text: string): Shard | undefined {
	const parts = SHARD.exec(text)
	if (parts === null) {
		return undefined
	}
	const shard = { index: Number(parts[1]), count: Number(parts[2]) }
	return shard.index >= 1 && shard.index <= shard.count ? shard : undefined
}

/**
 * Returns the mutate entries one shard runs, every shard holding about the same weight.
 * @param targets - The mutate entries, as file:start-end or a whole file.
 * @param shard - The shard to return the entries of.
 * @param weight - Weighs a mutate entry, as the mutants it holds.
 * @returns The entries of the shard, in their given order.
 */
export function shardTargets(targets: string[], shard: Shard, weight: (target: string) => number): string[] {
	const loads = Array.from({ length: shard.count }, () => 0)
	const owners = new Map<string, number>()
	const weighed = targets.toSorted().map((target) => ({ target, load: weight(target) }))
	for (const { target, load } of weighed.toSorted((left, right) => right.load - left.load)) {
		const lightest = loads.reduce((least, held, place) => (held < loads[least] ? place : least), 0)
		loads[lightest] += load
		owners.set(target, lightest + 1)
	}
	return targets.filter((target) => owners.get(target) === shard.index)
}

/**
 * Returns how many shards a run needs, one for every perShard mutants and never more than its pieces.
 * @param weights - The mutants each piece of the run holds.
 * @param perShard - How many mutants one shard should test.
 * @param most - The most shards a run may use.
 * @returns The shard count, at least 1.
 */
export function shardCount(weights: number[], perShard: number, most: number): number {
	const mutants = weights.reduce((sum, weight) => sum + weight, 0)
	return Math.max(1, Math.min(most, weights.length, Math.ceil(mutants / perShard)))
}

/**
 * Returns the whole number, 1 or more, that an environment setting holds.
 * @param env - The environment to read.
 * @param name - The setting.
 * @returns Its value.
 */
export function wholeSetting(env: Record<string, string | undefined>, name: string): number {
	const value = Number(env[name] ?? '')
	if (!Number.isInteger(value) || value < 1) {
		throw new Error(`set ${name} to a whole number of at least 1`)
	}
	return value
}

/**
 * Returns the line ranges a diff with no context adds or changes, by file.
 * @param diff - The output of git diff --unified=0 with a/ and b/ prefixes.
 * @returns Each file with its changed ranges, as start-end.
 */
function changedLines(diff: string): Map<string, string[]> {
	const ranges = new Map<string, string[]>()
	let file = ''
	let header = false
	for (const line of diff.split('\n')) {
		const hunk = HUNK.exec(line)
		if (line.startsWith('diff --git ')) {
			file = ''
			header = true
		} else if (header && line.startsWith('+++ b/')) {
			file = line.slice('+++ b/'.length).replace(/\t$/, '')
		} else if (hunk !== null) {
			header = false
			addRange(ranges, file, Number(hunk[1]), Number(hunk[2] ?? '1'))
		}
	}
	return ranges
}

/**
 * Records the lines a hunk covers on the new side, unless it covers none.
 * @param ranges - The ranges found so far, by file.
 * @param file - The file the hunk changes.
 * @param start - The first line the hunk covers.
 * @param count - How many lines it covers.
 */
function addRange(ranges: Map<string, string[]>, file: string, start: number, count: number) {
	if (count > 0) {
		ranges.set(file, [...(ranges.get(file) ?? []), `${start}-${start + count - 1}`])
	}
}

/** The part of the Stryker config the mutation scripts read. */
interface StrykerConfig extends MutationConfig {
	mutate: string[]
	dryRunTimeoutMinutes?: number
}

/**
 * Returns the Stryker config the mutation runs use.
 * @returns The parsed config.
 */
export function strykerConfig(): StrykerConfig {
	return JSON.parse(readFileSync(join(repositoryRoot(), 'frontend', 'stryker.config.json'), 'utf8')) as StrykerConfig
}

/**
 * Returns the mutate patterns the Stryker config holds.
 * @returns The patterns, in the order the config lists them.
 */
export function strykerPatterns(): string[] {
	return strykerConfig().mutate
}
