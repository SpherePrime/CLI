# Reviewing removed behavior

Check guarantees deleted or replaced code enforced.
For each removed guard, validation, error path or test, name its behavior and locate where the new implementation establishes the guarantee. Trace the old failure case through new code. Report a missing equivalent only when it matters to intended behavior. Distinguish requirement changes from regressions and cite the concrete failing case.

Detailed references: read_instruction with `source-code-review-angle-b-removed-behavior-auditor`.
