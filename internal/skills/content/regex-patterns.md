# Regex patterns and pitfalls

**Greedy vs. lazy quantifiers.** `.*` is greedy — it grabs as much as
possible, then backtracks. For `<.*>` against `<a><b>`, greedy matches
the whole `<a><b>` (both tags), not just `<a>`. Use the lazy form `.*?`
when you want the shortest match: `<.*?>` correctly matches just `<a>`.

**Catastrophic backtracking.** A pattern like `(a+)+$` against a long
string of `a`s with no trailing match can take exponential time — nested
quantifiers over overlapping character sets are the usual culprit.
Rewrite to avoid the ambiguity (`a+$` has the same intent without the
nested quantifier) rather than hoping the input is always short.

**Anchors matter.** `^` and `$` anchor to the start/end of the *string*
by default in most engines, or of each *line* in multiline mode — know
which your engine/flag combination gives you, since an unanchored
pattern matches anywhere in the string, which is often not what's
actually wanted for validation (`^\d+$` for "is this entirely digits",
not just `\d+` which matches if digits appear anywhere).

**Character classes.** `[^x]` means "any character except x" — a common
mistake is writing `[^xyz]` meaning "not x, not y, not z" when a
pattern-match for a literal `^` or multi-character exclusion was
intended elsewhere. Escape special characters (`. * + ? ( ) [ ] { } |
\ ^ $`) when you mean them literally — `\.` for a literal dot, not `.`
which means "any character."

**Don't parse HTML/XML with regex** — nesting and edge cases (attributes
containing `>`, CDATA sections) make it unreliable past trivial cases.
Use a real parser.

**When a regex is unreadable, prefer named groups and comments** (most
engines support an extended/verbose mode, e.g. Python's `re.VERBOSE`) or
just split the logic into a few simpler regexes/string operations
instead of one dense expression nobody can maintain.

**Test against the actual edge cases**, not just the happy path: empty
string, the longest realistic input, input with the exact special
characters the pattern is supposed to handle.
