// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { setUserRole, usersQueryKey } from '@gopherium/react-auth/admin'
import type { User } from '@gopherium/react-auth/admin'
import { ErrorNotice, useToaster } from '@gopherium/godmin'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useId, useState } from 'react'

import { roleFailureMessage, roleLabel } from './roles'

/**
 * Renders the button opening the dialog that writes the role another account holds.
 * @param props - The account the dialog acts on and the roles the reader may give, widest first.
 * @returns The button and the dialog it opens.
 */
export function ChangeRole({ user, grantable }: { user: User, grantable: string[] }) {
	const [open, setOpen] = useState(false)
	return (
		<>
			<Button
				variant="outline"
				aria-label={sprintf(__('Change the role of %(name)s', 'gophenberg'), { name: user.name })}
				onClick={() => setOpen(true)}
			>
				{__('Change role', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup size="small">
					<Dialog.Header>
						<Dialog.Title>{__('Change role', 'gophenberg')}</Dialog.Title>
						<Dialog.CloseIconButton />
					</Dialog.Header>
					{open && <RoleForm user={user} grantable={grantable} close={() => setOpen(false)} />}
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the form picking and writing the role of one account, its failure kept inside.
 * @param props - The account, the roles the reader may give, and the handler closing the dialog.
 * @returns The role form.
 */
function RoleForm({ user, grantable, close }: { user: User, grantable: string[], close: () => void }) {
	const client = useQueryClient()
	const toaster = useToaster()
	const [chosen, setChosen] = useState(user.role)
	const write = useMutation({
		mutationFn: () => setUserRole(user.id, chosen),
		onSuccess: async () => {
			await client.invalidateQueries({ queryKey: usersQueryKey })
			toaster.show(__('Role changed.', 'gophenberg'))
			close()
		},
	})
	const offered = grantable.map((role) => ({ value: role, label: roleLabel(role) }))
	const formId = useId()
	return (
		<>
			<Dialog.Content>
				<form
					id={formId}
					onSubmit={(event) => {
						event.preventDefault()
						write.mutate()
					}}
				>
					<Stack direction="column" gap="lg">
						<Text>{sprintf(__('Choose the role of %(name)s.', 'gophenberg'), { name: user.name })}</Text>
						<SelectControl
							label={_x('Role', 'account', 'gophenberg')}
							items={offered}
							value={offered.find((option) => option.value === chosen)}
							onValueChange={(item) => item?.value != null && setChosen(item.value)}
						/>
						{write.isError && <ErrorNotice>{roleFailureMessage(write.error)}</ErrorNotice>}
					</Stack>
				</form>
			</Dialog.Content>
			<Dialog.Footer>
				<Button variant="minimal" disabled={write.isPending} onClick={close}>
					{__('Cancel', 'gophenberg')}
				</Button>
				<Button type="submit" form={formId} loading={write.isPending} disabled={chosen === user.role}>
					{__('Change role', 'gophenberg')}
				</Button>
			</Dialog.Footer>
		</>
	)
}
