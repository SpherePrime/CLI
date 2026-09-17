# Contributing

## Development

```sh
cargo build --workspace
cargo test --workspace
cargo fmt --all
cargo clippy --workspace -- -D warnings
```

## Guidelines

- Follow Rust idioms; no `unsafe` without justification
- Small focused commits
- New tools: register in `agent-tools`, add a test
- New provider: implement `ModelProvider` in `agent-model`
- Keep the event bus decoupled: no cross-crate direct dependencies outside `agent-sdk`

## Branch naming

`feature/<short-name>`, `fix/<issue>`, `docs/<topic>`

## CI

All changes must pass the full workspace test suite and clippy.
