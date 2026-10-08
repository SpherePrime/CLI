# Skill Code Review Angle C Cross File Tracer

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

### Angle C — cross-file tracer

For each function the diff changes, find its callers (grep for the symbol) and
check whether the change breaks any call site: a new precondition, a changed
return shape, a new exception, a timing/ordering dependency. Also check callees:
does a parallel change in the same PR make a call unsafe?
