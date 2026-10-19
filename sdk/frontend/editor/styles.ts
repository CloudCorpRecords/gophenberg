// SPDX-License-Identifier: Apache-2.0

import blockLibraryStyle from '@wordpress/block-library/build-style/style.css?inline'
import blockLibraryTheme from '@wordpress/block-library/build-style/theme.css?inline'

/** The content and wide sizes of the canvas column. */
export const CANVAS_LAYOUT = { contentSize: '700px', wideSize: '900px' }

const CANVAS_TYPOGRAPHY = `
	body {
		--wp--style--root--padding-top: 32px;
		--wp--style--root--padding-right: clamp(30px, 5vw, 50px);
		--wp--style--root--padding-bottom: 32px;
		--wp--style--root--padding-left: clamp(30px, 5vw, 50px);
		--wp--style--global--content-size: ${CANVAS_LAYOUT.contentSize};
		--wp--style--global--wide-size: ${CANVAS_LAYOUT.wideSize};
		font-family: system-ui, sans-serif;
		font-size: 16px;
		margin: 0;
		padding-top: var(--wp--style--root--padding-top);
		padding-right: 0;
		padding-bottom: var(--wp--style--root--padding-bottom);
		padding-left: 0;
	}
	p {
		font-size: inherit;
		line-height: inherit;
	}
	ul,
	ol {
		margin: 0;
		padding: 0 0 0 1.5em;
	}
	ul {
		list-style-type: disc;
	}
	ol {
		list-style-type: decimal;
	}
	.rich-text [data-rich-text-placeholder] {
		pointer-events: none;
	}
	.rich-text [data-rich-text-placeholder]::after {
		content: attr(data-rich-text-placeholder);
		opacity: 0.62;
	}
`

const CANVAS_ROOT_PADDING = `
	.has-global-padding {
		padding-right: var(--wp--style--root--padding-right);
		padding-left: var(--wp--style--root--padding-left);
	}
	.has-global-padding > .alignfull {
		margin-right: calc(var(--wp--style--root--padding-right) * -1);
		margin-left: calc(var(--wp--style--root--padding-left) * -1);
	}
	.has-global-padding
		:where(:not(.alignfull.is-layout-flow) > .has-global-padding:not(.wp-block-block, .alignfull)) {
		padding-right: 0;
		padding-left: 0;
	}
	.has-global-padding
		:where(:not(.alignfull.is-layout-flow) > .has-global-padding:not(.wp-block-block, .alignfull))
		> .alignfull {
		margin-left: 0;
		margin-right: 0;
	}
`

const CANVAS_LAYOUTS = `
	.is-layout-flow > .alignleft,
	.is-layout-constrained > .alignleft {
		float: left;
		margin-inline-start: 0;
		margin-inline-end: 2em;
	}
	.is-layout-flow > .alignright,
	.is-layout-constrained > .alignright {
		float: right;
		margin-inline-start: 2em;
		margin-inline-end: 0;
	}
	.is-layout-flow > .aligncenter,
	.is-layout-constrained > .aligncenter {
		margin-left: auto !important;
		margin-right: auto !important;
	}
	.is-layout-constrained > :where(:not(.alignleft):not(.alignright):not(.alignfull)) {
		max-width: var(--wp--style--global--content-size);
		margin-left: auto !important;
		margin-right: auto !important;
	}
	.is-layout-constrained > .alignwide {
		max-width: var(--wp--style--global--wide-size);
	}
`

export const CANVAS_STYLES = [
	{ css: blockLibraryStyle },
	{ css: blockLibraryTheme },
	{ css: CANVAS_TYPOGRAPHY },
	{ css: CANVAS_ROOT_PADDING },
	{ css: CANVAS_LAYOUTS },
]
