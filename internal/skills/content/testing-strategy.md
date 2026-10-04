# Testing strategy

**The pyramid.** Many fast unit tests (single function/module, no I/O),
fewer integration tests (real database/filesystem, still one process),
very few end-to-end tests (the whole system, slow, brittle, but catches
what unit tests structurally can't). If a test suite is mostly e2e,
it'll be slow and flaky; if it's 100% unit with mocked everything, it
can pass while the real integration is broken.

**What to actually test.** Business logic and edge cases, not getters/
setters or framework boilerplate. Edge cases worth deliberately writing
a test for: empty input, the boundary values of a range (0, -1, max
int), concurrent access if the code touches shared state, and the exact
bug you just fixed (a regression test — otherwise it comes back).

**Table-driven tests** (common in Go, worth doing in any language) —
one test function, a slice of {input, expected} cases, loop and assert.
Much less duplication than one test function per case, and new cases are
one line to add.

**Mocking.** Mock at a boundary you own (an interface your code defines),
not a third-party library's internals directly — otherwise the mock
breaks every time the library's internals change even if its behavior
didn't. Don't mock what you're actually trying to test; a test that
mocks the database and the business logic and the validation isn't
testing anything real.

**Don't mock what you can run for real cheaply.** A real SQLite/in-memory
database in a test is often both more honest and less code than mocking
a database layer — this project's own feedback history has exactly this
lesson: a mocked-database test passed while a real migration was broken.

**Test names should state the scenario and expectation**, e.g.
`TestSave_EmptyMessages_IsNoop`, not `TestSave2`.

**Flaky tests are worse than no test** — a test that sometimes fails for
reasons unrelated to the code (timing, shared global state, test
ordering) trains people to ignore CI failures. Fix or delete it, don't
let it linger.
