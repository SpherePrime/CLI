# Reviewing security boundaries

Find concrete exploitable vulnerabilities introduced by changes.
Establish attacker-controlled inputs, privilege boundaries and mitigations. Trace input to SQL, subprocesses, templates, paths, deserialization and unsafe rendering; inspect authorization, sessions, cryptography and secret exposure. Confirm reachability and lack of mitigation. Avoid hardening-only speculation; do not inherit blanket exclusions concealing demonstrable exploitation. Report location, severity, category, exploit scenario, confidence and correction. During read-only security review do not modify files or execute exploit payloads; permitted inspection remains read-only.

Detailed references: read_instruction with `source-doing-tasks-security`.
