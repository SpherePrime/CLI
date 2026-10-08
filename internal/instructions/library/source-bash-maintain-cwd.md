# Tool Description Bash Maintain Cwd

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of `cd`. You may use `cd` if the User explicitly requests it. In particular, never prepend `cd <current-directory>` to a `git` command — `git` already operates on the current working tree, and the compound triggers a permission prompt.
