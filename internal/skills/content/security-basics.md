# Security basics

**Injection (SQL, shell, command).** Never build a query/command by
string-concatenating user input. Use parameterized queries
(`db.Query("... WHERE id = ?", id)`, never `fmt.Sprintf` into SQL). For
shell commands, avoid passing user input to a shell interpreter at all
if possible (`exec.Command("prog", arg)` not `sh -c "prog " + arg`); if
you must build a shell string, every piece of untrusted input needs
proper quoting/escaping for that specific shell.

**XSS.** Never insert untrusted content into HTML via `innerHTML` /
unescaped template output without sanitizing it first (a library like
DOMPurify client-side, or auto-escaping templates server-side). This
applies to content that round-trips through an LLM too — model output
that quotes web content it fetched is still untrusted when rendered.

**Secrets.** Never hardcode API keys/passwords/tokens in source — load
from environment variables or a secrets manager. Never log them, even
at debug level. Never commit a `.env` file with real values (commit
`.env.example` with placeholder keys instead).

**Auth.** Hash passwords with bcrypt/argon2/scrypt, never plain SHA-256
(too fast, too brute-forceable) and never roll your own. Compare secrets
(tokens, HMACs) with a constant-time comparison, not `==`, to avoid
timing attacks.

**Input validation.** Validate at the boundary (API handler, form
submit) — trust nothing from the network. Validate type, length, and
range, not just "is it present."

**Dependencies.** Pin versions; don't blindly pull `latest` in anything
that ships. Be wary of adding a dependency for something trivial enough
to write in 10 lines — more dependencies is more supply-chain surface.

**The real OWASP Top 10 pattern to internalize:** most real-world
vulnerabilities are not exotic — they're "we trusted input we shouldn't
have" in one of the forms above. When writing code that touches
user-supplied data, ask explicitly: where does this data go, and does
that destination treat it as code (SQL, shell, HTML, a template) or
data?
