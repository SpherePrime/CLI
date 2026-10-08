# Using language-server tools

Use semantic navigation and refactoring when configured.
Use lsp_symbols, lsp_definition, lsp_references and lsp_call_hierarchy to locate and trace contracts. Inspect schemas for position indexing; do not assume generic LSP parameters. Use lsp_rename or lsp_replace_symbol after inspecting uses. Check lsp_diagnostics and tests. If unavailable or stale, try lsp_restart where appropriate and fall back to grep plus view. Missing semantic responses do not prove zero references.
