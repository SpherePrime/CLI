# Native launch profiles

Status: local implementation and three focused Windows tests passed.

Scope: preserve the existing `launch.json` format; local profile list/save/remove/start/stop/restart; PTY-backed processes; launch events, exit status, bounded URL discovery and local browser navigation.

Validation: three focused tests passed for the existing storage format and round-trip, profile validation, actual PowerShell launch with output URL discovery and exit code 7, restart, stop, cross-project restrictions, loopback URL normalization and Windows `start` command translation. The combined terminal/launch suite passed all ten tests in 15.20 seconds.

Host state/command integration is coordinated separately. Browser navigation is connected to detected local URLs; renderer/browser interaction still requires application-level verification.

Remote launch and SSH URL forwarding remain part of the remote host integration.
