# Skill Code Review Angle B Removed Behavior Auditor

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

### Angle B — removed-behavior auditor

For every line the diff DELETES or replaces, name the invariant or behavior it
enforced, then search the new code for where that invariant is re-established.
If you can't find it, that's a candidate: a removed guard, a dropped error
path, a narrowed validation, a deleted test that was covering a real case.
