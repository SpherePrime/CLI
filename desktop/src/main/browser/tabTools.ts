import type { Tool } from './tools'
import { browserTarget, tabIdProperty } from './tabTarget'

const result = (value: unknown) => [{ type: 'text' as const, text: JSON.stringify(value) }]

export const browserTabTools: Tool[] = [
  {
    name: 'browser_tabs',
    description:
      'List built-in browser tabs with stable IDs, URLs, titles and the selected tab. Use tabId on page tools to work across tabs without changing the user selection.',
    inputSchema: { type: 'object', properties: {} },
    run: async (panel) => {
      const { tabs, activeTab } = panel.state()
      return result({ tabs, activeTab })
    }
  },
  {
    name: 'browser_new_tab',
    description:
      'Open a separate built-in browser tab and return its tabId. Opens in the background by default so existing tabs and the user selection remain unchanged.',
    inputSchema: {
      type: 'object',
      properties: {
        url: { type: 'string', description: 'URL or local file path; defaults to about:blank' },
        activate: {
          type: 'boolean',
          description: 'Select and show this tab to the user; defaults to false'
        }
      }
    },
    run: async (panel, args) => {
      const tab = await panel.openTab(String(args.url || 'about:blank'), args.activate === true)
      if (args.activate === true) panel.reveal(true)
      return result({ ...tab, tabId: tab.id })
    }
  },
  {
    name: 'browser_select_tab',
    description:
      'Explicitly select a built-in browser tab for the user to view. Page tools can target background tabs by tabId without selecting them.',
    inputSchema: { type: 'object', properties: { tabId: tabIdProperty }, required: ['tabId'] },
    run: async (panel, args) => {
      if (!args.tabId) throw new Error('tabId is required')
      const target = browserTarget(panel, args)
      panel.reveal(true)
      return result(panel.selectTab(target.id))
    }
  },
  {
    name: 'browser_close_tab',
    description: 'Close the specified built-in browser tab. Other tabs remain open.',
    inputSchema: { type: 'object', properties: { tabId: tabIdProperty }, required: ['tabId'] },
    run: async (panel, args) => {
      if (!args.tabId) throw new Error('tabId is required')
      const target = browserTarget(panel, args)
      return result(panel.closeTab(target.id))
    }
  }
]
