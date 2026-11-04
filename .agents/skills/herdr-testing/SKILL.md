---
name: Herdr Real Test Loop
description: Use when a change needs a real end-to-end or interactive check in a Herdr workspace, especially for TUIs, audio, browser flows, daemons, or long-running processes. Runs fast checks first, drives the actual app in an isolated Herdr tab, captures pane evidence, and cleans up.
---

# Herdr Real Test Loop

The feedback loop is the product. A change is not done because it compiles or
because the diff looks right. Exercise the real thing.

This skill is project-agnostic. Read the repository's `AGENTS.md` for build,
run, and test commands. If the official `herdr` skill is available, load it for
full CLI details; this skill defines the testing workflow.

## Preconditions

```bash
test "${HERDR_ENV:-}" = 1
command -v herdr
```

If the check fails, stop and say the test must run inside Herdr. Do not fall
back to tmux for a "real" test.

## Step 0 — close the fast loop

Run the project's fastest checks before creating a test tab:

```bash
# Use the commands in AGENTS.md. Typical shape:
go test ./...
go vet ./...
go build ./...
```

Fix failures here first. A real test is not a substitute for unit tests.

## Step 1 — create an isolated test tab

Use the helper, always with `--no-focus` semantics through the helper:

```bash
S=.agents/skills/herdr-testing/scripts/herdr-tab
$S create "test-<change-slug>" "$PWD"
# tab_id=w33:t7
# pane_id=w33:p7
```

Keep the returned `tab_id` and `pane_id`. Never reuse or close the user's tabs.
Never close a tab unless you created it in this test run.

## Step 2 — run the real app

```bash
$S run "$PANE" "<exact run command from AGENTS.md>"
$S wait "$PANE" "<ready marker>" 30000
$S read "$PANE" 160
```

Run long-lived TUI or server processes in the pane. Do not use a one-shot
`go test` here unless the test itself is interactive.

For a TUI:

```bash
$S keys "$PANE" "?"
$S read "$PANE" 160
$S keys "$PANE" "esc"
```

For mouse-only behavior, send real terminal mouse events when appropriate, or
verify the event path with a unit test plus a real terminal smoke test.

## Step 3 — add the real-world stimulus

If the change interacts with the system, create the real stimulus:

- **Audio/video**: open a headed browser or real player in a dedicated test
  pane. Headless automation may not create a system audio stream. Keep volume
  low. Use a short clip or a live stream you can stop.
- **Network**: use the real service, not a mock, after the unit path passes.
- **Filesystem**: use a temporary directory outside the repo.
- **Input**: send keys or mouse events to the app pane and read the result.

For a browser driven by Playwright, prefer a headed browser in a pane. Put the
browser command in the pane's command, not in the app pane.

## Step 4 — assert both directions

Every stateful behavior needs a transition and a recovery:

1. Assert the starting state.
2. Apply the stimulus.
3. Assert the new state from pane output.
4. Remove the stimulus.
5. Assert recovery.

Example shape for this repository: silence → `FILLING`; browser audio →
`OTHER AUDIO`/held; close browser → `FILLING` again. Capture the pane output
for each step.

Use `--format ansi` only when color or layout is the evidence. Use
`recent-unwrapped` for logs and TUI transcripts.

## Step 5 — save evidence and clean up

Save pane reads under a temporary directory, not the repo:

```bash
EVIDENCE="$(mktemp -d)"
$S read "$PANE" 200 > "$EVIDENCE/before.txt"
```

Clean up only your tab:

```bash
$S close "$TAB"
```

If the test fails, read the pane before cleanup, keep the evidence path, fix
the cause, and restart from Step 0. Do not retry blindly.

## Rules

- No tmux for real tests; use Herdr.
- No closing the user's panes, tabs, or workspaces.
- No loud audio tests. Use low volume or a silent test configuration.
- No claiming success without pane output or test output.
- Repeat the loop until the fast checks and the real test both pass.
- Keep project commands in `AGENTS.md`, not in this skill.

## Helper

`scripts/herdr-tab` wraps create/run/keys/wait/read/close and includes a smoke
test:

```bash
.agents/skills/herdr-testing/scripts/herdr-tab smoke
```

Run the smoke test after changing the helper. It creates a tab, proves output,
and closes only that tab.
