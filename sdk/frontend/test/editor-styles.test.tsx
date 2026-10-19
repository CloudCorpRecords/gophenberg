// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import ambient from '../editor/ambient.ts?raw'
import { CANVAS_LAYOUT, CANVAS_STYLES } from '../editor'

const CONSTRAINED_CHILD = '.is-layout-constrained > :where(:not(.alignleft):not(.alignright):not(.alignfull))'

const NESTED_BOX =
	'.has-global-padding :where(:not(.alignfull.is-layout-flow) > .has-global-padding:not(.wp-block-block, .alignfull))'

/**
 * Returns every stylesheet the canvas loads, each run of white space squeezed to one space.
 * @returns The canvas CSS on one line.
 */
function canvasCss(): string {
	return CANVAS_STYLES.map((entry) => entry.css)
		.join('\n')
		.replaceAll(/\s+/g, ' ')
}

const PARENT_SHEETS = [
	'@wordpress/components/build-style/style.css',
	'@wordpress/block-editor/build-style/style.css',
	'@wordpress/block-library/build-style/editor.css',
]

test.each(PARENT_SHEETS)('loads %s into the document the chrome renders in', (sheet) => {
	expect(ambient).toContain(sheet)
})

test('leaves the format popover stylesheet alone, since the package exports no path to it', () => {
	expect(ambient).not.toContain('@wordpress/format-library/build-style')
})

test('registers the formats a writer marks text up with', async () => {
	await import('../editor/ambient')
	const { select } = await import('@wordpress/data')
	const { store } = await import('@wordpress/rich-text')

	const formats = select(store as never) as { getFormatTypes: () => { name: string }[] }
	const registered = formats.getFormatTypes()
	const names = registered.map((format) => format.name)

	expect(names).toEqual(
		expect.arrayContaining(['core/bold', 'core/italic', 'core/link', 'core/code']),
	)
})

test('names the content and wide sizes the canvas column and the editor settings share', () => {
	expect(CANVAS_LAYOUT).toEqual({ contentSize: '700px', wideSize: '900px' })
})

test('holds the blocks of a constrained box in a readable column, as the WordPress constrained layout does', () => {
	const css = canvasCss()

	expect(css).toContain('--wp--style--global--content-size: 700px;')
	expect(css).toContain(
		`${CONSTRAINED_CHILD} { max-width: var(--wp--style--global--content-size); ` +
			'margin-left: auto !important; margin-right: auto !important; }',
	)
	expect(css).not.toContain('.wp-block { max-width: 700px;')
})

test('shows the hint an empty block carries', () => {
	const css = CANVAS_STYLES.map((entry) => entry.css).join('\n')

	expect(css).toMatch(
		/\[data-rich-text-placeholder\]::after\s*\{[^}]*content:\s*attr\(data-rich-text-placeholder\)/,
	)
})

test('sets the root padding of the canvas as the WordPress theme sets it, 30px on a phone and 50px wide', () => {
	const css = canvasCss()

	expect(css).toContain('--wp--style--root--padding-top: 32px;')
	expect(css).toContain('--wp--style--root--padding-right: clamp(30px, 5vw, 50px);')
	expect(css).toContain('--wp--style--root--padding-bottom: 32px;')
	expect(css).toContain('--wp--style--root--padding-left: clamp(30px, 5vw, 50px);')
})

test('pads the canvas body only at its top and bottom, leaving the sides to the root container', () => {
	const css = canvasCss()

	expect(css).toMatch(/body \{[^}]* padding-top: var\(--wp--style--root--padding-top\);/)
	expect(css).toMatch(/body \{[^}]* padding-right: 0;/)
	expect(css).toMatch(/body \{[^}]* padding-bottom: var\(--wp--style--root--padding-bottom\);/)
	expect(css).toMatch(/body \{[^}]* padding-left: 0;/)
})

test('pads every root padding aware box and runs its full children out to its edges, as WordPress does', () => {
	const css = canvasCss()

	expect(css).toContain(
		'.has-global-padding { padding-right: var(--wp--style--root--padding-right); ' +
			'padding-left: var(--wp--style--root--padding-left); }',
	)
	expect(css).toContain(
		'.has-global-padding > .alignfull { margin-right: calc(var(--wp--style--root--padding-right) * -1); ' +
			'margin-left: calc(var(--wp--style--root--padding-left) * -1); }',
	)
})

test('unpads a nested box that is not full and keeps its full children inside it, as WordPress does', () => {
	const css = canvasCss()

	expect(css).toContain(`${NESTED_BOX} { padding-right: 0; padding-left: 0; }`)
	expect(css).toContain(`${NESTED_BOX} > .alignfull { margin-left: 0; margin-right: 0; }`)
})

test('lets a wide block of a constrained box run to the wide size', () => {
	const css = canvasCss()

	expect(css).toContain('--wp--style--global--wide-size: 900px;')
	expect(css).toContain('.is-layout-constrained > .alignwide { max-width: var(--wp--style--global--wide-size); }')
})

test('floats left and right blocks and centres a centred one, as the WordPress flow and constrained layouts do', () => {
	const css = canvasCss()

	expect(css).toContain(
		'.is-layout-flow > .alignleft, .is-layout-constrained > .alignleft ' +
			'{ float: left; margin-inline-start: 0; margin-inline-end: 2em; }',
	)
	expect(css).toContain(
		'.is-layout-flow > .alignright, .is-layout-constrained > .alignright ' +
			'{ float: right; margin-inline-start: 2em; margin-inline-end: 0; }',
	)
	expect(css).toContain(
		'.is-layout-flow > .aligncenter, .is-layout-constrained > .aligncenter ' +
			'{ margin-left: auto !important; margin-right: auto !important; }',
	)
})
