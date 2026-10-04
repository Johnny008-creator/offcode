# Go idioms

**Errors.** Return `error` as the last value, check it immediately
(`if err != nil { return fmt.Errorf("...: %w", err) }`), don't ignore it
with `_`. Use `errors.Is`/`errors.As` to check for specific error types/
sentinels, not string-matching the error message. Define sentinel errors
with `errors.New` or a custom type when callers need to distinguish
error cases programmatically.

**Interfaces.** Accept interfaces, return concrete types. Keep
interfaces small — one or two methods is typical idiomatic Go (`io.Reader`
is one method). Don't define an interface until you actually have two
implementations or a real testing need for one; a single-implementation
interface "for future flexibility" is usually unnecessary indirection.

**Receivers.** Pointer receiver if the method mutates the receiver, or
the struct is large, or other methods on the type already use pointer
receivers (consistency within a type matters more than the exact rule).
Value receiver for small, immutable-in-practice types.

**Concurrency.** Channels for communicating/synchronizing between
goroutines, mutexes for protecting shared mutable state accessed from
multiple goroutines without a clear ownership handoff. "Don't communicate
by sharing memory; share memory by communicating" — but a simple mutex
around a map is often clearer than a goroutine-per-access-with-channels
design for straightforward cases. Always know which goroutine owns a
piece of state at any given time.

**Zero values should be useful** where practical — a `sync.Mutex` or
`bytes.Buffer` works with no initialization; design your own types the
same way when it's easy to.

**Avoid:** naked returns in long functions (hurts readability even
though legal), `interface{}`/`any` as a lazy substitute for a proper
type when the concrete type is actually known, panicking for ordinary
error conditions (panic is for truly unrecoverable programmer errors,
not expected failure paths), ignoring `context.Context` cancellation in
anything that does I/O or could run long.

**gofmt/go vet are not optional** — always-on, zero-debate formatting;
run `go vet` before considering something done.
