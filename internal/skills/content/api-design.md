# REST API design

**Resource naming.** Plural nouns for collections (`/users`, not
`/user`), nested resources for ownership (`/users/42/orders`), no verbs
in the path (`/users/42/activate` is borderline-acceptable for an action
with no good resource model; `POST /activateUser` is not REST).

**HTTP methods.** GET (read, no side effects, safe to cache/retry), POST
(create, or a non-idempotent action), PUT (replace a whole resource,
idempotent), PATCH (partial update), DELETE (remove, idempotent — calling
it twice on an already-deleted resource should still return 2xx or 404,
not error).

**Status codes that actually matter:** 200 (ok), 201 (created, include a
Location header), 204 (success, no body), 400 (client sent bad input),
401 (not authenticated), 403 (authenticated but not allowed), 404 (not
found), 409 (conflict, e.g. duplicate), 422 (semantically invalid
input), 429 (rate limited), 500 (server bug). Don't return 200 with an
error message in the body — that breaks every HTTP-level tool.

**Pagination.** Cursor-based (`?cursor=abc&limit=50`) scales better than
offset-based (`?page=3`) for anything that can grow large or change
while paginating. Always include a way to know if there's a next page.

**Versioning.** Prefix the path (`/v1/...`) or a header
(`Accept: application/vnd.api+json;version=1`) — path prefix is simpler
and more visible, pick one and be consistent.

**Idempotency.** For POST endpoints that create something expensive
(payments, orders), accept an `Idempotency-Key` header so a retried
request doesn't double-create.

**Error responses.** A consistent shape across the whole API — at
minimum `{"error": {"code": "...", "message": "..."}}` — not a different
error format per endpoint.
