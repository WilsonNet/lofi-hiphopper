---
name: lofi-hiphopper
description: A late-night terminal instrument panel for filling silence with lofi.
colors:
  ultraviolet: "#c9a7ff"
  hot-pink: "#ff86c2"
  peach-glow: "#ffc48a"
  mint-signal: "#7fe7d4"
  signal-blue: "#6d8bff"
  midnight: "#07070e"
  panel-ink: "#0d0d18"
  panel-raised: "#131322"
  hairline: "#23233a"
  code-void: "#090911"
  ink: "#e9e4ff"
  muted: "#8a85a3"
  dim: "#7c7799"
  success: "#8fe7b0"
  cream: "#fff3dd"
typography:
  display:
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Monaco, Cascadia Code, Roboto Mono, monospace"
    fontSize: "clamp(2.6rem, 4.8vw, 5.4rem)"
    fontWeight: 700
    lineHeight: 1.02
    letterSpacing: "-0.045em"
  headline:
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Monaco, Cascadia Code, Roboto Mono, monospace"
    fontSize: "clamp(1.7rem, 2.8vw, 2.8rem)"
    fontWeight: 700
    lineHeight: 1.08
    letterSpacing: "-0.03em"
  title:
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Monaco, Cascadia Code, Roboto Mono, monospace"
    fontSize: "0.98rem"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Avenir Next, Segoe UI, system-ui, -apple-system, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.65
    letterSpacing: "0em"
  lede:
    fontFamily: "Avenir Next, Segoe UI, system-ui, -apple-system, sans-serif"
    fontSize: "clamp(1.05rem, 1.2vw, 1.18rem)"
    fontWeight: 400
    lineHeight: 1.7
    letterSpacing: "0em"
  label:
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Monaco, Cascadia Code, Roboto Mono, monospace"
    fontSize: "0.74rem"
    fontWeight: 400
    lineHeight: 1.65
    letterSpacing: "0.12em"
  terminal:
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Monaco, Cascadia Code, Roboto Mono, monospace"
    fontSize: "0.86rem"
    fontWeight: 400
    lineHeight: 1.65
    letterSpacing: "0em"
rounded:
  micro: "3px"
  control: "4px"
  base: "6px"
  terminal: "12px"
  pill: "999px"
spacing:
  xxs: "4px"
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "38px"
  2xl: "56px"
components:
  button-primary:
    backgroundColor: "{colors.ultraviolet}"
    textColor: "{colors.midnight}"
    rounded: "{rounded.base}"
    padding: "9px 14px"
  button-secondary:
    backgroundColor: "{colors.panel-ink}"
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    padding: "9px 14px"
  terminal-shell:
    backgroundColor: "{colors.midnight}"
    textColor: "{colors.ink}"
    rounded: "{rounded.terminal}"
    padding: "0px"
  panel:
    backgroundColor: "{colors.panel-ink}"
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    padding: "14px 11px 11px"
  code-block:
    backgroundColor: "{colors.code-void}"
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    padding: "16px 18px"
  kbd:
    backgroundColor: "{colors.panel-raised}"
    textColor: "{colors.ultraviolet}"
    rounded: "{rounded.micro}"
    padding: "0 0.3em"
---

# Design System: lofi-hiphopper

## Overview

**Creative North Star: "The Late-Night Terminal"**

The system is a dark room with one screen awake. Most surfaces are near-black,
flat, and separated by a single hairline; the terminal preview is the only
object allowed to cast a deep shadow and a violet glow. The page should feel
like a quiet desk at 1 a.m., not a product launch.

The voice is calm, nocturnal, and quietly playful. Mono type carries the
interface and the headlines, so the page reads as software rather than a
brochure. Avenir Next appears only for human explanation: the lede, section
intros, card body, FAQ answers. That split keeps the terminal authority without
making long prose feel like a changelog.

Color behaves like a spectrum, not a brand wash. Ultraviolet, hot pink, and
peach are concentrated in the signal: the colored headline, the primary
action, the visualizer, the live dots. The rest of the page stays in midnight,
ink, muted grey, and one hairline violet. Motion is scarce. The spectrum, the
pulsing live dot, and a one-pixel hover lift are the whole vocabulary.

