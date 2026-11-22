CRITICAL: Respond with TEXT ONLY. Do NOT call any tools. Do NOT execute any Next Steps or continue the task. Tools are not available in this summarization step and any tool call will fail.

Your task when summarizing the conversation is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions. This summary will be the ONLY context available when the conversation resumes, so it must be thorough, accurate, and self-contained.

Before providing your final summary, wrap your reasoning in `<analysis>...</analysis>` tags to carefully review the chronological flow, every user message, files touched, errors fixed, and active team state. Then provide the structured summary inside `<summary>...</summary>` tags using the 9 required sections below.

<summary>
1. Primary Request and Intent:
   Capture all explicit user requests, goals, constraints, and acceptance criteria in full detail.

2. Key Technical Concepts:
   List the key technologies, frameworks, libraries, conventions, and architectural concepts discussed.

3. Files and Code Sections:
   List all files read, modified, or created, along with important symbols, line numbers, and code snippets or implementation details needed to continue without re-discovering them.

4. Errors and Fixes:
   Document all errors, failed builds/tests, or edge cases encountered and how they were resolved (or note if still failing).

5. Problem Solving:
   Summarize problems solved, technical decisions made, tradeoffs, and any user feedback or corrections.

6. All User Messages:
   List all user messages verbatim in chronological order (excluding tool results) so the exact user wording and intent are preserved.

7. Pending Tasks:
   List all remaining tasks, todo items with their current statuses, any active team worker IDs/names/roles/statuses (if any), and any parts already marked `VERDICT: APPROVED` so completed work is not needlessly redone.

8. Current Work:
   Describe in detail what was being worked on immediately before this summary, including exact file paths and the current state of edits or checks.

9. Optional Next Step:
   State the single most direct next step aligned strictly with the user's latest explicit request, or note if all requested work is complete.
</summary>

No emojis. Err on the side of thoroughness and precision.
