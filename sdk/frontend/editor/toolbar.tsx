// SPDX-License-Identifier: Apache-2.0

import { BlockIcon, BlockToolbar as EditorBlockToolbar, NavigableToolbar } from '@wordpress/block-editor'
import { getBlockType, getDefaultBlockName } from '@wordpress/blocks'
import { ToolbarButton, ToolbarGroup } from '@wordpress/components'
import { useSelect } from '@wordpress/data'
import { __ } from '@wordpress/i18n'
import { blockDefault, chevronDown, chevronUp, moreVertical } from '@wordpress/icons'

import { BLOCK_EDITOR_STORE } from './media'

/** What the block editor store answers about the selection. */
interface SelectionReader {
	getSelectedBlockClientIds: () => string[]
}

/**
 * Returns the icon and name of the block an empty paragraph stands for.
 * @returns The default block's icon and title, or the generic block's when none is registered.
 */
function defaultBlock() {
	const type = getBlockType(getDefaultBlockName() as string)
	return { icon: type?.icon ?? blockDefault, title: type?.title ?? __('Block') }
}

/**
 * Renders the block toolbar as it stands while no block is selected, every control disabled.
 * @returns The placeholder toolbar.
 */
function BlockToolbarPlaceholder() {
	const block = defaultBlock()
	return (
		<NavigableToolbar
			focusEditorOnEscape
			shouldUseKeyboardFocusShortcut={false}
			className="block-editor-block-contextual-toolbar is-placeholder"
			aria-label={__('Block tools')}
			variant="unstyled"
		>
			<div className="block-editor-block-toolbar">
				<ToolbarGroup className="block-editor-block-toolbar__block-controls">
					<ToolbarButton icon={<BlockIcon icon={block.icon} />} label={block.title} disabled accessibleWhenDisabled />
					<ToolbarGroup className="block-editor-block-mover">
						<div className="block-editor-block-mover__move-button-container">
							<ToolbarButton
								className="block-editor-block-mover-button is-up-button"
								icon={chevronUp}
								label={__('Move up')}
								disabled
								accessibleWhenDisabled
							/>
							<ToolbarButton
								className="block-editor-block-mover-button is-down-button"
								icon={chevronDown}
								label={__('Move down')}
								disabled
								accessibleWhenDisabled
							/>
						</div>
					</ToolbarGroup>
				</ToolbarGroup>
				<ToolbarGroup>
					<ToolbarButton icon={moreVertical} label={__('Options')} disabled accessibleWhenDisabled />
				</ToolbarGroup>
			</div>
		</NavigableToolbar>
	)
}

/**
 * Renders the block editor's toolbar, standing a disabled placeholder in while no block is selected when asked to.
 * @param props - Whether to leave the drag handle out and whether to hold the row while no block is selected.
 * @returns The toolbar, or nothing while no block is selected and no placeholder is asked for.
 */
export function BlockToolbar(
	{ hideDragHandle, showPlaceholder }: { hideDragHandle?: boolean, showPlaceholder?: boolean },
) {
	const selected = useSelect(
		(select) => (select(BLOCK_EDITOR_STORE) as unknown as SelectionReader).getSelectedBlockClientIds().length > 0,
		[],
	)
	if (showPlaceholder && !selected) {
		return <BlockToolbarPlaceholder />
	}
	return <EditorBlockToolbar hideDragHandle={hideDragHandle} />
}
