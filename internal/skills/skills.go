// Package skills holds a library of loadable reference documents —
// concrete, practical knowledge (CSS patterns, git conventions, API
// design, language idioms, and so on) that's too long to keep in every
// system prompt but genuinely useful when actually relevant. Only a
// one-line catalog (name + description) is always in the prompt; the
// full content of a skill loads on demand via the load_skill meta-tool,
// the same always-on-vs-on-demand split flagged as a design question in
// docs/ROADMAP.md before this was built.
package skills

import "embed"

//go:embed content/*.md
var contentFS embed.FS

type Entry struct {
	Name        string
	Description string
	file        string
}

// catalog is deliberately a flat Go slice (not parsed from frontmatter
// in the markdown files) — descriptions are easy to scan/maintain here,
// and content files stay plain prose with no metadata syntax to get
// wrong.
var catalog = []Entry{
	{"css-design", "Modern CSS/web layout, spacing, color, and typography patterns for genuinely good-looking UI output.", "css-design.md"},
	{"git-conventions", "Commit message style, when to split commits, branching, and what belongs in .gitignore.", "git-conventions.md"},
	{"api-design", "REST resource naming, HTTP methods/status codes, pagination, versioning, idempotency.", "api-design.md"},
	{"sql-schema-design", "Normalization, keys, indexing, nullability, and safe migrations for relational schemas.", "sql-schema-design.md"},
	{"security-basics", "Injection, XSS, secrets handling, auth hashing, input validation — the recurring real-world vulnerability patterns.", "security-basics.md"},
	{"testing-strategy", "Test pyramid, what's worth testing, table-driven tests, mocking boundaries, flaky tests.", "testing-strategy.md"},
	{"error-handling", "Fail-fast vs. recover, adding context when propagating errors, retry policy, user-facing vs internal messages.", "error-handling.md"},
	{"performance-optimization", "Measure-first mindset, the N+1 query problem, algorithmic complexity, caching tradeoffs.", "performance-optimization.md"},
	{"code-review-checklist", "What to actually check reviewing a diff: correctness, scope, tests, readability, security.", "code-review-checklist.md"},
	{"go-idioms", "Go-specific patterns: error wrapping, small interfaces, receiver choice, concurrency idioms, what to avoid.", "go-idioms.md"},
	{"python-idioms", "Pythonic patterns and classic footguns: mutable defaults, context managers, comprehensions, is vs ==.", "python-idioms.md"},
	{"js-ts-idioms", "Modern JS/TS patterns: async/await pitfalls, optional chaining, avoiding any, common mutation bugs.", "js-ts-idioms.md"},
	{"docker-containers", "Multi-stage builds, layer caching order, minimal base images, non-root users, docker-compose basics.", "docker-containers.md"},
	{"shell-scripting", "Bash safety (set -euo pipefail, quoting), when to stop using bash and reach for a real language.", "shell-scripting.md"},
	{"regex-patterns", "Greedy vs lazy quantifiers, catastrophic backtracking, anchors, common regex mistakes.", "regex-patterns.md"},
	{"accessibility", "Semantic HTML, keyboard navigation, alt text, color contrast, and when ARIA is actually needed.", "accessibility.md"},
	{"technical-writing", "Commit messages, PR descriptions, README structure, and when a code comment actually earns its place.", "technical-writing.md"},
	{"debugging-methodology", "Reproduce-first, bisecting instead of guessing, forming and testing a specific hypothesis.", "debugging-methodology.md"},
	{"concurrency-patterns", "Race conditions, deadlocks, shared-state vs message-passing, bounded concurrency.", "concurrency-patterns.md"},
	{"document-formats", "What reading PDF/DOCX via the read tool actually gives you, and what it can't (tables, scans, writing them back out).", "document-formats.md"},
}

// Catalog returns every skill's name and description, for the always-on
// system prompt listing.
func Catalog() []Entry {
	return catalog
}

// Load returns a skill's full content by name, and ok=false if no such
// skill exists.
func Load(name string) (string, bool) {
	for _, e := range catalog {
		if e.Name == name {
			data, err := contentFS.ReadFile("content/" + e.file)
			if err != nil {
				return "", false
			}
			return string(data), true
		}
	}
	return "", false
}
