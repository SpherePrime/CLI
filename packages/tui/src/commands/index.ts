import { navigateTo } from "../context/route"
import { resetSession, transcriptText, lastAssistantText } from "../context/session"
import { openInfo, openSelect, openModelDialog, openProviderDialog } from "../context/dialog"
import { quitApp } from "../context/app"
import { copyToClipboard } from "../util/clipboard"
import { exportTranscript, openEditor } from "../util/files"
import type { AgentClient } from "../client"

export type CommandSection = "Prompt" | "Session" | "Agent" | "Provider" | "System" | "Exit"

export type Command = {
  id: string
  title: string
  section: CommandSection
  suggested?: boolean
  shortcut?: string
  leader?: string
  run: () => void | Promise<void>
}

export const SECTION_ORDER: CommandSection[] = ["Prompt", "Session", "Agent", "Provider", "System", "Exit"]

export function buildCommands(client: AgentClient): Command[] {
  const notAvailable = (feature: string) => () => {
    openInfo({ title: feature, body: "This command is not available in this build yet." })
  }

  return [
    {
      id: "prompt.stash",
      title: "Stash prompt",
      section: "Prompt",
      run: notAvailable("Stash prompt"),
    },
    {
      id: "prompt.skills",
      title: "Skills",
      section: "Prompt",
      run: notAvailable("Skills"),
    },
    {
      id: "session.new",
      title: "New session",
      section: "Session",
      suggested: true,
      shortcut: "ctrl+x n",
      leader: "n",
      run: () => {
        resetSession()
        navigateTo({ type: "session" })
      },
    },
    {
      id: "session.switch",
      title: "Switch session",
      section: "Session",
      suggested: true,
      shortcut: "ctrl+x l",
      leader: "l",
      run: async () => {
        try {
          const sessions = await client.sessions()
          if (sessions.length === 0) {
            openInfo({ title: "Switch session", body: "No sessions yet." })
            return
          }
          openSelect({
            title: "Switch session",
            options: sessions.map((session) => ({
              title: session.title?.trim() || session.id.slice(0, 8),
              value: session.id,
              description: new Date(session.updated).toLocaleString(),
            })),
            onSelect: (id) => {
              resetSession()
              navigateTo({ type: "session", sessionId: id })
            },
          })
        } catch (error) {
          openInfo({ title: "Switch session", body: String(error) })
        }
      },
    },
    {
      id: "session.rename",
      title: "Rename session",
      section: "Session",
      shortcut: "ctrl+r",
      run: notAvailable("Rename session"),
    },
    {
      id: "session.fork",
      title: "Fork session",
      section: "Session",
      run: notAvailable("Fork session"),
    },
    {
      id: "session.compact",
      title: "Compact session",
      section: "Session",
      shortcut: "ctrl+x c",
      run: notAvailable("Compact session"),
    },
    {
      id: "session.undo",
      title: "Undo previous message",
      section: "Session",
      shortcut: "ctrl+x u",
      run: notAvailable("Undo previous message"),
    },
    {
      id: "session.editor",
      title: "Open editor",
      section: "Session",
      shortcut: "ctrl+x e",
      leader: "e",
      run: () => openEditor(process.cwd()),
    },
    {
      id: "session.copy-last",
      title: "Copy last assistant message",
      section: "Session",
      shortcut: "ctrl+x y",
      leader: "y",
      run: async () => {
        const text = lastAssistantText()
        if (!text) {
          openInfo({ title: "Copy last message", body: "No assistant message to copy." })
          return
        }
        const ok = await copyToClipboard(text)
        openInfo({ title: "Copy last message", body: ok ? "Copied to clipboard." : "Clipboard unavailable." })
      },
    },
    {
      id: "session.copy-transcript",
      title: "Copy session transcript",
      section: "Session",
      run: async () => {
        const text = transcriptText()
        if (!text) {
          openInfo({ title: "Copy transcript", body: "Session is empty." })
          return
        }
        const ok = await copyToClipboard(text)
        openInfo({ title: "Copy transcript", body: ok ? "Copied to clipboard." : "Clipboard unavailable." })
      },
    },
    {
      id: "session.export",
      title: "Export session transcript",
      section: "Session",
      shortcut: "ctrl+x x",
      leader: "x",
      run: async () => {
        const text = transcriptText()
        if (!text) {
          openInfo({ title: "Export transcript", body: "Session is empty." })
          return
        }
        const path = await exportTranscript(text)
        openInfo({ title: "Export transcript", body: path ? `Saved to ${path}` : "Failed to write file." })
      },
    },
    {
      id: "session.clear",
      title: "Clear conversation",
      section: "Session",
      run: () => {
        resetSession()
        openInfo({ title: "Clear conversation", body: "Conversation cleared." })
      },
    },
    {
      id: "agent.model",
      title: "Switch model",
      section: "Agent",
      suggested: true,
      shortcut: "ctrl+x m",
      leader: "m",
      run: () => openModelDialog(),
    },
    {
      id: "agent.switch",
      title: "Switch agent",
      section: "Agent",
      shortcut: "ctrl+x a",
      run: notAvailable("Switch agent"),
    },
    {
      id: "agent.mcp",
      title: "Toggle MCPs",
      section: "Agent",
      run: notAvailable("Toggle MCPs"),
    },
    {
      id: "agent.variant",
      title: "Variant cycle",
      section: "Agent",
      shortcut: "ctrl+t",
      run: notAvailable("Variant cycle"),
    },
    {
      id: "provider.connect",
      title: "Connect provider",
      section: "Provider",
      shortcut: "ctrl+a",
      run: () => openProviderDialog(),
    },
    {
      id: "system.status",
      title: "View status",
      section: "System",
      shortcut: "ctrl+x s",
      leader: "s",
      run: async () => {
        try {
          const info = await client.info()
          openInfo({
            title: "Status",
            body: [
              `server: ${info.name} ${info.version}`,
              `provider: ${info.model.provider}`,
              `model: ${info.model.model}`,
              `base url: ${info.model.base_url ?? "default"}`,
              `cwd: ${process.cwd()}`,
            ].join("\n"),
          })
        } catch (error) {
          openInfo({ title: "Status", body: `offline: ${String(error)}` })
        }
      },
    },
    {
      id: "system.debug",
      title: "View debug info",
      section: "System",
      run: () =>
        openInfo({
          title: "Debug info",
          body: [
            `platform: ${process.platform}`,
            `bun: ${Bun.version}`,
            `cwd: ${process.cwd()}`,
            `columns: ${process.stdout.columns ?? "?"}`,
            `rows: ${process.stdout.rows ?? "?"}`,
          ].join("\n"),
        }),
    },
    {
      id: "system.theme",
      title: "Switch theme",
      section: "System",
      shortcut: "ctrl+x t",
      run: notAvailable("Switch theme"),
    },
    {
      id: "system.help",
      title: "Help",
      section: "System",
      run: () =>
        openInfo({
          title: "Help",
          body: [
            "ctrl+p — command palette",
            "ctrl+x n — new session",
            "ctrl+x l — switch session",
            "ctrl+x m — switch model",
            "enter — send message",
            "shift+enter — new line",
            "esc — back to home",
            "ctrl+c — exit",
          ].join("\n"),
        }),
    },
    {
      id: "exit.app",
      title: "Exit the app",
      section: "Exit",
      shortcut: "ctrl+c",
      leader: "q",
      run: () => quitApp(),
    },
  ]
}
