# Skills

A skill is a package of prompt instructions, tools, hooks, and resources that extends the agent.

## Layout

```
skills/
  react/
    SKILL.md
    config.json
    scripts/
  testing/
    SKILL.md
```

## SKILL.md format

```markdown
---
tools: read_file, run_command
hooks: on_save
resources: templates/
---
# Instructions
...
```

## CLI

```sh
agent skill list
agent skill install <name>
agent skill remove <name>
agent skill enable <name>
agent skill disable <name>
agent skill inspect <name>
```

## Global vs project

Global skills live in `~/.agent/skills/`. Project skills live in `./agent/skills/` and override globals with the same name.
