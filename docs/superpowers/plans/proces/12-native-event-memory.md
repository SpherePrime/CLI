# Native streaming event memory

Status: implementation and focused checks passed.

Ported the existing 75 ms updated-message coalescing policy to Rust. Created/deleted messages and non-message events preserve immediate ordering; pending snapshots are bounded by 64 IDs and 8 MiB. Idle subscriptions flush without waiting for another server event. This prevents sending every token snapshot through the WebView bridge.

Validation: both burst/byte-budget/ordering tests passed in the combined Windows run. A real native renderer received message/completion events and reloaded persisted history after a provider response. Long-running memory comparison remains a separate migration gate.
