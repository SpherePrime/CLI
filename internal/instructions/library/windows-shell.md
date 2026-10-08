# Using PowerShell on Windows

Use native PowerShell syntax only when explicitly invoking an available PowerShell executable. Prime's bash tool itself remains Bash-compatible on Windows.
Use $env:NAME and the call operator for executable paths containing spaces. Pipes carry objects; use Where-Object and Select-Object. Check edition before newer operators and $LASTEXITCODE after native commands. Use literal here-strings or safe body files for multiline content. Avoid reserved variables including $HOME. Before recursive deletion or moving verify resolved targets stay in the intended directory; use native cmdlets with -LiteralPath throughout. Launch background GUI helpers hidden unless visibility is needed.

Detailed references: read_instruction with `source-powershell-edition-for-7`, `source-powershell-edition-for-5-1`.
