# Concurrency patterns

**Race conditions** happen when two threads/goroutines/processes access
shared mutable state without coordination, and the outcome depends on
timing. The fix is always some form of coordination: a mutex/lock around
the shared state, an atomic operation for simple counters, or redesigning
so the state isn't actually shared (message passing / channels instead).
"It usually works" is the signature symptom of a race — timing-dependent
bugs don't reproduce reliably, which is exactly what makes them
dangerous.

**Deadlocks** happen when two or more threads each wait for a resource
the other holds, with no way forward. Common cause: acquiring multiple
locks in inconsistent order across different code paths (thread A locks
X then Y, thread B locks Y then X) — always acquire locks in the same
global order everywhere, or use a single coarser lock if the finer-
grained locking isn't earning its complexity.

**Shared state vs. message passing.** A mutex protecting a shared data
structure is simple and fine for straightforward "multiple readers/
writers touch this map" cases. Channels (Go) / queues / actor models fit
better when the real shape of the problem is "work flows from one stage
to another" — forcing that into shared-state-plus-locks tends to produce
more complex code than modeling the actual data flow directly.

**Know what "thread-safe" actually promises.** A thread-safe data
structure guarantees individual operations are atomic — it does NOT
mean a sequence of operations on it is atomic. `if !map.Has(k) {
map.Set(k, v) }` on a "thread-safe" map still races between the check
and the set unless the structure offers a single atomic
check-and-set/compare-and-swap operation for that exact purpose.

**Goroutine/thread lifecycle.** Every goroutine/thread you start should
have a clear way to know when to stop (a context cancellation, a done
channel, a flag checked in its loop) — a goroutine with no exit
condition that outlives its purpose is a leak. Always consider what
happens to in-flight work if the program needs to shut down mid-
operation.

**Bounded concurrency.** Unbounded `go func()` per incoming item (or
unbounded thread creation) under load can exhaust memory/OS resources —
use a worker pool or a semaphore (buffered channel, or a real semaphore
primitive) to cap how much runs concurrently.
