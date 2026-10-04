# Web accessibility (a11y)

**Semantic HTML first.** `<button>` for things that act like buttons,
`<a href>` for navigation, `<nav>`/`<main>`/`<header>`/`<footer>` for
page structure, real `<h1>`–`<h6>` headings in order (don't skip levels
for visual sizing — use CSS for that). A `<div onclick>` styled to look
like a button gets none of a real button's free keyboard/screen-reader
support — you'd have to hand-build all of it.

**Keyboard navigation.** Everything clickable must be reachable and
operable via Tab + Enter/Space, with a visible focus indicator
(`:focus-visible` — don't remove the default outline without replacing
it with something equally visible). Modal dialogs should trap focus
while open and return it to the triggering element on close.

**Images and icons.** `alt` text on every `<img>` that conveys meaning
(describe what it shows, not "image of..."); `alt=""` (empty, not
omitted) for purely decorative images so screen readers skip them.
Icon-only buttons need an accessible label (`aria-label="Close"`), since
there's no visible text for a screen reader to announce.

**Color contrast.** Body text needs at least 4.5:1 contrast against its
background (3:1 for large text) — this is also just good design, see
the css-design skill. Never convey meaning through color alone (a red
border for "error") — pair it with text or an icon, for colorblind users
and anyone on a low-contrast display.

**Forms.** Every input needs an associated `<label>` (via `for`/`id`, or
wrapping), not just a placeholder (placeholders disappear on focus/input
and aren't reliably announced). Error messages should be programmatically
associated with their field (`aria-describedby`) so a screen reader
announces them when the field is focused.

**ARIA is a last resort, not a first tool** — "No ARIA is better than
bad ARIA." Reach for it only when semantic HTML genuinely can't express
the pattern (a custom combobox, a live region for dynamic status
updates), and test with an actual screen reader if the interaction is
non-trivial.
