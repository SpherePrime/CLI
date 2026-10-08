# Reviewing cross-file contracts

Trace changed APIs through callers, callees and wrappers.
Use lsp_references or grep to check new preconditions, return shapes, errors and ordering. Inspect parallel changes altering contracts. Verify adapters, caches and decorators forward to their delegate rather than re-entering a global registry and recursing. Check language pitfalls against the actual version: coercion, nil maps, captures, timezone drift and numeric precision. Do not report obsolete pitfalls without confirming applicability.

Detailed references: read_instruction with `source-code-review-angle-c-cross-file-tracer`, `source-code-review-angle-e-wrapper-proxy-correctness`, `source-code-review-angle-d-language-pitfall-specialist`.
