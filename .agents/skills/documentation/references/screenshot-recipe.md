# Screenshot recipe

Use the real TUI. Capture its ANSI output, then render it with Charm's `freeze`.

## 1. Run the real UI

Use a fixed terminal size (120x40 is a good default) in a demo or test config.
For a TUI that keeps running, use a Herdr pane or a disposable terminal session
and let it settle for at least one animation frame.

## 2. Capture ANSI

For Herdr:

```bash
herdr pane read <pane-id> --source visible --format ansi --lines 40 > /tmp/tui.ansi
```

For a disposable terminal session:

```bash
tmux capture-pane -e -p -t <session> > /tmp/tui.ansi
```

Make sure the capture is the alternate-screen app, not the shell prompt.

## 3. Render with freeze

```bash
freeze -x "cat /tmp/tui.ansi" \
  -o docs/images/example.png \
  -b "#07070e" \
  --margin 12 \
  --padding 12 \
  --font.size 13
```

The `-W` and `-H` flags size the final image in pixels, not terminal columns.
Leave them off and let the captured content determine the image size.

## 4. Inspect and resize

Open the image and check that:

- text is readable;
- the screenshot matches the current UI;
- no secrets or unrelated windows appear;
- all images in a set use the same size and style.

Resize a large image with `sips` or `ffmpeg`, preserving aspect ratio. Keep the
README copy around 1200-1600px wide.

## 5. Combine modes (optional)

If ImageMagick is unavailable, use ffmpeg:

```bash
ffmpeg -i a.png -i b.png -i c.png \
  -filter_complex "[0:v][1:v][2:v]hstack=inputs=3" grid.png
```

Keep each input the same dimensions.

## Fallback: macOS screenshot

If `freeze` cannot render the captured ANSI, focus the Herdr tab and take a
macOS screenshot instead:

```bash
herdr tab focus <tab-id>
screencapture -x /tmp/tui-screen.png
```

Crop to the terminal window before using it. Never ship a full desktop capture
with unrelated apps, notifications, or secrets. The screenshot must still show
the real TUI state.

## 6. Verify in the docs check

```bash
.agents/skills/documentation/scripts/check-docs
```
