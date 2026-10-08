# Reviewing efficiency

Identify measured or demonstrable wasted work.
Inspect repeated I/O, redundant computation, serialized independent operations and hot-path or startup blocking. Check long-lived closures retaining oversized scopes; store only needed fields when that solves actual retention. Measure cost, identify a cheaper alternative and compare behavior and resources. Avoid premature optimization obscuring bounded work.

Detailed references: read_instruction with `source-code-review-efficiency-dimension`.
