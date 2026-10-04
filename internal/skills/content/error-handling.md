# Error handling

**Fail fast vs. recover.** A programming error (nil pointer, index out
of range, a broken invariant) should generally crash/panic loudly in
development — swallowing it and continuing just hides the bug further
downstream. An expected failure (network timeout, file not found, bad
user input) should be handled as a normal return value/exception, not a
crash.

**Don't swallow errors silently.** `catch (e) {}` or `if err != nil { }`
with nothing in the block hides real failures. If an error genuinely
doesn't matter (e.g. best-effort telemetry), say so with a comment
explaining why it's safe to ignore, don't just drop it unexplained.

**Add context when propagating.** A bare "file not found" bubbling up
from deep in a call stack is useless; wrap it with what was being
attempted ("loading config from /etc/app/config.yaml: file not found").
Go: `fmt.Errorf("doing X: %w", err)`. Most languages have an equivalent
wrap-with-context pattern — use it every time an error crosses a
meaningful boundary (e.g. into/out of a package).

**Retry only idempotent, transient failures.** Network blips and
5xx/timeout responses are reasonable to retry (with backoff). A 400
(bad request) or a logic error will fail identically every time — retry
only multiplies load for no benefit. Cap retry attempts; unbounded retry
loops are a real production incident pattern.

**User-facing vs. internal error messages.** Don't leak stack traces,
file paths, or internal implementation details to end users — log the
full detail server-side, show the user a message they can act on ("that
email is already registered," not "UNIQUE constraint failed:
users.email").

**Errors as values vs. exceptions.** Whichever the language idiom is
(Go returns errors, Python/JS/Java throw), be consistent within a
codebase — don't invent a third parallel error-signaling mechanism
(sentinel return values alongside exceptions) in a language that already
has one idiom.
