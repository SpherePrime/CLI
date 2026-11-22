Fetches raw content from a URL in the requested format (`text`, `markdown`, or `html`, max {{ .MaxFetchSizeKB }}KB); no AI processing.
- Use `agentic_fetch` when you need AI-assisted analysis, extraction, or multi-step web research.
{{- if .GhAvailable }}
- For GitHub content when an exact repo, issue, or PR link is provided, use the `gh` CLI in `bash` instead.
{{- end }}

Sources:
- Only fetch URLs provided by the user, found in local project files, or returned by `web_search`.
- When citing fetched content in your response, include markdown source hyperlinks (`[Title](URL)`).
