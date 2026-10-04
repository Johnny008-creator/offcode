# Performance optimization

**Measure before optimizing.** Guessing where the bottleneck is and
"optimizing" that spot is usually wasted effort — profile first (a real
profiler, or at minimum timing instrumentation around suspected hot
paths). The actual bottleneck is often somewhere surprising.

**The N+1 query problem** is the single most common real-world
performance bug: a loop that issues one database query per iteration
instead of one batched query for all iterations. If you see a query
inside a `for` loop over rows that came from another query, that's
almost always fixable with a JOIN or a single `WHERE id IN (...)`.

**Algorithmic complexity matters more than micro-optimization** for
anything that scales with input size. A O(n²) algorithm that's "fast
enough" at n=100 will not be fine at n=100,000 — know the complexity of
the data structures/algorithms you reach for (a linear search through a
list vs. a hash-map lookup, repeated string concatenation in a loop
building a large string vs. a builder/buffer).

**Caching** trades memory/staleness for speed — only add it once you've
confirmed the thing being cached is actually expensive and actually
re-computed often. Caching adds invalidation complexity; a cache with
no invalidation strategy is a bug waiting to happen (stale data served
forever).

**Premature optimization is real but so is premature pessimization** —
don't contort code for a performance gain nobody asked for and no
profiler justified, but also don't pick an obviously-quadratic approach
"for now" when the linear one is just as easy to write.

**Know what's cheap and what's expensive** in the environment you're
in: a network call or disk I/O is orders of magnitude slower than an
in-memory operation — batch/parallelize I/O-bound work, don't bother
parallelizing something that's already CPU-bound and fast.
