# Review report template

Copy into the final answer (or a `DESIGN-QA.md` file in the project) and fill every row. Write in the user's language. Keep it short: facts and evidence, no praise.

```markdown
## Design QA — <page or feature>

**Verdict:** PASS | PASS WITH ACCEPTED ISSUES | FAIL
**Checked:** <date>, <browser + version>, widths 320 / 390 / 768 / 1280 / 1440 / 1920, light + dark, reduced motion

### Design contract
- Intent: <who, what they must understand or do>
- Signature: <the one memorable idea>
- System: <type pairing, scale ratio, palette roles, spacing base, motion character>

### Automated audit
- Command: `node scripts/audit.mjs <url> --dark`
- Result: <errors> errors, <warnings> warnings, CLS <value>
- Report: <path to report.md>

### Gate results
| Area | Status | Evidence |
| --- | --- | --- |
| Layout integrity (L1–L10) | ✅ / ❌ / ⚠ not verified | <screenshot file or rule output> |
| Content resilience (C1–C7) | | <stress strings used, states shown> |
| Accessibility (A1–A13) | | <contrast pairs, focus walk, zoom test> |
| Visual quality (V1–V14) | | <what was judged on which screenshot> |
| Interaction and motion (I1–I7) | | <actions performed and observed> |
| Runtime and performance (R1–R7) | | <console, measured LCP/CLS> |

### Fixed during QA
- <defect> → <root-cause fix> (<file>)

### Open issues
- P1/P2 <issue>, <why not fixed>, <suggested fix>

### Not verified
- <check>, <reason>
```
