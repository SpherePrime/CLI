Delegate a self-contained job to another agent, then keep working yourself.

Each agent runs on its own model, in its own context, and cannot see this
conversation. Give it a complete task with the context and the deliverable it
needs, and it returns its answer, the files it touched, or what it found.

Delegation is the default. Do the work yourself only when the user explicitly
says not to use agents or to do it yourself, or when the task is trivial (a
question, a few-line edit, a single command). You stay the one deciding,
integrating and talking to the user.

Default flow: when the project context is not known yet, launch explore agents
first and wait for their reports. Then split the work into independent parts
with no overlapping files and launch one code agent per part in a single
message with background=true on every assignment. Give every agent its own
distinct assignment and put the shared conventions (paths, interfaces, naming,
styles) into each one so every agent can start immediately. When all results
are in, integrate them, verify the whole result and finish the task yourself.

Choosing the agent:

- explore - collects project context before the work is split: structure,
  conventions, build and test commands, relevant code, a suggested split.
  Does not change files.
- code - carries out a whole job end to end. Can read, edit and run
  commands. Use this for anything that changes files.
- review - strict independent check of finished work: builds, tests, runs
  it, takes and inspects screenshots, grades it and returns VERDICT,
  SCORE, LIKED, DEFECTS. Does not change files. Give it the original request,
  acceptance criteria, the part, its author and the files involved.
- task - reads only, cannot run commands. Quick search and tracing.
- plan - reads only, produces an implementation plan.

Omit "agent" to use code, the editing agent. Give each agent a short name and a complete assignment with paths, constraints, and verification requirements. Write the name and assignment in English regardless of the conversation language, and state the user's language in the assignment (for example "User language: Russian") so the agent writes user-facing text, UI strings and docs in that language. The primary chat coordinates and integrates the results.

The team defaults to ten simultaneously running agents. Further assignments wait in a queue. Agents cannot delegate; they communicate through agent_message and request further assignments from the primary chat. Use agent_message to coordinate peers or continue an existing agent conversation, including sending review defects back to the code agent that built that part. Do not poll agent_jobs or wait for agents: end your turn, and their results and messages resume the primary chat automatically as one team notice. Agent failures and cancellations reach the primary chat's active turn at its next step; continue the stopped work or assign it again instead of waiting for that agent. Do not restart completed work.
