# Python idioms

**Mutable default arguments are a classic footgun** —
`def f(items=[]):` reuses the SAME list object across every call with
no argument given, so mutations persist across calls. Use
`def f(items=None): items = items if items is not None else []`
instead.

**Context managers for anything with a cleanup step** — files, locks,
database connections, network sockets. `with open(path) as f:` not
manual `open()`/`close()` pairs, which leak the resource on an exception
between them.

**List/dict/set comprehensions** over manual loops that just build a new
collection — `[x*2 for x in items if x > 0]` is both more idiomatic and
usually faster than the equivalent `for` loop with `.append()`. Don't
nest comprehensions deep enough to hurt readability, though — a regular
loop is better once a comprehension needs more than one condition/
transform.

**`is` vs `==`.** `is` checks identity (same object), `==` checks
equality (same value). Only use `is` for `None`/`True`/`False` and
sentinel objects — comparing values with `is` (especially small
integers/strings, which CPython sometimes caches) is a bug waiting to
surface differently across Python versions.

**f-strings** for formatting (`f"{name}: {count}"`) over `%`-formatting
or `.format()` in new code — clearer and faster.

**Type hints** on function signatures, even in a dynamically-typed
codebase — `def process(items: list[str]) -> dict[str, int]:` — they
document intent and let tools (mypy, IDEs) catch real mistakes before
runtime.

**Virtual environments, always** — never `pip install` into the system
Python. One venv per project, dependencies pinned in
`requirements.txt`/`pyproject.toml`.

**Avoid:** bare `except:` (catches `SystemExit`/`KeyboardInterrupt` too
— use `except Exception:` at minimum), `from module import *` (pollutes
the namespace, hides where names come from), building strings with `+`
in a loop (use `"".join(parts)` instead — `+` in a loop is quadratic).
