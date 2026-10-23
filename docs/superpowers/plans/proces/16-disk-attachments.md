# Disk-backed native attachments

Status: investigation.

The preview currently retains base64 content in Rust and copies it through JSON into the Go ABI. A batch can reach 200 MiB while the ABI request limit is 32 MiB. Raising the request limit would increase transient memory and does not solve the problem.

Next: private disk storage, small preview images, trusted file references resolved inside the embedded engine, bounded reads, release and crash cleanup. References must not enable arbitrary file reads through the ordinary HTTP API. Preserve existing image formats and attachment limits.

Checks: pending large image batches, release cleanup, invalid references and provider delivery.
