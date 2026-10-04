# Shell scripting

**Start every bash script with:**
```
#!/usr/bin/env bash
set -euo pipefail
```
`-e` exits on any command failure instead of silently continuing, `-u`
errors on an undefined variable instead of treating it as empty, `-o
pipefail` makes a pipeline fail if ANY stage fails, not just the last
one (`false | true` would otherwise report success).

**Quote every variable expansion:** `"$var"` not `$var` — unquoted
expansion undergoes word-splitting and glob expansion, which breaks on
filenames with spaces and is a real source of bugs and (if the variable
is ever attacker-influenced) a command-injection vector. `"$@"` not
`$@` when forwarding arguments.

**Prefer `[[ ]]` over `[ ]`** in bash specifically — fewer quoting
footguns, supports `&&`/`||`/pattern matching directly inside the test.

**Check command existence before using it:**
`command -v tool >/dev/null || { echo "tool not found"; exit 1; }`
rather than letting a cryptic "command not found" surface later.

**Use `mktemp` for temp files/dirs**, never a hardcoded path like
`/tmp/myfile` — a predictable temp path is both a race condition and a
symlink-attack vector in multi-user systems.

**When a script grows past ~50-100 lines of real logic (not just
sequential commands), or needs real data structures/error handling,
switch to a real scripting language** (Python, Go) instead of fighting
bash's limitations — this is a judgment call, not a hard rule, but bash
control flow and string handling get painful fast past simple
sequential/looping logic.

**Portability.** `#!/usr/bin/env bash` not `#!/bin/bash` (bash isn't
always at that exact path, e.g. on some BSD/macOS setups). If the script
needs to run on both Linux and macOS, watch for GNU vs BSD flag
differences in common tools (`sed -i` needs a different form on each).
