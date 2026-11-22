Fetches content from a specified URL and converts HTML to readable markdown; for use inside sub-agents.
- Use this tool when you need to retrieve and analyze web page content or documentation from a URL.
- Large pages (>50KB) are saved to a temporary file; use `view` and `grep` to inspect the saved file.
{{- if .GhAvailable }}
- For GitHub content when an exact repo, issue, or PR link is provided, use the `gh` CLI in `bash` instead.
{{- end }}

Sources:
- Only fetch URLs provided by the user, found in local project files, or returned by `web_search`.
- When citing fetched pages in your response, include markdown source hyperlinks (`[Title](URL)`).
