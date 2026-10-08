# Skill Code Review Phase 2 Verify 3 State

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

## Phase 2 — Verify (1-vote, 3-state)

Dedup candidates that point at the same line/mechanism, keeping the one with
the most concrete failure scenario. For each remaining candidate, when delegation is authorized and useful, run **one
verifier** via agent; otherwise verify directly: give it the diff, the relevant
file(s), and the candidate, and have it return exactly one of:

CONFIRMED: the trigger and consequence follow from inspected code or execution.
PLAUSIBLE: the candidate has a concrete scenario but lacks specified evidence.
REFUTED: code or execution disproves the scenario. Report uncertainty explicitly; do not present PLAUSIBLE as confirmed.

Keep candidates where the vote is CONFIRMED or PLAUSIBLE.
