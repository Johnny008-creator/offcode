# Git conventions

**Commit messages.** Subject line under ~70 chars, imperative mood
("Add X", "Fix Y", not "Added"/"Fixed"). Body explains *why*, not what —
the diff already shows what changed. Blank line between subject and body.

**When to split commits.** One logical change per commit. A bug fix and
an unrelated refactor belong in separate commits even if touched in the
same session — `git add -p` to split a dirty working tree if needed.
Don't split so finely that individual commits don't build/pass tests on
their own, though.

**Before committing:** run `git status` and `git diff --staged` to see
exactly what's going in — never `git add -A` blindly if there's any
chance of picking up secrets, build artifacts, or unrelated edits.

**Branching.** Short-lived feature branches off main/master. Rebase a
feature branch onto main before merging if the team prefers linear
history; merge commits are fine if they don't.

**Never rewrite published history** (`push --force`, `rebase` on a
shared branch, `commit --amend` after pushing) unless explicitly told to
— it breaks everyone else's local copy.

**.gitignore basics:** build output, dependency directories
(node_modules, vendor if not vendoring intentionally), IDE config
(.idea, .vscode unless the team shares settings), OS files (.DS_Store),
and anything with secrets (.env, credentials.json) — the last one should
never even be committed once, since removing it from history later is
its own project.

**Tags** for releases, annotated (`git tag -a v1.2.0 -m "..."`) not
lightweight, so there's a message and date attached.
