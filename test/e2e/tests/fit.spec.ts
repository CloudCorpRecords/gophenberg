// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

const PHONE = { width: 390, height: 844 }

const LAPTOP = { width: 1280, height: 800 }

const LONG_TITLE =
	'A post whose title runs on well past the width any phone screen could ever offer a reader'

const UNBREAKABLE = 'W'.repeat(60)

const RUN = Math.random().toString(36).slice(2, 8)

/**
 * Returns how far the document and the canvas overflow the screen.
 * @param page - The page to measure.
 * @returns The overflow of each region in pixels.
 */
async function spills(page: Page) {
	return page.evaluate(() => {
		const canvas = document.querySelector('.godmin-layout__canvas')
		const width = document.documentElement.clientWidth
		return {
			document: document.documentElement.scrollWidth - width,
			canvas: canvas === null ? 0 : Math.round(canvas.scrollWidth - canvas.clientWidth),
		}
	})
}

test.beforeEach(async ({ page }) => {
	await page.setViewportSize(PHONE)
})

test('seeds a hostile post and fits every screen to a phone', async ({ page }) => {
	const created = await page.request.post('/api/content', {
		data: { type: 'post', title: `${LONG_TITLE} ${UNBREAKABLE}` },
	})
	expect(created.status()).toBe(201)

	const screens = [
		'/admin/',
		'/admin/content/post',
		'/admin/content/post?status=trash',
		'/admin/content/page',
		'/admin/content/category',
		'/admin/users',
		'/admin/users/new',
	]
	for (const path of screens) {
		await page.goto(path)
		await expect(page.getByRole('main')).toBeVisible()

		const overflow = await spills(page)
		expect(overflow.document, `${path} spills the document`).toBeLessThanOrEqual(0)
		expect(overflow.canvas, `${path} spills the canvas`).toBeLessThanOrEqual(0)
	}
})

test('fits the posts table to its card at 1280 beside a long title, the listed fields left in View options', async ({
	page,
}) => {
	const title = `${LONG_TITLE} ${RUN}`
	const created = await page.request.post('/api/content', { data: { type: 'post', title } })
	expect(created.status()).toBe(201)
	const { id } = (await created.json()) as { id: string }
	await page.setViewportSize(LAPTOP)

	await page.goto(`/admin/content/post?search=${encodeURIComponent(title)}`)
	const table = page.locator('.dataviews-view-table')
	await expect(table.getByRole('link', { name: title, exact: true })).toBeVisible()

	const scroll = await table.evaluate((element) => {
		const scroller = element.closest('.dataviews-layout__container') as HTMLElement
		return scroller.scrollWidth - scroller.clientWidth
	})
	expect(scroll).toBeLessThanOrEqual(0)
	await expect(table.locator('.dataviews-view-table__actions-column--stuck')).toHaveCount(0)
	await expect(page.getByRole('columnheader', { name: 'Featured' })).toHaveCount(0)
	await page.getByRole('button', { name: 'View options' }).click()
	await expect(page.locator('.dataviews-field-control').getByRole('button', { name: 'Featured' })).toBeVisible()
	await page.request.delete(`/api/content/${id}?force=true`)
})
