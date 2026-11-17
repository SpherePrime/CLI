{{- if .ProviderSearch -}}
Search the web via the configured provider's search engine; returns a grounded summary, titles, URLs, and snippets. Follow up with {{if .SubAgent}}web_fetch{{else}}fetch{{end}} to get full page content.
{{- else -}}
Search the web via DuckDuckGo; returns titles, URLs, and snippets. Follow up with {{if .SubAgent}}web_fetch{{else}}fetch{{end}} to get full page content.
{{- end -}}
{{- if .GhAvailable }} For GitHub searches when an exact repo name, issue, or link is provided, use `gh search` in bash instead.{{- end }}
