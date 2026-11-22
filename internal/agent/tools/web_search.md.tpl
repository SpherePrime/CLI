{{- if .ProviderSearch -}}
Search the web via the configured provider's search engine; returns grounded results, titles, URLs, and snippets.
{{- else -}}
Search the web via DuckDuckGo; returns titles, URLs, and snippets.
{{- end }}
Use this when you need up-to-date information beyond your knowledge cutoff, recent documentation, or current online facts.
Follow up with {{if .SubAgent}}web_fetch{{else}}fetch{{end}} to get full page content.
{{- if .GhAvailable }} For GitHub searches when an exact repo name, issue, or link is provided, use `gh search` in `bash` instead.{{- end }}

Sources:
- When answering using information from `web_search`, include a `Sources:` section at the end of your response listing the relevant source titles as markdown hyperlinks (`[Title](URL)`).
