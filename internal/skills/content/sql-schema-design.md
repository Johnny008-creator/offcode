# SQL schema design

**Normalization.** Default to 3NF (no repeated groups, every non-key
column depends only on the key) — denormalize deliberately for a
specific read-performance reason, not by default. Duplicated data across
rows is a sign something should be a foreign key instead.

**Keys.** Every table gets a primary key — a surrogate auto-increment or
UUID is usually simpler than a natural composite key, even when a
natural key exists, because natural keys sometimes change (emails get
updated, names get corrected). Foreign keys should be indexed —
most databases don't do this automatically.

**Indexing.** Index columns used in WHERE, JOIN, and ORDER BY clauses.
A composite index's column order matters — put the most selective /
most commonly-equality-filtered column first. Don't over-index: every
index slows down writes and takes space, so index for actual query
patterns, not speculatively.

**Nullability.** Make a column NOT NULL unless the absence of a value is
a real, meaningful state — "unknown" and "not applicable" are different
things from "defaults to empty string." A nullable foreign key usually
means "this relationship is optional," which is fine, just be deliberate
about it.

**Migrations.** Every schema change goes through a migration file, never
a manual `ALTER TABLE` run by hand against production. Additive changes
(new nullable column, new table) are safe to deploy before the app code
that uses them; removing a column or making one NOT NULL needs a
backfill step first and should be its own migration, done after the
application stops referencing the old shape.

**Timestamps.** `created_at`/`updated_at` on basically everything, in
UTC, set by the database (`DEFAULT now()`) rather than the application
layer, so they're consistent regardless of which service writes the row.
