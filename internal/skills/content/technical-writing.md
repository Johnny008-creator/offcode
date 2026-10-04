# Technical writing

**Commit messages:** imperative subject line ("Add retry logic", not
"Added" or "Adding"), body explains why the change was made and any
non-obvious tradeoff, not a restatement of the diff.

**PR/change descriptions:** lead with what changed and why a reviewer
should care, not a chronological narration of your debugging process.
A short "what changed" + "why" + "how to verify" structure covers almost
every case.

**README structure that actually gets read:** what the project is (one
sentence), how to run it (the actual commands, copy-pasteable), then
everything else. Don't bury the run instructions under three paragraphs
of philosophy — people skim for the command block first.

**Code comments explain WHY, not WHAT.** `// increment counter` above
`counter++` is noise — the code already says that. A comment earns its
place when it explains a non-obvious constraint, a workaround for a
specific bug/library quirk, or why an apparently-simpler alternative
doesn't work. If you'd need to delete a comment because the code changed
underneath it, that's often a sign the comment was restating the code
rather than adding information.

**Docs rot — write less of it, make it count.** A long doc nobody
updates becomes actively misleading. Prefer: self-documenting code and
names over a separate doc restating structure; a short doc for things
that genuinely can't be inferred from reading the code (why a design
decision was made, an external constraint, a non-obvious setup step).

**Audience-match the detail level.** A doc for end users needs zero
implementation detail and a lot of "how do I accomplish X." A doc for
future maintainers needs the reverse — the reasoning behind decisions,
not a tutorial on how to use the thing.

**Avoid weasel words and hedging** ("might possibly in some cases") in
technical docs — state what's actually true, and if something is
genuinely uncertain, say specifically what's unknown rather than
hedging vaguely.
