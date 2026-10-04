# JavaScript / TypeScript idioms

**`===` not `==`** — `==` does type coercion with surprising rules
(`[] == false` is `true`). Always use strict equality unless you have a
specific, commented reason not to.

**`const` by default, `let` when reassignment is actually needed, never
`var`** — `var` is function-scoped (not block-scoped) and hoisted,
source of classic bugs in loops with closures.

**Async/await over raw `.then()` chains** for anything beyond a single
step — easier to read, and `try/catch` works naturally around it. Don't
forget `await` on a promise you actually need to wait for — a dropped
`await` is a silent bug (the promise still runs, but the calling code
moves on without its result, and an unhandled rejection may surface far
from where it happened).

**`Promise.all`** for independent concurrent async work, not a
sequential `for` loop with `await` inside it when the operations don't
depend on each other — the loop version runs them one at a time for no
reason.

**Optional chaining and nullish coalescing** (`obj?.field?.method?.()`,
`value ?? defaultValue`) over manual `if (obj && obj.field)` chains —
more concise and `??` (unlike `||`) correctly treats `0`/`""`/`false` as
valid values, not falsy-therefore-replace.

**TypeScript: avoid `any`** — it opts a value out of type checking
entirely. `unknown` plus a narrowing check (`typeof`, a type guard) is
almost always the correct replacement when the type genuinely isn't
known yet. Prefer `interface`/`type` definitions that match the real
shape of data over casting with `as` to force a mismatch through.

**Avoid:** mutating function parameters/props (React and many frameworks
assume immutability — mutating in place skips re-renders and causes
subtle bugs), deeply nested callback pyramids (restructure with
async/await or named intermediate functions), comparing objects/arrays
with `===` expecting value equality (it's reference equality — use a
deep-equal check or compare the specific fields that matter).
