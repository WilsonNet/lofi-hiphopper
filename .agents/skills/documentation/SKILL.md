---
name: Documentation
description: Use when writing or changing a README, docs/ pages, user guides, release notes, or screenshots. Keeps the README human-friendly, moves deep material into docs/, captures real TUI screenshots with Charm freeze, and verifies links and commands.
---

# Documentation

Write for two audiences:

- `README.md` is for a person deciding whether to use the project. It should be
  short, friendly, visual, and start with the result.
- `docs/` is for a person maintaining or extending it. Put architecture,
  configuration references, internals, testing strategy, and troubleshooting
  there.

Do not turn the README into an architecture document. Link to the deep pages.

## Workflow

1. Read `AGENTS.md` and confirm the current commands by running them.
2. Write or update the README first:
   - one-sentence purpose;
   - a real screenshot near the top;
   - quick start;
   - the few controls a new user needs;
   - requirements and caveats;
   - links to `docs/`.
3. Move deep material into `docs/`:
   - `architecture.md` for data flow and invariants;
   - `configuration.md` for every config field;
   - `testing.md` for the feedback loop and real tests;
   - `development.md` for local setup and extension points.
4. Capture screenshots from the real TUI. Use `freeze`; see
   `references/screenshot-recipe.md`. Never mock a screenshot.
5. Run the docs check:
   ```bash
   .agents/skills/documentation/scripts/check-docs
   ```
6. Re-run the quick-start commands in a clean shell or temp HOME.
7. If you added or renamed a skill, update the skill index in `AGENTS.md` with
   the path and a one-line summary.

## README rules

- Use plain language. Explain the user-visible behavior before the implementation.
- Put the most important command within the first screenful.
- Prefer short tables for keys and options.
- Every screenshot needs useful alt text and a caption or surrounding context.
- Keep images in `docs/images/` and reference them relatively.
- Do not duplicate full config tables in the README; link to
  `docs/configuration.md`.
- Do not include generated binaries, local absolute paths, or secrets.

## Docs rules

- State the audience at the top of each deep page.
- Include copy-pasteable commands.
- Document defaults and override rules explicitly.
- Keep architecture claims testable; name the package and the invariant.
- Include failure modes and recovery, not only the happy path.
- Add a link back to the README or the docs index.

## Screenshot rules

- Capture a real frame from the running TUI, not a hand-written mock.
- Use one terminal size and one theme for a set.
- Show a meaningful state: playing, help open, compact layout, or a specific
  visualizer.
- Keep the image readable after resizing; do not ship a 4000px-wide image when
  1200px is enough.
- Verify the image yourself before committing it.
- Never capture personal notifications, secrets, or unrelated windows.

## Verification

Docs are code-adjacent. The check is not complete until:

- local links and image paths resolve;
- commands work from a clean checkout or temp HOME;
- screenshots match the current UI;
- the README quick start is internally consistent;
- `AGENTS.md` lists every skill with its path and summary.
