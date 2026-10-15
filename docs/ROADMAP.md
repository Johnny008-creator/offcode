# offcode roadmap / backlog

## Current development status

v0.3.0 (2026-10-15) ships installation preferences, model downloads and
automatic updates. See [README](../README.md)
for usage and [DECISIONS](DECISIONS.md) for design details.

For the current continuation checkpoint and measured results, read
[HANDOFF](HANDOFF.md) first. The older backlog below preserves historical
rationale and is not a list of currently missing implementations.

Before the next release:

- Exercise the Windows GUI wizard and batch installer interactively; native CI
  compilation and PowerShell installation/setup/uninstall now pass.
- Mobile layouts, multi-tab behavior, live semantic ranking and the small-model
  LM Studio trial have passed their integration checks; see HANDOFF for limits.
- Validate optional video/music, transcription, document export and training
  against installed dependencies. Offline protocol tests are not live validation.

Historical implementation and test evidence is tracked in
[IMPLEMENTATION](IMPLEMENTATION.md). Hardware-specific results do not guarantee
performance on other machines. No release date is committed.

## Backlog

Ideas for future work, not commitments or a schedule. Written 2026-10-03
after the 4-tier model campaign, as a starting point for future contributions. Rough effort tags: **S** (a session or less),
**M** (multi-session feature), **L** (a project in its own right).

## Implementation update — 2026-10-09

The remaining capabilities now have implementations or integration paths:
local ComfyUI image/video/music tools, Pandoc document export, local voice I/O,
reviewed LoRA datasets/training/retraining/deployment, a five-tier/LM Studio
campaign runner, Windows CPU/memory support and CI, web artifact/diff previews
and usage charts, and repository/git tools. These are **not all live-validated**.

[IMPLEMENTATION.md](IMPLEMENTATION.md) is the current completion checklist;
[EXTENSIONS.md](EXTENSIONS.md), [TRAINING.md](TRAINING.md), and
[EVALUATION.md](EVALUATION.md) describe setup and exact limits. The older notes
below retain the original rationale; references to features being absent are
historical where superseded by these documents. Full runtime validation is
blocked in the current development sandbox by socket restrictions and missing
optional services, model weights and binaries. There is no trained personal
adapter or generated AI image/video/music artifact yet.

## User-requested follow-ups

- ~~**[M] Session persistence: `--resume` flag, multiple sessions per dir.**~~
  Done 2026-10-04. `internal/session` now stores a directory of sessions
  per cwd-hash (not one file); `-resume` opts in (TUI shows a picker,
  newest first, "start new" pinned at top; `offcode web -resume` attaches
  to the most recent). Nothing auto-resumes anymore. Mid-run `/sessions`
  reopens the same picker. `config.Config.Fresh` → `Resume` (inverted).
- ~~**[M] Web interface: visual/UX pass, more like Grok/ChatGPT.**~~ Done
  2026-10-04. Assistant replies render as real markdown (marked.js) with
  syntax-highlighted code (highlight.js), sanitized via DOMPurify before
  innerHTML insertion (web_search/fetch_url content can end up echoed
  back by the model — not trusted input). Left sidebar lists/switches/
  creates sessions, backed by three new endpoints. All three JS libs
  vendored locally via go:embed — no CDN, no build step. Also fixed a
  real pre-existing bug found along the way: the stop button sent a
  bodyless GET that /api/stop always 405'd on silently — it was a no-op
  since the feature shipped.

## Testing & validation

- **[S] Re-run the 10-task set across all 4 tiers.** Only Forge got deep
  re-verification after the prompt/mechanism fixes; Spark/Ember/Anvil's
  fixes are theory-backed but not re-confirmed live the way Forge was.
  User is running this manually with `~/Desktop/offcode_tasks.md`.
- **[S] Actually test the LM Studio backend.** Every live test this whole
  project has been Ollama — LM Studio's OpenAI-compat path has never been
  exercised against a real LM Studio instance, just read about.
- **[M] Windows.** Cross-build, native `internal/sysmon` CPU/memory
  counters (`sysmon_windows.go`), CI matrix, and a GUI/cmd/PowerShell
  installer (`scripts/windows/`, see README "Windows installer") are done
  as of 2026-10-10. Still nothing has run on a real Windows machine: the
  installers, the bash tool's `runtime.GOOS` branches, and SearXNG/Docker
  setup all need a live pass there.

## Context, memory & learning

