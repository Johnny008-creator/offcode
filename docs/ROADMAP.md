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

- **[L] MCP (Model Context Protocol) client support.** Let offcode
  connect to external MCP servers as additional tool providers —
  filesystem, databases, whatever the MCP ecosystem has — instead of
  only the hand-built tool set. Biggest capability multiplier on this
  list without reinventing every integration ourselves. Needs a spec
  read-through and a design decision on how MCP tools surface alongside
  the built-ins (naming collisions, per-tier tool-list filtering like
  Spark's ask_user removal).
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
- ~~**[S] Usage export.**~~ **Done 2026-10-04** — `offcode usage` (CLI
  report) and `offcode usage --csv`.
- **[M] Vision support.** gemma4 has vision-capable variants. If a loaded
  model supports images, a `view_image`-style tool (screenshot, local
  image file) could let the agent actually look at UI output it built
  rather than only reasoning about code — directly relevant to the
  output-quality problem this whole project keeps circling back to.
- **[S] Hooks.** User-definable pre/post tool-call hooks (shell commands
  run around specific tool invocations) — mentioned as out of scope early
  on, still plausible as a small, self-contained feature whenever there's
  a concrete use case for it.
- **[M] Visual preview / browser automation tool.** Directly relevant to
  the output-quality problem this project keeps finding (models write
  HTML/CSS blind, never see the result). A `preview_page` tool that
  screenshots a local HTML file in a headless browser — or further, a
  small set of browser-automation primitives (click, scroll, read visible
  text) — would let the agent actually check its own UI work instead of
  only reasoning about markup. Pairs naturally with vision support
  (already on the list above): screenshot the page, feed it back to a
  vision-capable tier, let it judge/fix its own design.
- **[M] Semantic codebase search.** Grep is exact-match only; embeddings
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
