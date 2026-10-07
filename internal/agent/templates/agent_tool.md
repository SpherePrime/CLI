Delegate a self-contained job to another agent, then keep working yourself.

Each agent runs on its own model, in its own context, and cannot see this
conversation. Give it a complete task with the context and the deliverable it
needs, and it returns its answer, the files it touched, or what it found.

Reach for this by default. If part of the request is a job rather than a
question, hand that part over: a change confined to one area, a wide search, a
review of what you just wrote, an investigation whose conclusion is all you
need, a bug to fix and verify. Only keep what genuinely depends on the context
you are holding. You stay the one deciding, editing the main files, and talking
to the user.

For a large task, split it up: launch one agent per independent piece in a
single message, with different agent types where they suit. They run in
parallel, which is far faster than doing the pieces one by one, so prefer
launching several at once over a single big sequential job. Each agent
returns on its own; then verify what came back, apply anything that still
needs applying, and finish the task yourself.

Choosing the agent:

- general - carries out a whole job end to end. Can read, edit and run
  commands. Use this for anything that changes files.
- task - reads only, cannot change anything. Use it to search, trace and
  investigate.
- plan - reads only, produces an implementation plan.

Omit "agent" to use the default agent.
