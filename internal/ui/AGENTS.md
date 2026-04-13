# UI Agent Guide

## Scope

This file applies to all work under `internal/ui`.

UI code should preserve a terminal-native Papermap experience: clean layout,
strong hierarchy, minimal chrome, and fast keyboard-driven interaction.

Also follow the root `AGENTS.md`.

## UI Goals

- Branded but restrained.
- Works well on common terminal sizes.
- Handles narrow widths without falling apart.
- Keeps important actions obvious.
- Makes streaming output readable.

The UI should feel purpose-built for Papermap, not like a generic demo TUI.

## Preferred UI Structure

Organize UI by screen or focused component:

- `landing/` for first-run and signed-out experience.
- `auth/` for login-specific UI.
- `chat/` for prompt input, transcript, and streamed responses.
- `workspace/` for switching and selection flows.
- `components/` for shared presentational building blocks.

Keep reusable components small and boring. Do not create a component library
for its own sake.

## Bubble Tea Guidance

- Keep `Init`, `Update`, and `View` responsibilities clear.
- Prefer explicit message types for transitions and async results.
- Keep side effects in commands, not in `View`.
- Avoid hidden shared mutable state across models.
- Use focused sub-models only when they reduce complexity.

For async work such as login, workspace loading, or streaming insight results,
model the pending, success, and error states explicitly.

## Layout Guidance

- Prefer centered layouts for landing and login flows.
- Use stable spacing and alignment over decorative complexity.
- Preserve readable margins on wide terminals.
- On smaller terminals, degrade gracefully instead of forcing exact mockups.
- Keep status and key hints concise.

Use lipgloss for layout, spacing, borders, and emphasis. Centralize repeated
style decisions in `internal/theme` once they are shared across screens.

## Interaction Guidance

- Keyboard-first always.
- Make the focused control obvious.
- Keep key bindings consistent across screens.
- Use `Esc` for cancel/back where appropriate.
- Keep loading states visible but low-noise.

Planned default bindings:

- `Enter` submit or confirm.
- `Tab` switch focus in forms.
- `Ctrl+W` switch workspace.
- `Ctrl+L` clear chat.
- `Ctrl+C` quit.
- `Esc` cancel or go back.

## Rendering Insight Output

- Use `glamour` for markdown-like narrative output when it improves clarity.
- Use terminal-friendly tables for structured results.
- Prefer truncation, wrapping, or scrollable viewports over broken layouts.
- Streaming text should remain readable while it updates.

If a rich rendering path becomes fragile, prefer a simpler readable fallback.

## Error States

- Errors should be short, actionable, and visually distinct.
- Authentication failures should not dump raw backend payloads.
- Empty states should guide the next action.
- Offline or retryable states should make recovery obvious.

## Testing Guidance

- Add rendering tests for important views once UI code exists.
- Prefer deterministic view output for golden or snapshot tests.
- If using `catwalk`, keep snapshots focused and intentional.
- Test narrow and wide terminal layouts for critical screens.

## UI Editing Rules

- Avoid over-abstracting styles before patterns repeat.
- Avoid clever animation unless it clearly improves feedback.
- Keep copy concise and product-aligned.
- Preserve accessibility within terminal constraints through contrast,
  hierarchy, and clear focus indication.