- ~~**[M] Context compaction.**~~ Done 2026-10-04. Shipped the cheap
  mechanical version (`internal/tui/compaction.go`): summarize-oldest-
  messages once history passes 40 messages, keeping the last 16 in full.
  The tree-sitter/PageRank repo-map idea is still undone and would be a
  separate, much larger follow-up if context pressure keeps being an
  issue beyond what compaction alone fixes.
- ~~**[S] Session persistence.**~~ Done 2026-10-04. `internal/session`
  saves conversation messages per working directory (keyed by SHA256 of
  cwd) to `~/.offcode/sessions/`, restored on next launch unless `-fresh`
  is passed. Model/mode are deliberately NOT restored from the saved
  session — only messages — so they don't fight with config-file/CLI-flag
  resolution.
- ~~**[M] Config file.**~~ Done 2026-10-04. `~/.offcode/config.json` (JSON,
  not TOML — avoids an extra dependency) via `internal/config/file.go`;
  `offcode config show|set` manages it, and `runChat` merges it under
  CLI flags.
- ~~**[M] Persistent memory across sessions.**~~ Done 2026-10-04. Shipped
  the simpler flat-file design flagged as the fallback, as the primary
  path rather than the fallback: `~/.offcode/memory.md`
  (`internal/memory`), appended via both `/remember <note>` and a
  `remember(note)` meta-tool, loaded into every system prompt. Skipped
  the typed project/feedback/user entry system for now — a single
  append-only note list was judged good enough until it proves limiting
  in practice.
- **[L] Actual learning, not just memory-as-context.** A level beyond the
  above: periodically fine-tuning a small LoRA adapter per tier from
  accumulated corrections/successful sessions, so the model's own weights
  shift over time instead of every past lesson being re-injected as text
  context forever. Genuinely heavier — needs a training pipeline (data
  collection from the feedback-memory above, a LoRA training script,
  swapping the adapter into Ollama/the serving layer) that doesn't exist
  in any form yet. Worth treating as a distinct, much-later idea from
  plain memory rather than bundling them — memory-as-context is buildable
  soon, LoRA adaptation is a project of its own.

## Bigger capabilities (user's suggestions)

