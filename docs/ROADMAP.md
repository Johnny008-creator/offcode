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
- **[M] Persistent memory across sessions.** Right now every offcode
  session starts from zero — no memory of the user's preferences, past
  corrections, or project-specific conventions, even for the exact same
  project the day before. Worth building something modeled directly on
  the memory system this very assistant uses day to day, since it's a
  proven design: a handful of typed entries —
  - **project**: facts about a specific codebase that aren't derivable by
    reading the code (why a weird workaround exists, what's mid-refactor)
  - **feedback**: corrections or confirmed approaches ("don't use X here,
    user said Y broke last time" / "user confirmed Z approach works") —
    the single highest-value type, since it directly prevents repeating a
    mistake the user already corrected once
  - **user**: standing preferences (verbosity, tone, how they like changes
    staged/committed)

  Mechanically: a small `~/.offcode/memory/` directory (global) plus
  maybe a per-project file, loaded into the system prompt (or a
  `recall_memory` tool a model calls on demand — same always-on-vs-on-
  demand tradeoff as the CSS/design skill idea above) at session start.
  The harder design question isn't storage, it's **when an entry gets
  written** — this assistant relies on a person (Claude) noticing "the
  user just corrected me" or "the user just confirmed something
  non-obvious" and deciding to save it; a local model, especially the
  smaller tiers, may need a more mechanical trigger (an explicit
  `/remember <note>` command the user invokes themselves is the safe
  fallback if automatic noticing proves unreliable for weaker tiers).
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
- **[L] Web interface (`/web` or similar).** A browser UI alongside the
  TUI, reusing the same agent/backend logic behind a small local HTTP
  server. Opens the door to viewing from another device on the LAN,
  richer rendering (inline images, syntax-highlighted diffs) than a
  terminal allows, and is a natural home for things that don't fit a TUI
  well (a real usage dashboard with actual charts instead of ASCII bars).
  Biggest open question: how much of internal/tui's logic is reusable vs.
  needs a parallel implementation — worth a design pass before coding.
- **[M] Deep research mode, with a depth selector.** Current
  `web_search`/`fetch_url` are single-shot. A proper research mode loops —
  search, read several results, decide what's still missing, search
  again, synthesize a written answer with sources — closer to what "deep
  research" means in other products. Could be its own meta-tool
  (`research(question, depth)`) that internally drives several
  web_search/fetch_url rounds before returning a synthesized result,
  keeping the main agent loop simple. User specifically wants three depth
  levels — rough shape:
  - **lite**: one search, read the top result or two, short summary (close
    to what `web_search` + a single `fetch_url` already does today).
  - **normal**: several searches from different angles, read the best
    handful of pages, a proper synthesized write-up with sources.
  - **deep**: iterative — synthesize interim findings, notice gaps,
    search again to fill them, repeat for a bounded number of rounds,
    then a long-form final report.
  Should be able to **save the result as a document** — plain `.md` is
  free (just `write`), `.pdf` needs a real generation step (a Go PDF
  library, or render Markdown→HTML→PDF via a headless renderer), `.docx`
  is a zip of XML so tractable without a huge dependency but still real
  work. Scope `.md` first, treat `.pdf`/`.docx` as a follow-up once the
  research loop itself is solid.
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
