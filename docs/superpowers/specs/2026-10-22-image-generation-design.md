# Design: Built-in image generation tool

Date: 2026-10-22
Status: approved in chat

## Goal

Make image generation a built-in Prime capability (native tool, no MCP, no Node):
`generate_image` calls any OpenAI-compatible `POST {baseURL}/images/generations`,
saves files to disk, and is configured from the desktop settings UI. If settings
are missing, the tool returns a hint plus a `prime://settings/image-generation`
link that opens the settings dialog on the right tab.

## Config

`internal/config/config.go`:

- New `ImageGenOptions` struct in `Options` (`options.image_gen.*`):
  `api_key`, `model` (default `gpt-image-1`), `base_url`
  (default `https://api.openai.com/v1`), `response_format`
  (`b64_json|url`, optional), `timeout_ms` (default 180000).
  Output directory and size are not configurable: the model chooses the
  folder per task via the `output_dir` parameter (fallback
  `./generated-images`), and size is passed per call when needed.
- API key resolves `$VAR` through the config resolver (same as `voice.api_key`).
- `generate_image` added to `allToolNames()`.
- `schema.json` regenerated (`prime schema`).

## Tool

`internal/agent/tools/imagegen.go` + `imagegen.md.tpl`:

- Params: `prompt` (required), `size`, `count` (1-10), `output_dir`,
  `filename_prefix`.
- Registered unconditionally in `coordinator.buildTools()`.
- Missing config → text response "image generation is not configured" plus
  markdown link `[Settings](prime://settings/image-generation)`; the agent is
  told to offer configuring instead of retrying.
- Permission request before saving files (action `write`, path = output dir),
  same pattern as bash `execute`.
- Response handling: `b64_json` and `url` formats; extension detected from
  content (png/jpg/webp); HTTP/HTML error bodies are summarized in the error.

## Desktop

- `SettingsDialog.tsx`: new tab "Images" (`image-generation`), component
  `ImageGenSettings.tsx` modeled after `PrimeSettings.tsx`: api key (masked),
  model, base URL, response format select, timeout;
  saves via `configSet` with `options.image_gen.*` keys; scope selector
  (global/project).
- `MessageItem.tsx`: links with `prime://` prefix do not open externally;
  `prime://settings/<tab>` opens `SettingsDialog` at that tab (generic
  mechanism via App-level handler).
- Labels added to the i18n catalog.

## Testing

- Go: config section load + defaults, tool response handling (mock HTTP server),
  permission flow, missing-config message.
- Desktop: manual verification via dev run (build + typecheck).

## Out of scope

- No changes to the standalone image-mcp repository.
- No third-party Go dependencies (stdlib `net/http` only) unless needed.
