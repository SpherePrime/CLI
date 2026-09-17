import solidPlugin from "@opentui/solid/bun-plugin"

const result = await Bun.build({
  entrypoints: ["src/index.tsx"],
  target: "bun",
  plugins: [solidPlugin],
  compile: {
    outfile: "dist/agent-tui.exe",
    autoloadBunfig: false,
    autoloadDotenv: false,
  },
})

if (!result.success) {
  for (const message of result.logs) {
    console.error(message)
  }
  process.exit(1)
}

console.log("built dist/agent-tui.exe")
