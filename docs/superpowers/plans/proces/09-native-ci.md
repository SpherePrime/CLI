# Native platform checks

Status: in progress.

Added a separate Windows/Linux CI workflow for the native renderer, Go shared library, backend lifecycle checks and Rust library tests/build. It does not publish a release or replace the existing installer workflow.

Pending: push the compiled native host and observe both CI jobs; fix platform failures before recording either platform as validated. ARM64 runners and signed install/update validation remain separate gates.
