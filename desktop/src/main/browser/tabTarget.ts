import type { BrowserPanel } from './panel'

export type BrowserTarget = ReturnType<BrowserPanel['getTab']>

export const tabIdProperty = {
  type: 'string',
  description:
    'Tab ID from browser_tabs, browser_new_tab or browser_snapshot. Pass it for independent or parallel tab work. Omit only to use the currently selected tab.'
}

export function browserTarget(panel: BrowserPanel, args: { tabId?: unknown }): BrowserTarget {
  if (args.tabId !== undefined && (typeof args.tabId !== 'string' || !args.tabId.trim()))
    throw new Error('tabId must be a nonempty tab ID from browser_tabs')
  return panel.getTab(args.tabId as string | undefined)
}
