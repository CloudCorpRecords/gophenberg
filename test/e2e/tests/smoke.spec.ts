// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

import { credentials } from '../env'

test('serves the admin shell to a stored session', async ({ page }) => {
	await page.goto('/admin/')

	await expect(page.getByRole('link', { name: 'Gophenberg', exact: true })).toBeVisible()
	await expect(page.getByRole('heading', { name: 'Posts', level: 1 })).toBeVisible()
	await expect(page).toHaveURL(/\/admin\/content\/post$/)
	await expect(page.locator('.gophenberg-rail__account').getByText(credentials.name)).toBeVisible()
	await expect(page.getByRole('link', { name: 'Users' })).toBeVisible()
})

test('reports the application version to a stored session', async ({ page }) => {
	const response = await page.request.get('/api/version')

	expect(response.status()).toBe(200)
	expect((await response.json()).version).toMatch(/^\d+\.\d+\.\d+$/)
})
