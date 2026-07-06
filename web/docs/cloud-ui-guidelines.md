# Cloud UI Guidelines

The main UI borrows the first login prototype's cloud playground language, but keeps dense automation controls easy to scan.

## Visual Tokens

- Background: bright sky gradient in light mode, quiet night sky in dark mode.
- Surfaces: translucent cloud-white cards with 8px radius, soft blue shadow, thin white or sky border.
- Primary action: coral button for the one action that moves work forward.
- Secondary action: cloud-white or pale sky controls.
- Accent: yellow stars and small coral/purple details, used sparingly.
- Data text: navy foreground in light mode, pale sky text in dark mode.

## Layout Rules

- Keep decoration outside data-heavy areas. Use clouds, paper planes, and stars in the shell, empty states, and selected states.
- Decorative motion should be slow and ambient. Paper planes may use gentle long-duration glides, and all motion must respect `prefers-reduced-motion`.
- Do not put cute elements inside logs, tables, or numeric controls unless they clarify state.
- Repeated modules use the same structure: header, optional badges, content surface, empty state.
- Cards stay compact and bounded; no large landing-page hero inside the app workspace.

## Component Rules

- Cards use `Card` defaults; add `cloud-surface` only for larger feature panels.
- Buttons use the `default` variant for commit/save/login/start actions, `outline` for neutral tools, `ghost` for icon-only chrome.
- Badges summarize state and counts; destructive badges are only for blocked/error states.
- Empty states may use one small sparkle/cloud icon plus a short title.
- Inputs, switches, and segmented controls keep stable dimensions to avoid layout shift.