**Key Characteristics:**
- Near-black field with a faint violet and pink atmospheric wash.
- Hairline-bordered instrument panels, not floating cards.
- Mono for structure, sans for explanation.
- One glowing terminal as the focal object.
- Spectrum color concentrated in signal and action, never spread as page tint.
- Still page; motion belongs to live data.

## Colors

An ultraviolet spectrum on midnight: cool violet structure, hot pink and peach
heat, mint reserved for configuration and success.

### Primary
- **Ultraviolet** (`#c9a7ff`): The structural accent. Panel titles, links, key
  caps, and focus-adjacent details. It is the color that makes the terminal
  feel powered on.

### Secondary
- **Hot Pink** (`#ff86c2`): The live signal. Playing dots, mode labels, the
  second line of the hero headline, the GitHub link, the middle of the spectrum.
- **Peach Glow** (`#ffc48a`): The warm end of the spectrum. Inline code, focus
  ring, the spectrum tail. Use sparingly; it is heat, not a surface color.

### Tertiary
- **Mint Signal** (`#7fe7d4`): Configuration and secondary status. The
  `config.yaml` panel title and any calm, non-live technical accent.
- **Signal Blue** (`#6d8bff`): Visualizer-only support color. Not a UI accent.

### Neutral
- **Midnight** (`#07070e`): Page field. Never pure black.
- **Panel Ink** (`#0d0d18`): Panel and card base.
- **Panel Raised** (`#131322`): Hover and pressed control surface.
- **Code Void** (`#090911`): Code blocks, one step darker than panels.
- **Hairline** (`#23233a`): Every border and divider. One pixel, never thicker.
- **Ink** (`#e9e4ff`): Primary text. Slightly violet, never pure white.
- **Muted** (`#8a85a3`): Body prose, secondary labels, navigation at rest.
- **Dim** (`#7c7799`): Metadata, hints, footer, and non-essential labels. It must still clear 4.5:1 on the page field.
- **Success** (`#8fe7b0`): Live and silence-ready status dots.
- **Cream** (`#fff3dd`): The brightest highlight, used on hover text and dialog
  titles. Less than a handful of uses per page.

### Named Rules
**The One Lamp Rule.** Only the terminal preview gets a deep shadow and a broad
violet glow. Every other surface is flat with a hairline border. If a second
object glows, the room is no longer dark.

**The Spectrum Is The Accent Rule.** Ultraviolet, hot pink, and peach belong to
signal, action, and the headline accent. They never become a full-page
background tint. Keep accent coverage under roughly 10% of any viewport.

## Typography

**Display Font:** ui-monospace / SF Mono (with Menlo, Monaco, Cascadia Code)
**Body Font:** Avenir Next (with Segoe UI, system-ui, sans-serif)
**Label/Mono Font:** The same mono stack, tracked and uppercased.

**Character:** Terminal precision with a human explanatory voice. Mono is the
instrument; Avenir Next is the person reading the manual. The pairing should
feel technical but not cold.

### Hierarchy
- **Display** (700, `clamp(2.6rem, 4.8vw, 5.4rem)`, 1.02, -0.045em): The hero
  only. Tight, almost stacked; the line break is part of the composition.
- **Headline** (700, `clamp(1.7rem, 2.8vw, 2.8rem)`, 1.08, -0.03em): Section
  titles. Sentence case, short, declarative.
- **Title** (700, `0.98rem`, 1.3, -0.01em): Feature and command headings.
- **Body** (400, `1rem`, 1.65): Explanation and FAQ answers, set in Avenir
  Next. Cap prose at roughly 55–62ch.
- **Lede** (400, `clamp(1.05rem, 1.2vw, 1.18rem)`, 1.7): Hero description,
  capped at 54ch.
- **Label** (400, `0.74rem`, 0.12em, uppercase): Kickers, panel titles,
  metadata. The mono voice at its smallest.

