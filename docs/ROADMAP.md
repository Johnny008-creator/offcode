# offcode roadmap / backlog

Ideas for future work, not commitments or a schedule. Written 2026-10-03
after the 4-tier model campaign, so the team has a starting point instead
of a blank page next session. Rough effort tags: **S** (a session or less),
**M** (multi-session feature), **L** (a project in its own right).

## Testing & validation

- **[S] Re-run the 10-task set across all 4 tiers.** Only Forge got deep
  re-verification after the prompt/mechanism fixes; Spark/Ember/Anvil's
  fixes are theory-backed but not re-confirmed live the way Forge was.
  User is running this manually with `~/Desktop/offcode_tasks.md`.
- **[S] Actually test the LM Studio backend.** Every live test this whole
  project has been Ollama — LM Studio's OpenAI-compat path has never been
  exercised against a real LM Studio instance, just read about.
- **[M] Windows.** Mentioned as a target from the start, never touched.
  `internal/sysmon` is Linux-only by construction (procfs/sysfs); the
  bash tool already branches on `runtime.GOOS` but is untested; path
  handling, the monitor, and SearXNG/Docker setup all need a real pass.

## Context & memory

- **[M] Context compaction.** Called out as deferred since the very first
  DECISIONS.md entry and still not built. Matters more for local models
  than Claude (8-32K effective context vs 200K) — needs a real plan:
  summarize-oldest-N-turns is the cheap version, Aider's tree-sitter +
  PageRank repo map (also flagged early, also still undone) is the
  higher-effort/higher-payoff version for keeping large codebases
  navigable in a small context window.
- **[S] Session persistence.** Save/resume a conversation (messages,
  plan state, model) across restarts — right now closing offcode loses
  everything. Natural pairing with compaction since a resumed long
  session needs it anyway.
- **[M] Config file.** `~/.offcode/config.toml` (or similar) for defaults
  (model, mode, backend) instead of CLI flags every time — flagged as
  deferred in the original v0.1 scope.

## Bigger capabilities (user's suggestions)

- **[L] MCP (Model Context Protocol) client support.** Let offcode
  connect to external MCP servers as additional tool providers —
  filesystem, databases, whatever the MCP ecosystem has — instead of
  only the hand-built tool set. Biggest capability multiplier on this
  list without reinventing every integration ourselves. Needs a spec
  read-through and a design decision on how MCP tools surface alongside
  the built-ins (naming collisions, per-tier tool-list filtering like
  Spark's ask_user removal).
- **[L] Web interface (`/web` or similar).** A browser UI alongside the
  TUI, reusing the same agent/backend logic behind a small local HTTP
  server. Opens the door to viewing from another device on the LAN,
  richer rendering (inline images, syntax-highlighted diffs) than a
  terminal allows, and is a natural home for things that don't fit a TUI
  well (a real usage dashboard with actual charts instead of ASCII bars).
  Biggest open question: how much of internal/tui's logic is reusable vs.
  needs a parallel implementation — worth a design pass before coding.
- **[M] Deep research mode.** Current `web_search`/`fetch_url` are
  single-shot. A proper research mode would loop — search, read several
  results, decide what's still missing, search again, synthesize a
  written answer with sources — closer to what "deep research" means in
  other products. Could be its own meta-tool (`research(question)`) that
  internally drives several web_search/fetch_url rounds before returning
  a synthesized result, keeping the main agent loop simple.
- **[M] Skills: CSS/HTML/design quality.** The system prompt already
  pushes "good modern styling," but a loadable skill (reference palettes,
  layout patterns, a checklist) could push quality further than prompt
  text alone, especially for Spark/Ember where prompt space and model
  capacity are both tight. Needs a decision on the loading mechanism —
  always-on addition to the prompt vs. a tool the model calls on demand
  (`load_skill("css-design")`), which also affects token budget.
- **[M] Skills: document formats (PDF, DOCX, MD, etc.).** Today `read`
  and `fetch_url` only handle plain text/HTML. Reading a PDF or DOCX
  needs a real extraction library (e.g. a Go PDF text extractor, DOCX is
  a zip of XML so more tractable without a huge dependency). Writing
  these formats is a further step up in complexity — scope the read side
  first, writing only if it turns out to matter.

## Other ideas worth considering

- **[S] Diff preview in the confirm dialog.** Right now confirming a
  `write`/`edit` shows raw tool-call JSON (truncated to 80 chars) — showing
  an actual diff (new file vs. old, or the old_string/new_string change)
  would make the auto-mode-off confirmation step genuinely useful instead
  of just a speed bump.
- **[S] `offcode doctor --fix`.** Doctor currently only diagnoses. Since
  it already knows exactly what's missing (model not pulled, Ollama not
  running), it could offer to run the fix commands itself rather than
  just printing them.
- **[M] Git-aware helpers.** A tool or system-prompt policy for
  committing after a successful task, or drafting a PR description from
  the session's plan — the plan-tracking data already captures "what was
  done," which maps naturally onto a commit message.
- **[M] Multi-agent / subagents.** Flagged as deliberately out of v0.1
  scope from the start. Worth reconsidering once context compaction
  exists — a sub-agent for a side investigation (research, test-writing)
  that reports back a summary is the same pattern Claude Code's own
  Agent tool uses, and local models doing it to each other could help
  the weaker tiers punt hard sub-problems to a stronger one.
- **[S] Usage export.** `/usage` and the monitor show a live report;
  exporting `~/.offcode/usage.log` to CSV (or just documenting that it's
  plain JSONL and already greppable) would help anyone wanting to graph
  it elsewhere.
- **[M] Vision support.** gemma4 has vision-capable variants. If a loaded
  model supports images, a `view_image`-style tool (screenshot, local
  image file) could let the agent actually look at UI output it built
  rather than only reasoning about code — directly relevant to the
  output-quality problem this whole project keeps circling back to.
- **[S] Hooks.** User-definable pre/post tool-call hooks (shell commands
  run around specific tool invocations) — mentioned as out of scope early
  on, still plausible as a small, self-contained feature whenever there's
  a concrete use case for it.

## Explicitly not re-litigating

Things already decided and working, noted here only so they don't get
re-debated from scratch next time: Go + single static binary; OpenAI-
compatible wire format for non-Ollama backends, native `/api/chat` for
Ollama specifically (needed for `think` control); SearXNG self-hosted via
Docker for web search; branded tier identity (Spark/Ember/Forge/Anvil)
hiding the underlying model family from the model's own self-description.
