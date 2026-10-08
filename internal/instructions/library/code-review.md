# Reviewing correctness

Review the requested diff and verify actionable failure scenarios.
Inspect status, staged and unstaged changes and the specified revision range using bash; verify the base exists. Include relevant new files. Trace changed functions and contracts, deduplicate by mechanism and classify candidates as confirmed, plausible with explicit missing evidence, or refuted. Sweep for missed changes. Report introduced bugs with priority, exact file and minimal line range, concrete trigger, consequence and feasible correction. Do not edit during review-only assignments.

Detailed references: read_instruction with `source-code-review-phase-0-gather-the-diff`, `source-code-review-phase-2-verify-3-state`.

Detailed reference: read_instruction with `source-code-review-phase-3-sweep-for-gaps`.

Detailed reference: read_instruction with `source-code-review-inline-gap-sweep-phase`.
