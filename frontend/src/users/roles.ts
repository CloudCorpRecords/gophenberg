// SPDX-License-Identifier: Apache-2.0

import { ADMIN, AUTHOR, EDITOR } from '@gophenberg/frontend-sdk'
import {
	LastPrivilegedError,
	RoleBeyondReachError,
	RoleUnknownError,
	SelfRoleError,
} from '@gopherium/react-auth/admin'
import { __, _x } from '@wordpress/i18n'

import { errorTemplates } from '../i18n/errorTemplates'

/** The error code each turned down role write answers with, by the error it raises. */
const refusals: [new (...args: never[]) => Error, string][] = [
	[SelfRoleError, 'self_role_refused'],
	[LastPrivilegedError, 'last_privileged_refused'],
	[RoleBeyondReachError, 'role_beyond_reach'],
	[RoleUnknownError, 'role_unknown'],
]

/**
 * Returns the label each core role reads as, and the one an account holding no role reads as.
 * @returns The labels, keyed by role.
 */
function coreLabels(): Map<string, string> {
	return new Map<string, string>([
		[ADMIN, _x('Administrator', 'role', 'gophenberg')],
		[EDITOR, _x('Editor', 'role', 'gophenberg')],
		[AUTHOR, _x('Author', 'role', 'gophenberg')],
		['', _x('No role', 'role', 'gophenberg')],
	])
}

/**
 * Returns the label a role reads as, any role outside the core's reading as stored with a capital first letter.
 * @param role - The role the account holds.
 * @returns The label.
 */
export function roleLabel(role: string): string {
	return coreLabels().get(role) ?? role.charAt(0).toUpperCase() + role.slice(1)
}

/**
 * Returns the role a new account starts under, author when the reader may give it, else the narrowest it may give.
 * @param grantable - The roles the reader may give, widest first.
 * @returns The starting role.
 */
export function startingRole(grantable: string[]): string {
	return grantable.includes(AUTHOR) ? AUTHOR : (grantable.at(-1) ?? AUTHOR)
}

/**
 * Returns the code a turned down role write carries, none for any other failure.
 * @param error - The failure the write raised.
 * @returns The error code, or nothing.
 */
export function roleRefusal(error: unknown): string | undefined {
	return refusals.find(([refused]) => error instanceof refused)?.[1]
}

/**
 * Returns the message a failed role write shows, in the reader's language.
 * @param error - The failure the write raised.
 * @returns The message.
 */
export function roleFailureMessage(error: unknown): string {
	const code = roleRefusal(error)
	const template = code === undefined ? undefined : errorTemplates()[code]
	return template ?? __('The role could not be changed.', 'gophenberg')
}
