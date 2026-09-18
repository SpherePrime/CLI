import { navigateTo } from "../context/route"
import { resetSession, loadStoredMessages, transcriptText, lastAssistantText, useSession } from "../context/session"
import { openInfo, openSelect, openForm, openModelDialog, openProviderDialog } from "../context/dialog"
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
  return [
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
      run: () => {
        const session = useSession()
        openForm({
          title: "Rename session",
          fields: [{ kind: "text", key: "title", label: "Title", placeholder: "Session name" }],
          onSubmit: async (values) => {
            const id = session.sessionId()
            if (!id || !values.title?.trim()) return
            try {
              await client.renameSession(id, values.title.trim())
              openInfo({ title: "Rename session", body: "Session renamed." })
            } catch (error) {
              openInfo({ title: "Rename session", body: String(error) })
            }
          },
        })
      },
    },
    {
      id: "session.fork",
      title: "Fork session",
      section: "Session",
      run: async () => {
        const session = useSession()
        const id = session.sessionId()
        if (!id) {
          openInfo({ title: "Fork session", body: "No active session." })
          return
        }
        try {
          const forked = await client.forkSession(id)
          resetSession()
          navigateTo({ type: "session", sessionId: forked.id })
        } catch (error) {
          openInfo({ title: "Fork session", body: String(error) })
        }
      },
    },
    {
      id: "session.compact",
      title: "Compact session",
      section: "Session",
      shortcut: "ctrl+x c",
      run: async () => {
        const session = useSession()
        const id = session.sessionId()
        if (!id) return
        try {
          const detail = await client.compactSession(id)
          resetSession()
          loadStoredMessages(detail.messages)
          openInfo({ title: "Compact session", body: "Session compacted." })
        } catch (error) {
          openInfo({ title: "Compact session", body: String(error) })
        }
      },
    },
    {
      id: "session.delete",
      title: "Delete session",
      section: "Session",
      shortcut: "ctrl+x d",
      run: () => {
        const session = useSession()
        const id = session.sessionId()
        if (!id) {
          openInfo({ title: "Delete session", body: "No active session." })
          return
        }
        openSelect({
          title: "Delete session",
          options: [
            { title: "Delete this session", value: "delete" },
            { title: "Cancel", value: "cancel" },
          ],
          onSelect: async (value) => {
            if (value !== "delete") return
            try {
              await client.deleteSession(id)
              resetSession()
              navigateTo({ type: "home" })
            } catch (error) {
              openInfo({ title: "Delete session", body: String(error) })
            }
          },
        })
      },
    },
    {
      id: "session.cancel",
      title: "Stop generation",
      section: "Session",
      shortcut: "esc",
      run: async () => {
        const session = useSession()
        try {
          await client.cancelMessage(session.sessionId())
        } catch {}
      },
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
      id: "agent.skills",
      title: "Skills",
      section: "Agent",
      run: async () => {
        try {
          const skills = await client.listSkills()
          if (skills.length === 0) {
            openInfo({ title: "Skills", body: "No skills installed." })
            return
          }
          openSelect({
            title: "Skills",
            options: skills.map((skill) => ({
              title: `${skill.enabled ? "[on] " : "[off] "}${skill.name}`,
              value: skill.name,
              description: skill.description || `scope: ${skill.scope}`,
            })),
            onSelect: async (name) => {
              try {
                const skill = skills.find((entry) => entry.name === name)
                if (!skill) return
                await client.toggleSkill(name, !skill.enabled)
                openInfo({
                  title: "Skills",
                  body: `${skill.name} ${skill.enabled ? "disabled" : "enabled"}.`,
                })
              } catch (error) {
                openInfo({ title: "Skills", body: String(error) })
              }
            },
          })
        } catch (error) {
          openInfo({ title: "Skills", body: String(error) })
        }
      },
    },
    {
      id: "agent.plugins",
      title: "Plugins",
      section: "Agent",
      run: async () => {
        try {
          const plugins = await client.listPlugins()
          if (plugins.length === 0) {
            openInfo({ title: "Plugins", body: "No plugins installed." })
            return
          }
          openSelect({
            title: "Plugins",
            options: plugins.map((plugin) => ({
              title: `${plugin.enabled ? "[on] " : "[off] "}${plugin.name}`,
              value: plugin.name,
              description: `${plugin.description || "plugin"} · v${plugin.version || "-"}`,
            })),
            onSelect: async (name) => {
              try {
                const plugin = plugins.find((entry) => entry.name === name)
                if (!plugin) return
                await client.togglePlugin(name, !plugin.enabled)
                openInfo({
                  title: "Plugins",
                  body: `${plugin.name} ${plugin.enabled ? "disabled" : "enabled"}.`,
                })
              } catch (error) {
                openInfo({ title: "Plugins", body: String(error) })
              }
            },
          })
        } catch (error) {
          openInfo({ title: "Plugins", body: String(error) })
        }
      },
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
            "ctrl+x c — compact session",
            "enter — send message",
            "shift+enter — new line",
            "esc — back to home / cancel",
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