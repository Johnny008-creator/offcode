# Debugging methodology

**Reproduce first.** Before changing anything, get a reliable way to
trigger the bug — "it fails sometimes" is much harder to fix than "it
fails every time I run X with input Y." If it's intermittent, find what
varies between the failing and passing runs (timing, input size,
concurrency) before trying fixes.

**Read the actual error**, don't skim past it. The exact message, the
exact line number, the exact stack trace — most of the time the real
cause is stated plainly and the temptation to skip straight to guessing
causes wasted cycles.

**Bisect, don't shotgun.** If the bug appeared somewhere between a
working state and now, binary-search the commits (`git bisect`) or the
code path (comment out/isolate half the suspected logic at a time)
rather than changing several things at once and hoping. Changing
multiple things at once means you won't know which change actually
fixed it, which risks reintroducing the bug later.

**Form a hypothesis, then test it specifically.** "I think X is null at
this point" → add a print/log/breakpoint that confirms or denies exactly
that, don't add fixes for guessed causes without first confirming the
guess.

**Check your assumptions against reality, not memory.** If you believe
a function returns a certain shape, a file exists at a certain path, or
a config value is set a certain way — verify it directly (read the
actual file, print the actual value) rather than trusting what you
recall writing or what a doc claims, especially if the surrounding code
has changed since.

**Known anti-patterns:** making a change "just in case" without
understanding why it would fix anything; adding a broad try/catch that
swallows the error instead of fixing the cause; re-running the exact
same failing command repeatedly hoping it passes (if it's not flaky,
nothing changes); fixing the symptom at the call site instead of the
actual defect in the called code.

**Once fixed:** add a regression test for it if one doesn't already
exist — the whole point of finding a bug is to make sure it doesn't
come back silently.