### Named Rules
**The Two-Voice Rule.** Mono sets the interface, headlines, labels, code, and
terminal content. Sans sets paragraphs a person reads. Never set a long
paragraph in mono; never set a terminal label in sans.

**The Tight Display Rule.** Large mono headings close their tracking
(-0.03em to -0.045em) and sit near 1.0 line-height. Small mono labels open
their tracking (+0.12em to +0.13em) and stay uppercase. The scale contrast is
the hierarchy.

## Layout

A fluid, full-bleed workspace with no max-width. The page gutters are
`clamp(18px, 2.6vw, 36px)`, so the composition grows with the monitor: at
1440px the content fills 1429px, and at 1920px it fills 1909px. The page is a
persistent 58px command bar, a `clamp(184px, 15vw, 224px)` session rail, and a
single fluid main pane. The rail is a hairline-bordered column at least as tall
as the viewport.

The hero uses a `1.12fr / 0.88fr` grid for copy and the install panel, then a
full-width terminal beneath. Section heads use `1fr / 0.82fr`; behavior output is one three-column table;
screens use `1.45fr / 1fr`; install uses `1.05fr / 0.95fr`; FAQ is a single
help-output column. The main column has no fixed width. Prose caps at 54–55ch
inside the fluid pane, so text remains readable at ultrawide sizes.

The root type size is `clamp(18px, 0.5vw + 12px, 20px)`, so all rem-based
spacing and type scale with the viewport. At 1440px the root is 19.2px; at
390px it is 18px.

Breakpoints:
- **1200px:** the session rail hides and its navigation returns to the top bar.
- **1040px:** the hero becomes one column; header metadata hides.
- **900px:** behavior rows collapse to key plus copy; screens and install become
  one column.
- **860px:** the top navigation wraps to its own scrollable row; the terminal
  panels stack.
- **640px:** gutters drop to 18px; hero actions stack; signal panel and
  keyboard hints hide; visualizer shrinks to 210px.
- **420px:** brand and navigation tighten; radio list truncates to three items.

**The Hairline Rule.** Structure comes from 1px `#23233a` borders and spacing,
not from shadows or background fills. A panel should still read as a panel in
grayscale.

## Elevation & Depth

Flat by default. Panels, cards, and code blocks are tonal rectangles with a
hairline border and no shadow. Depth is reserved for the one object that is
supposed to feel physically present: the terminal.

### Shadow Vocabulary
- **Terminal ambient** (`box-shadow: 0 28px 90px rgba(0, 0, 0, 0.55), 0 0 80px rgba(123, 92, 255, 0.12)`):
  Lifts the terminal off the page and adds the room's violet spill.
- **Terminal rim** (`box-shadow: 0 0 0 1px rgba(201, 167, 255, 0.04)`): A faint
  edge highlight, not a second border.
- **Terminal inset** (`box-shadow: inset 0 0 60px rgba(201, 167, 255, 0.045)`):
  Screen bloom inside the shell.
- **Dialog lift** (`box-shadow: 0 30px 80px rgba(0, 0, 0, 0.65)`): The shortcut
  dialog only, because it sits above a dimmed page.
- **Status glow** (`box-shadow: 0 0 12px rgba(143, 231, 176, 0.85)`): Live and
  silence dots. A signal light, not decoration.

### Named Rules
**The One Lamp Rule.** A deep shadow plus a broad glow is a terminal-only
treatment. Cards may lift 2px on hover and change border color; they never gain
a shadow.

## Shapes

Small radii, hard edges, and instrument geometry. Most controls, panels, and
code blocks sit at 6px; the terminal shell at 12px. Inline code and key caps use
3px; copy buttons and the install field 4px. Pills are reserved for the hero
status chip (`999px`).

Borders are always 1px. Panel titles overlap the top border, sitting on a
`#0d0d18` background chip so the rule appears to break around the label. Key
caps use a 1px border with a darker bottom edge to read as physical keys.

The recurring silhouette is the terminal window: a wide rounded rectangle with
a thin title bar, three traffic dots, and a dense interior grid. Decorative
forms stay geometric and sparse; no blobs, no oversized circles, no soft
gradient orbs.

