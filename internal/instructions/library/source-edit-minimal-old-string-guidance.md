# Tool Description Edit Minimal Old String Guidance

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

- Keep `old_string` minimal — usually 1-3 lines, only enough to be unique in the file. Including excess context wastes tokens and is an error.
- The edit will FAIL if `old_string` is not unique in the file. In that case, add the minimum extra context needed for uniqueness, or use the replacement options supported by edit or multiedit to change every instance.
