// SPDX-License-Identifier: Apache-2.0

import { ACCESS_ADMIN, Link, Stack, Text, can } from '@gophenberg/frontend-sdk'
import { useSession } from '@gopherium/react-auth'
import { AccountPanel } from '@gopherium/react-auth/wpds'
import { __ } from '@wordpress/i18n'
import type { ReactNode } from 'react'

/**
 * Renders the admin for a session carrying access_admin, and the screen turning any other reader away.
 * @param props - The admin to render.
 * @returns The admin, or the screen turning the reader away.
 */
export function AdminGate({ children }: { children: ReactNode }) {
	if (can(useSession().data, ACCESS_ADMIN)) {
		return children
	}
	return <NoAccess />
}

/**
 * Renders the screen telling a signed-in reader the admin is not theirs to enter.
 * @returns The screen element.
 */
function NoAccess() {
	return (
		<main className="gophenberg-no-access">
			<Stack direction="column" gap="lg">
				<Text>{__('Sorry, you are not allowed to access this page.', 'gophenberg')}</Text>
				<Text>
					<Link href="/">{__('Visit Site', 'gophenberg')}</Link>
				</Text>
				<AccountPanel className="gophenberg-no-access__account" />
			</Stack>
		</main>
	)
}