## Components

### Buttons
- **Shape:** Compact rectangles, 6px radius, 9px × 14px padding.
- **Primary:** Solid ultraviolet with midnight text, 700 weight. Hover shifts
the fill to hot pink. It is the only filled action on the page.
- **Hover / Focus:** Primary shifts to hot pink and lifts 1px. Secondary shifts
  border to ultraviolet, background to panel-raised, and lifts 1px. Focus uses a 2px peach outline with a 4px offset.
- **Secondary:** Panel-ink fill, hairline border, ink text. Used for "Download
  release" and "View on GitHub".

### Cards / Containers
- **Corner Style:** 6px for panels, code blocks, and screenshot images; 12px for
  the terminal. FAQ rows are hairline dividers, not boxes.
- **Background:** Panel-ink at 94% opacity for instrument panels; screenshot
  figures sit directly on the page field.
- **Shadow Strategy:** None. See Elevation.
- **Border:** 1px hairline. Panels and buttons may brighten the border on hover.
- **Internal Padding:** 14px × 11px × 11px for terminal panels; 13px 0 for FAQ
  rows; 16px 18px for code blocks.

### Inputs / Fields
- **Style:** The install command is a bordered field with panel-ink fill and a
  compact copy button. The button uses panel-raised fill, ultraviolet text, a
  4px radius, and a hairline border.
- **Focus:** Global `:focus-visible` uses a 2px peach outline at 4px offset.
- **Behavior:** Copy shows a fixed success toast at the bottom right; the
  button label changes to "copied" for 1.8s. No layout shift.

### Navigation
- **Top bar:** Mono 0.82rem, muted grey, 2px gaps, no underlines. The active
  section shifts to ink; the GitHub link is hot pink at rest. A hairline left
  border separates `v0.3.2 · macOS`; it hides below 1040px.
- **Session rail:** A sticky 184–224px column with numbered section links,
  live status, and key hints. The active link takes a violet gradient wash.
- **Mobile:** Below 1200px the rail hides and its links return to the top bar.
  Below 860px the top nav becomes a full-width scrollable row. Below 480px the
  GitHub link hides so the four section links keep larger touch targets.

### Terminal Preview (Signature)
A tabindex-focusable terminal window with a 1px `#2d2b47` border, 12px radius,
and the only deep shadow on the page. The title bar carries traffic dots, a
`120×40` label, and a `DEMO` badge. Inside: a radio list, signal readout, and a
live canvas visualizer in a 220px/fluid two-column grid. Panel titles overlap
their top borders. The canvas is clickable and supports `h` / `l`; the demo is
decorative and must never imply real system audio.

### Key Caps (Signature)
Small mono caps with 3px radius, panel-raised fill, ultraviolet glyphs, and a
1px border whose bottom edge is darker. They are used inside prose and terminal
hints, never as buttons.

## Do's and Don'ts

### Do:
- **Do** keep the page field at Midnight `#07070e`; use Panel Ink `#0d0d18` for
  contained surfaces.
- **Do** use 1px Hairline `#23233a` borders for all structure.
- **Do** set headlines and interface labels in the mono stack, and long prose
  in Avenir Next or the system sans fallback.
- **Do** keep the terminal as the only object with a deep shadow and broad
  glow.
- **Do** concentrate ultraviolet, hot pink, and peach in the signal: colored
  headline, primary action, visualizer, live dots.
- **Do** use real TUI screenshots and label demo-mode captures.
- **Do** preserve keyboard access, visible focus, and reduced-motion support.

### Don't:
- **Don't** use pure black `#000` or pure white `#fff`.
- **Don't** give cards, panels, or buttons a drop shadow.
- **Don't** wash whole sections in violet or pink gradients.
- **Don't** use radii above 12px except the hero status pill.
- **Don't** add scroll reveals, parallax, or ambient motion. Motion belongs to
  live data and state changes.
- **Don't** set body paragraphs in mono or terminal labels in sans.
- **Don't** invent testimonials, user counts, benchmarks, or product claims.
