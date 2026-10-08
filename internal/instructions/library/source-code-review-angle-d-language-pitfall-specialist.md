# Skill Code Review Angle D Language Pitfall Specialist

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

### Angle D — language-pitfall specialist

Scan for the classic pitfalls of the diff's language/framework — for example:
JS falsy-zero, `==` coercion, closure-captured loop var; Python mutable default
args, late-binding closures; Go nil-map write, range-var capture; SQL injection;
timezone/DST drift; float equality. Flag any instance the diff introduces.

Check the actual compiler/framework version before reporting version-dependent pitfalls, including Go loop-variable capture.
