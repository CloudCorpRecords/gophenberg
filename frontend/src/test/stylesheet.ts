// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

/** The stylesheet the application ships. */
const sheet = readFileSync(join(import.meta.dirname, '..', 'index.css'), 'utf8')

/**
 * Escapes a string for use inside a regular expression.
 * @param text - The literal text.
 * @returns The escaped text.
 */
function literal(text: string): string {
	return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Returns the declarations of the top level rule for exactly one selector.
 * @param selector - The selector of the rule.
 * @returns The declarations, keyed by property.
 */
export function declarations(selector: string): Record<string, string> {
	const body = new RegExp(`(?:^|\\n)${literal(selector)} \\{([^}]*)\\}`).exec(sheet)?.[1] ?? ''
	return Object.fromEntries(
		body
			.split(';')
			.map((declaration) => declaration.trim())
			.filter((declaration) => declaration !== '')
			.map((declaration) => {
				const at = declaration.indexOf(':')
				return [declaration.slice(0, at).trim(), declaration.slice(at + 1).trim()]
			}),
	)
}
