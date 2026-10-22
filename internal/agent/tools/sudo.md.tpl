Run a shell command with root privileges on this Linux computer through the configured root password. The password is stored by the desktop app and is never shown to you.

Use this tool whenever a command needs elevated rights and the plain bash tool refuses or fails with a permission error. Typical cases: installing or removing system packages (apt, dnf, pacman), managing system services (systemctl), writing to system locations (/etc, /usr, /opt), changing file ownership or permissions outside the project, managing mounts, network configuration, and reading root-only files.

<usage>
- The command runs entirely as root: pass the command WITHOUT the sudo prefix, for example "apt-get install -y ripgrep" instead of "sudo apt-get install -y ripgrep".
- A brief description is required.
- working_dir optional (defaults to current directory); timeout in seconds, default {{ .DefaultTimeout }}, maximum {{ .MaxTimeout }}.
- The bash tool blocks sudo and system administration commands; run those through this tool instead.
- Regular project commands (builds, tests, git) do NOT need this tool: use bash.
- The command is confirmed by the user through the normal permission flow before execution.
- If the password is not configured, the tool returns a settings link: tell the user that root commands need setup and include that exact link in your reply. Do not retry until the settings are saved.
</usage>
