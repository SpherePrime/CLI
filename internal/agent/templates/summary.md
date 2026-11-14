You are summarizing a conversation to preserve context for continuing work later.

**Critical**: This summary will be the ONLY context available when the conversation resumes. Assume all previous messages will be lost. Be thorough.

**Required sections**:

## Current State

- What task is being worked on (exact user request)
- Current progress and what's been completed
- Active team assignments (agent names, roles, job IDs, linked reviewers, and owned files)
- Parts that already received VERDICT: APPROVED from a review agent (name each part, reviewer, score/verdict, and files so the resumed coordinator does NOT re-verify, re-test, or re-screenshot them unless modified afterwards)
- Team delegation rules to preserve: code agents write code only and must NOT run tests, builds, linters, dev servers, browsers, or audit scripts; review agents (or the coordinator when no reviewer is used) perform verification
- What's being worked on right now (incomplete work)
- What remains to be done (specific next steps, not vague)

## Files & Changes

- Files that were modified (with brief description of changes)
- Files that were read/analyzed (why they're relevant)
- Key files not yet touched but will need changes
- File paths and line numbers for important code locations

## Technical Context

- Architecture decisions made and why
- Patterns being followed (with examples)
- Libraries/frameworks being used
- Commands that worked (exact commands with context)
- Commands that failed (what was tried and why it didn't work)
- Environment details (language versions, dependencies, etc.)

## Strategy & Approach

- Overall approach being taken
- Why this approach was chosen over alternatives
- Key insights or gotchas discovered
- Assumptions made
- Any blockers or risks identified

## Exact Next Steps

Be specific. Don't write "implement authentication" - write:

1. Assign code agent auth-backend (linked to reviewer auth-review, code only with no self-testing) to add JWT middleware in src/middleware/auth.js:15 and update login handler in src/routes/user.js:45 to return token
2. Skip re-verifying or re-testing parts already marked VERDICT: APPROVED by a review agent unless modified afterwards
3. Verify only unreviewed integration edits via the linked review agent (or directly in the coordinator when no reviewer is used), e.g. checking auth integration with `npm test -- auth.test.js`

**Tone**: Write as if briefing a teammate taking over mid-task. Include everything they'd need to continue without asking questions. No emojis ever.

**Length**: No limit. Err on the side of too much detail rather than too little. Critical context is worth the tokens.
