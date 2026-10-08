# Debugging failures

Diagnose and reproduce the failure before choosing a fix.
Record expected versus observed behavior and a minimal reproduction. Inspect errors, warnings, stack traces, changes and configuration; separate evidence from hypotheses. For Prime session problems use prime_info and prime_logs without claiming earlier logging existed. Compare working and failing cases, trace the earliest incorrect value and test one hypothesis at a time. Make a focused correction, add an appropriate regression check and rerun the reproduction. Gather missing evidence rather than repeatedly applying guesses.

Detailed references: read_instruction with `source-troubleshooting-confirmation-policy`.
