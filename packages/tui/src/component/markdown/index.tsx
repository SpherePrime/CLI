import { For } from "solid-js"
import { theme } from "../../theme"
import { EmptyBorder } from "../../ui/border"

interface MarkdownLine {
  kind: "code" | "inline" | "heading" | "text"
  content: string
  indent?: boolean
}

function parseMarkdownLines(raw: string): MarkdownLine[] {
  const lines: MarkdownLine[] = []
  let inCode = false
  let codeBuffer: string[] = []

  for (const line of raw.split("\n")) {
    if (line.startsWith("```")) {
      if (!inCode) {
        inCode = true
        codeBuffer = []
      } else {
        lines.push({ kind: "code", content: codeBuffer.join("\n") })
        inCode = false
        codeBuffer = []
      }
      continue
    }
    if (inCode) {
      codeBuffer.push(line)
      continue
    }
    if (line.startsWith("# ")) {
      lines.push({ kind: "heading", content: line.slice(2) })
    } else if (line.startsWith("## ")) {
      lines.push({ kind: "heading", content: line.slice(3) })
    } else if (line.startsWith("### ")) {
      lines.push({ kind: "heading", content: line.slice(4) })
    } else if (line.startsWith("- ") || line.startsWith("* ")) {
      lines.push({ kind: "inline", content: `  • ${line.slice(2)}`, indent: true })
    } else {
      lines.push({ kind: "text", content: line })
    }
  }
  if (inCode && codeBuffer.length > 0) {
    lines.push({ kind: "code", content: codeBuffer.join("\n") })
  }
  return lines
}

export function MarkdownBlock(props: { text: string }) {
  const parsed = parseMarkdownLines(props.text)

  return (
    <box flexDirection="column">
      <For each={parsed}>
        {(line) => {
          if (line.kind === "code") {
            return (
              <box
                backgroundColor={theme.backgroundElement}
                border={["left"]}
                borderColor={theme.borderSubtle}
                customBorderChars={{ ...EmptyBorder, vertical: "│" }}
                paddingLeft={1}
                paddingRight={1}
              >
                <text fg={theme.accent}>{line.content}</text>
              </box>
            )
          }
          if (line.kind === "heading") {
            return <text fg={theme.primary}>{line.content}</text>
          }
          if (line.kind === "inline") {
            return <text fg={line.indent ? theme.dim : theme.text}>{line.content}</text>
          }
          return <text fg={theme.text}>{line.content}</text>
        }}
      </For>
    </box>
  )
}