- ~~**[L] MCP (Model Context Protocol) client support.**~~ Done
  2026-10-04. `internal/mcpclient` (official
  github.com/modelcontextprotocol/go-sdk), configured via
  `~/.offcode/mcp.json` + project-local, never model-writable. Naming
  collision question resolved with a `mcp_<server>_` prefix (matching
  this assistant's own MCP tool naming). A real bug was caught and fixed
  before shipping — see docs/DECISIONS.md — where the server subprocess
  was tied to the wrong context and died right after connecting.
  Live-tested against the real @modelcontextprotocol/server-filesystem.
- ~~**[L] Web interface (`offcode web`).**~~ Done 2026-10-04. Answer to
  the "how much of internal/tui is reusable" question: none of it
  directly — the TUI's loop is Bubble Tea continuation-passing style,
  which doesn't map onto HTTP/SSE. Shipped as a genuine parallel engine
  instead (`internal/webengine`, a sequential per-session goroutine where
  tool-confirm/ask_user pauses are just a blocking channel receive — no
  state machine needed) behind an HTTP+SSE server (`internal/webserver`,
  static assets embedded via `go:embed`) and a no-build-step vanilla
  HTML/CSS/JS frontend. One server instance = one session, keyed to the
  cwd it's launched from and persisted via the same `internal/session`
  store as the TUI, so `offcode` and `offcode web` resume the same
  per-directory conversation. Supports streaming chat, the plan sidebar,
  a real confirm-tool-call modal, an ask_user modal, model/mode pickers,
  `/research` as a modal + endpoint, `/remember`, and a usage report —
  live-tested end to end (message → streaming reasoning → tool_call →
  confirm pause → approve → file written → idle) against qwen3:8b over
  real HTTP. Inline images/diff rendering and a charted usage dashboard
  are left as later polish, not blocking for a first working version.
- ~~**[M] Deep research mode, with a depth selector.**~~ Done 2026-10-04.
  Shipped as `/research <question> [--depth=lite|normal|deep]`
  (`internal/tui/research.go`) rather than a separate meta-tool/sub-agent —
  it crafts one depth-scoped instruction and submits it through the
  existing agent loop, so web_search/fetch_url/plan_set/write and the
  normal confirm flow all apply unmodified. Always saves a `.md` report
  to the cwd; live-tested end to end (lite depth) against qwen3:8b.
  `.pdf`/`.docx` export remain a follow-up, as originally scoped.
- ~~**[M] Skills: CSS/HTML/design quality.**~~ Done 2026-10-04, as part of
  a full 20-skill library (`internal/skills`) rather than just this one
  topic — see below.
- ~~**[M] Skills: document formats (PDF, DOCX, MD, etc.).**~~ Done
  2026-10-04. `read` now extracts real text from `.pdf`
  (github.com/ledongthuc/pdf) and `.docx` (unzip + parse
  word/document.xml, no OOXML dependency) instead of returning binary
  garbage. Writing these formats back out is explicitly NOT supported
  (the `document-formats` skill tells the model to say so rather than
  produce a broken file) — still scoped out, as originally planned.
  Covered by a real regression test (`internal/tools/docformats_test.go`
  + fixtures), not just a manual check.
- ~~**[M] Skills library, general.**~~ Done 2026-10-04 — went beyond the
  two items above per explicit request ("klidně jich udělej 20"). 20
  reference docs total: css-design, git-conventions, api-design,
  sql-schema-design, security-basics, testing-strategy, error-handling,
  performance-optimization, code-review-checklist, go-idioms,
  python-idioms, js-ts-idioms, docker-containers, shell-scripting,
  regex-patterns, accessibility, technical-writing,
  debugging-methodology, concurrency-patterns, document-formats. Only a
  one-line catalog (name + description) sits in every system prompt;
  full content loads on demand via a new `load_skill` meta-tool — chosen
  over always-on inclusion specifically to keep prompt overhead near
  zero for the ~19 topics a given task doesn't touch. Live-tested: model
  correctly called load_skill, got real file content back, and answered
  a detail question from it accurately.

## Generative media (images, video, music)

User wants these eventually, the same way a lot of today's AI products
bundle them. Researched what's actually self-hostable as of 2026 before
writing this down, since "add image/video/music generation" means very
different amounts of work depending on what runs on an 8GB-VRAM card:

- **[M] Image generation.** The standard self-hosted stack is **ComfyUI**
  (node-graph pipeline, has an HTTP API that returns base64 images —
  straightforward to call from a Go tool the same shape as `web_search`)
  running an open-weights model (FLUX.2, HunyuanImage 3.0, Qwen Image Max
  are the current top picks). ComfyUI itself is a separate service to
  stand up (same pattern as the SearXNG container) — would need its own
  `docker run`-style setup doc and a `generate_image` tool that posts a
  prompt and workflow, polls for completion, and either returns the image
  path or (if vision support lands first, see below) hands it back
  in-context. Most realistic of the three media types for this hardware —
  worth prioritizing over video/music if only one gets built.
- **[L] Video generation.** Meaningfully heavier: even the lightest
  current production-capable open model (HunyuanVideo 1.5) wants ~14GB
  VRAM with FP8 quantization and CPU offloading just to run, and the
  flagship open model (LTX-2.5) wants 32GB. That's above this machine's
  8GB card and likely above most users' hardware for a while. Worth
  designing the tool interface for eventually (same shape as image gen,
  just slower and heavier), but not worth attempting locally until either
  models get lighter or as an explicitly optional "needs a bigger GPU"
  feature — could also be the first candidate for an optional
  cloud-API fallback if offcode ever wants one (tension with the
  local-first design stance, worth a real discussion before doing it).
- **[M] Music generation.** Better news here: **ACE-Step** explicitly
  supports AMD GPUs (not just CUDA) and ships a REST API alongside its
  Gradio UI — directly compatible with this project's actual hardware,
  unlike most generative-media tooling which assumes NVIDIA. **YuE2** is
  the other strong option (~3B params, reported under 8GB VRAM in local
  testing) and has a notable design: it writes a symbolic score (ABC
  notation) before rendering audio, meaning the intermediate output is
  human/model-editable text — arguably a better fit for an agentic tool
  than a black-box audio model, since offcode could inspect/adjust the
  score before rendering. Worth prototyping before image gen, actually,
  given how well it fits this specific GPU.

## Other ideas worth considering

- ~~**[S] Diff preview in the confirm dialog.**~~ **Done 2026-10-04** —
  `internal/tui/confirm.go`.
- ~~**[S] `offcode doctor --fix`.**~~ **Done 2026-10-04** — pulls missing
  tier models automatically; doesn't yet start Ollama/SearXNG if those
  aren't reachable, just the model-pull part.
- ~~**[M] Git-aware helpers.**~~ Covered 2026-10-04 via the
  `git-conventions` skill (commit style, splitting commits, branching,
  .gitignore) rather than a dedicated tool — `bash` already gives full
  `git diff`/`log`/`commit` access, so the actual gap was knowledge, not
  a missing capability. A plan-to-PR-description generator remains a
  possible future add if it turns out the skill alone isn't enough.
- ~~**[M] Multi-agent / subagents.**~~ Done 2026-10-04.
  `spawn_subagent(task, tier?)` (`internal/subagent`) runs a bounded
  headless loop, deliberately read-only (no write/edit/bash — no human
  is attached to approve anything riskier) with an optional tier
  override so a weaker model can punt a hard sub-problem to a stronger
  one. Live-tested: qwen3:8b delegated a bug-investigation task to a
  forge-tier subagent, which correctly identified a misleading comment
  wasn't actually a bug and reported back a specific, accurate analysis.
- ~~**[S] Usage export.**~~ **Done 2026-10-04** — `offcode usage` (CLI
  report) and `offcode usage --csv`.
- ~~**[M] Vision support.**~~ Done 2026-10-04. `view_image(path)` is
  added to a model's tool list only when a runtime check says it's
  actually supported — `backend.Client.SupportsVision` queries Ollama's
  native `/api/show` `capabilities` array, so it works for any
  vision-capable model, not a hardcoded tag (confirmed gemma4:12b/Anvil
  already reports vision/audio/tools/thinking). Live-tested: generated a
  test image, asked it to view_image + describe, got an accurate
  description of the real shapes/colors back through both the TUI and
  the web API.
- ~~**[S] Hooks.**~~ Done 2026-10-04. `internal/hooks` + hand-edited
  `~/.offcode/hooks.json` / `<cwd>/.offcode/hooks.json` (merged, never
  model-writable — a human trust boundary). pre_tool_use can deny a call
  (works even in auto mode), post_tool_use runs after for side effects,
  non-blocking. Unit- and live-tested (a project hook blocking a bash
  command mentioning "secret," a post-hook logging real tool output).
- ~~**[M] Visual preview / browser automation tool.**~~ Done 2026-10-04.
  `preview_page` (vision-gated, like view_image) screenshots a local
  HTML file or URL via chromedp; `browser_text` gets JS-rendered visible
  text for pages fetch_url's static-HTML strip can't handle. A second
  real bug caught live: a full-res PNG screenshot's base64 was large
  enough to cause an actual backend tokenizer error — fixed with JPEG +
  a smaller default viewport (see docs/DECISIONS.md). Live-tested:
  gemma4:12b correctly described a hand-built page's exact colors and
  text after calling preview_page. Click/scroll interactivity stayed out
  of scope — the vision-feedback loop was the actual value here.
- **[M] Semantic codebase search — implemented 2026-10-09.**
  Available in TUI and web via `semantic_search`, using Ollama
  `nomic-embed-text` and an incremental project-local cache. Regression
  tests cover deleted files, edits during embedding, cancellation, protocol
  errors, filesystem traversal, and TUI/web tool integration. Offline
  validation: `go test -short -race ./...`, `go vet ./...`, and the Linux
  build pass. Live model validation remains pending: this development
  sandbox cannot reach Ollama or run Chromium/MCP integration tests.
  Original motivation: Grep is exact-match only; embeddings
  over the codebase (even a small local embedding model) would let the
  agent find "the function that handles X" by meaning, not just literal
  string matches — complements rather than replaces the repo-map idea
  under context compaction above, and becomes more valuable the bigger a
  project the user points offcode at.
- **[S] Voice in/out.** Local speech-to-text (whisper.cpp runs fine on
  CPU) for dictating a prompt, and text-to-speech for having responses
  read aloud — lower priority than the other media types since it doesn't
  touch the core coding-agent use case, but cheap relative to image/video/
  music if it ever becomes wanted, since whisper.cpp in particular is
  lightweight and well-trodden.

## Explicitly not re-litigating

Things already decided and working, noted here only so they don't get
re-debated from scratch next time: Go + single static binary; OpenAI-
compatible wire format for non-Ollama backends, native `/api/chat` for
Ollama specifically (needed for `think` control); SearXNG self-hosted via
Docker for web search; branded tier identity (Spark/Ember/Forge/Anvil)
hiding the underlying model family from the model's own self-description.
