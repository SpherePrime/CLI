# Native terminal

Status: Windows PTY implementation and seven focused tests passed.

Scope: native local PTY lifecycle matching the existing terminal request/event contract, Unicode output, bounded journal, ownership handoff, workspace scoping, resize/write/report, process-tree cleanup and application shutdown.

Implementation uses portable-pty 0.9, Windows ConPTY with a kill-on-close Job object, and Unix process-group cleanup. Remote SSH terminals and launch profiles remain separate work.

Validation: seven focused Rust tests passed after the ConPTY startup cursor-position handshake was implemented. The checks include actual PowerShell Unicode output, ownership/workspace restrictions, resize, exit code 7 with retained output, UTF-8 decoding, output tail bounds, fragmented startup cursor queries, and termination of a real spawned descendant on shutdown. Initial runs stalled on ConPTY's startup `ESC[6n`; the host now answers that request before allowing user commands. The combined terminal/launch suite passed all ten tests in 15.20 seconds.

Input uses a bounded queue and a five-second timeout. Windows shutdown waits for the Job object's active process count to reach zero; readers and writers have bounded shutdown waits.

The host now registers the scoped terminal agent callbacks through the native workbench adapter; manager checks still enforce explicit user ownership transfer.

Remaining: full Rust suite, Linux/macOS runtime checks, renderer interaction tests and remote SSH terminal integration.
