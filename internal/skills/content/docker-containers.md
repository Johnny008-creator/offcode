# Docker / containers

**Multi-stage builds** for compiled languages — a `builder` stage with
the full toolchain compiles the binary, a final minimal stage (`alpine`,
`scratch`, or a `distroless` base) copies in just the binary. Keeps the
shipped image small and avoids shipping build tools/source in
production.

**Layer caching.** Order Dockerfile instructions from least to most
frequently changing: install OS packages and dependencies before
copying application source, so a source change doesn't invalidate the
(slow) dependency-install layer. Copy a dependency manifest
(`package.json`, `go.mod`, `requirements.txt`) and install dependencies
*before* copying the rest of the source for the same reason.

**Minimal base images.** `alpine` or `distroless` over a full `ubuntu`/
`debian` base when the app doesn't need a general-purpose OS — smaller
attack surface, smaller image, faster pulls. Watch for `alpine`'s musl
libc causing subtle behavior differences from glibc in some compiled
binaries if that matters for the stack in use.

**Don't run as root inside the container** unless there's a specific
reason — add a non-root `USER` instruction.

**One process per container** as the default model — a web server and
its cron jobs are two containers (or a scheduler outside the container),
not a container running a supervisor that starts both.

**`.dockerignore`** matching `.gitignore`'s intent — don't send
`node_modules`, `.git`, or build artifacts into the build context,
which slows builds and can leak things into layers unintentionally.

**docker-compose** for local multi-service development (a database +
the app + a cache, say) — pin image versions (`postgres:16`, not
`postgres:latest`) so local dev matches what's actually deployed.

**Health checks** (`HEALTHCHECK` in the Dockerfile, or the orchestrator's
equivalent) so a container that's running but not actually serving
traffic gets detected and restarted, not left silently broken.
