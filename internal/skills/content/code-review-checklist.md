# Code review checklist

Run through these when reviewing a diff (your own or someone else's):

**Correctness.** Does it actually do what the PR/commit message claims?
Trace through at least one real example by hand, don't just read the
code and assume. Check edge cases: empty input, the boundary of a loop,
what happens on the error path.

**Scope.** Does the diff match its stated purpose, or did unrelated
changes sneak in (a reformat of unrelated lines, a drive-by refactor)?
Flag scope creep even if the extra change is itself fine — it belongs in
its own commit/PR for a clean history and easier revert if needed.

**Tests.** Is new behavior covered by a test? Does an existing test
actually exercise the changed code path, or does it just happen to still
pass? A bug fix PR with no regression test risks the same bug coming
back silently.

**Readability.** Could someone unfamiliar with this code follow it in
6 months? Names that say what something is, not just what type it is
(`userCount` not `n`). Comments explain *why*, not *what* — the code
already says what.

**Security/safety.** Any user input reaching a database query, shell
command, or HTML output unsanitized? Any secret or credential visible
in the diff? Any destructive operation (delete, overwrite, force-push)
without a safety check?

**Error handling.** Are errors from called functions actually checked,
not silently dropped? Do error messages have enough context to debug
from, without leaking internals to end users?

**What NOT to block on:** pure style preference where the codebase has
no stated convention either way, or a "nice to have" that's genuinely
out of scope — leave those as a suggestion/comment, not a blocking
request, so review doesn't become a bottleneck over nothing.
