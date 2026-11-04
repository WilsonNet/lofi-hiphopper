# Portable agent layer

This directory is intentionally tool-agnostic. It contains workflows that any
coding agent can read, not vendor-specific configuration.

- `skills/<id>/SKILL.md` is the portable skill format. OpenCode discovers this
  path as a project compatibility source; other agents can read the same files.
- Skills may keep scripts, references, and templates beside `SKILL.md`.
- Project commands and invariants live at the repository root in `AGENTS.md`.
  Skills stay reusable; `AGENTS.md` supplies the project-specific specifics.
- Tool-specific files (for example `.opencode/`) should be thin pointers, not
  duplicate copies of these instructions.

## Skills

| Path | Purpose |
| --- | --- |
| `skills/herdr-testing/SKILL.md` | Run real interactive tests in isolated Herdr tabs and capture evidence. |
| `skills/documentation/SKILL.md` | Write human-friendly README/docs and capture real TUI screenshots with Charm freeze. |

## Verify the testing helper

```bash
.agents/skills/herdr-testing/scripts/herdr-tab smoke
```

## Verify documentation links

```bash
.agents/skills/documentation/scripts/check-docs
```
