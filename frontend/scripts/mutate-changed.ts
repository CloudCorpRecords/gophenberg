// SPDX-License-Identifier: Apache-2.0

import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { repositoryRoot } from './config.ts'
import { countMutants, instrumenterOptions } from './mutantCount.ts'
import { mutateTargets, parseShard, shardCount, shardTargets, strykerConfig, wholeSetting } from './mutants.ts'
import { cutTarget, statementStarts, targetFile } from './pieces.ts'

const [base, shardArgument = '1/1'] = process.argv.slice(2)
if (base === undefined) {
	console.error('name the base to compare with, as in pnpm run mutate:changed origin/main')
	process.exit(1)
}

const shard = shardArgument === 'count' ? 'count' : parseShard(shardArgument)
if (shard === undefined) {
	console.error('name the shard as index/count, as in pnpm run mutate:changed origin/main 2/6, or pass the word count')
	process.exit(1)
}

const root = repositoryRoot()

/**
 * Returns what git prints for the arguments, run at the repository root.
 * @param args - The git arguments.
 * @returns The output, however long it is.
 */
const git = (args: string[]): string => execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: Infinity })

/**
 * Returns the text of a file.
 * @param file - The file, relative to the repository root.
 * @returns Its text.
 */
const sourceOf = (file: string): string => readFileSync(join(root, file), 'utf8')

/**
 * Returns a mutate entry cut at the top-level statements that start inside it.
 * @param target - The mutate entry, as file:start-end or a whole file.
 * @returns Its pieces.
 */
const piecesOf = (target: string): string[] =>
	cutTarget(target, statementStarts(sourceOf(targetFile(target)), targetFile(target)))
const roots = ['frontend/src', 'frontend/scripts', 'sdk/frontend', 'plugins']
const diff = git([
	'-c',
	'core.quotePath=false',
	'diff',
	'--unified=0',
	'--inter-hunk-context=0',
	'--no-color',
	'--no-ext-diff',
	'--src-prefix=a/',
	'--dst-prefix=b/',
	'--diff-filter=d',
	'--merge-base',
	base,
	'--',
	...roots,
])
const untracked = git(['ls-files', '-z', '--others', '--exclude-standard', '--', ...roots]).split('\0').filter(Boolean)
const config = strykerConfig()
const pieces = mutateTargets(diff, untracked, config.mutate).flatMap(piecesOf)
const counts = await countMutants(pieces, sourceOf, instrumenterOptions(config))

/**
 * Returns how many mutants Stryker tests in a piece.
 * @param piece - The piece, as file:start-end or a whole file.
 * @returns Its mutant count.
 */
const weight = (piece: string): number => counts.get(piece) ?? 0
const weighed = pieces.filter((piece) => weight(piece) > 0)
const total = weighed.reduce((sum, piece) => sum + weight(piece), 0)

if (shard === 'count') {
	const perShard = wholeSetting(process.env, 'MUTANTS_PER_SHARD')
	const most = wholeSetting(process.env, 'MAX_SHARDS')
	const count = shardCount(weighed.map(weight), perShard, most)
	console.error(`${total} mutants in ${weighed.length} pieces, ${count} shards of ${perShard}, at most ${most}`)
	console.log(`shards=${JSON.stringify(Array.from({ length: count }, (_, place) => place + 1))}`)
} else {
	const targets = shardTargets(weighed, shard, weight)
	const held = targets.reduce((sum, piece) => sum + weight(piece), 0)
	console.log(`shard ${shardArgument}: ${targets.length} of ${weighed.length} pieces, ${held} of ${total} mutants`)
	if (targets.length === 0) {
		console.log('no changed source line to mutate')
	} else {
		execFileSync('stryker', ['run', 'frontend/stryker.config.json', '--ignoreStatic', '--mutate', targets.join(',')], {
			cwd: root,
			stdio: 'inherit',
		})
	}
}
