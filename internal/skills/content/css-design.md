# CSS / visual design

Concrete patterns for making generated UI look deliberately designed,
not like a first draft.

**Spacing.** Pick one spacing scale and use only values from it — e.g.
4/8/12/16/24/32/48/64px. Never arbitrary values like 13px or 22px.
Consistent spacing is the single biggest "does this look professional"
signal.

**Typography.** One font family for UI text (system font stack is fine:
`-apple-system, "Segoe UI", Roboto, sans-serif`), a second only for
headings if you want contrast. Limit yourself to 3–4 font sizes per page.
Line-height 1.4–1.6 for body text, tighter (1.1–1.3) for large headings.

**Color.** Never pure black (#000) on pure white (#fff) — use something
like #1a1a1a on #fafafa, easier on the eyes. Build a small palette: one
accent color, 2–3 neutrals/grays for text and borders, and semantic colors
(success/warning/error) only when actually needed. Check contrast — body
text should hit at least 4.5:1 against its background.

**Layout.** Flexbox for one-dimensional rows/columns, grid for anything
two-dimensional (cards, dashboards). Max-width a centered content column
(640–800px for reading text, wider for dashboards) rather than letting
text stretch edge-to-edge on wide screens.

**Dark mode.** If you support it, define colors as CSS variables on
`:root` and redefine under `@media (prefers-color-scheme: dark)` — never
hardcode colors in individual rules, or dark mode becomes a second pass
instead of a toggle.

**Interactive states.** Every clickable element needs a visible `:hover`
and `:focus-visible` state — don't rely on cursor:pointer alone. Buttons
need a `:disabled` state that's visibly different (lower opacity, no
pointer).

**Common tells of unstyled/lazy output to avoid:** default browser blue
links, unstyled `<button>`/`<select>` elements sitting next to styled
ones, inconsistent border-radius values across a page, box-shadow applied
everywhere instead of selectively, text touching the edge of its
container with no padding.
