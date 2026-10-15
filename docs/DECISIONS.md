# offcode — design decisions

Research synthesis from 2026-09-29, before writing any code. Kept as the
reference point for why v0.1 looks the way it does.

## Language: Go

Closest comparable project, OpenCode (~200k stars, provider-agnostic CLI with
first-class Ollama support), is Go + Bubble Tea — evidence the pairing scales
to a real agentic coding tool. Single static binary for Linux and Windows,
no runtime dependency for end users, fast edit-compile-test loop. Rust was
the strongest counter-argument (Goose uses it) but its slower iteration cycle
isn't worth it while the product is still behavior/UX-bound, not CPU-bound.

## The core problem this project is designed around

Tool-calling reliability collapses with local models, and it collapses
silently — unlike Claude/GPT-4, a local model frequently emits malformed
tool calls, or returns pseudo-JSON in plain text instead of a structured
`tool_calls` field. A naive agent loop hard-fails on this. Ollama's default
context length is also only 2048 tokens, which silently truncates prompts.

Consequences for the design:
- Wire protocol: OpenAI-compatible `chat/completions` + `tools` schema — the
  one shape Ollama, LM Studio, and llama.cpp server all speak.
- Small, single-purpose tools (Read/Write/Edit/Bash/Grep/Glob) instead of one
  overloaded tool — easier for a 7B-70B model to call correctly.
- Permission gate is a first-class must-have, not a nice-to-have: local
  models suggest destructive commands more readily than Claude does.
- Context length must always be set explicitly per request, never left at
  the backend's default.
- Aggressive context compaction from day one — design for 8-32K effective
  context, not 200K.

## Reference architectures worth stealing from

- Aider's repo map (tree-sitter + PageRank) for context budgeting — not yet
  implemented in v0.1, but the next major piece of work.
- Goose's "tool shim" idea (fallback path for models with weak native
  function-calling) — implemented as `internal/agent/fallback.go`, added
  2026-09-29 after live-testing against `qwen2.5-coder:7b` on Ollama
  reproduced the exact failure the research predicted: the model returned
  the tool call as a bare JSON object in `content` instead of the
  structured `tool_calls` field, on both `/v1/chat/completions` and native
  `/api/chat`. The fallback only fires when the whole trimmed content is a
  single valid JSON object with a name-like and arguments-like key — it
  deliberately declines (falls through to printing as a normal answer)
  when the model garbles multiple calls into one invalid blob, rather than
  guessing. Confirmed working end-to-end (parse → permission gate →
  execute → result fed back to model) against qwen2.5-coder:7b.

## Reference hardware and validated model (2026-09-29)

Dev machine: AMD Ryzen 5 5600GT (6C/12T), 16 GB RAM, AMD Radeon RX 6500 XT
(Navi 24, 8 GB VRAM), Arch Linux.

GPU acceleration on this card does **not** work through ROCm — RX 6500 XT
is not on Ollama's supported ROCm target list, and the `HSA_OVERRIDE_GFX_VERSION`
workaround did not make it discoverable. What worked: installing the
`ollama-vulkan` package (Arch `extra` repo) alongside the base `ollama`
package. Arch's base `ollama` package ships CPU-only backend libraries
(`/usr/lib/ollama/libggml-cpu-*.so`) with no GPU backend at all — the
Vulkan backend (`libggml-vulkan.so`) only appears after installing
`ollama-vulkan`, which uses the Mesa RADV driver rather than ROCm/HIP. No
env var override needed once that package is installed; Ollama's log
confirms with `library=Vulkan ... description="AMD Radeon RX 6500 XT
(RADV NAVI24)"`.

**Model: `qwen3:8b`** (5.2 GB) is the validated default for this hardware.
Live-tested against offcode on GPU: ~23.5 tokens/sec generation, correct
**native structured `tool_calls`** (not the pseudo-JSON-in-content failure
mode seen with `qwen2.5-coder:7b` — see fallback shim above), and handled
a multi-step tool task (glob → count → answer) cleanly in `auto` mode with
no garbled or duplicated calls. This is the current default recommendation
until something better is validated; `qwen2.5-coder:7b` remains useful as
a stress-test for the fallback parser since it reliably reproduces the
pseudo-tool-call failure mode.

## v0.1 scope

In: single-binary CLI, Ollama + LM Studio backends (both via
OpenAI-compatible endpoint, streaming), six tools (Read/Write/Edit/Bash/Grep/Glob),
three permission modes (plan/default/auto) with a static deny-list for
obviously destructive shell commands, and a Bubble Tea TUI (see below).

Out (deliberately, for later): multi-agent/subagents, MCP, plugins/hooks,
repo-map context budgeting, config file (flags only for now).

## TUI (added 2026-09-29)

Replaced the original blocking REPL with a real terminal UI
(`internal/tui`, Bubble Tea + Bubbles + Lipgloss) once the underlying
streaming plumbing existed to support it — a blocking request/response
loop can't show live token-by-token output or an activity indicator.

What it adds over the REPL:
- Token-by-token streaming of both the model's answer and its `reasoning`
  field (Ollama exposes qwen3's thinking trace as a separate delta field
  alongside `content` — shown as a distinct "thinking:" block).
- A status bar: elapsed session time, context usage (current/max, where
  the backend exposes it via `/api/show`), and measured tokens/sec.
- An activity indicator that reflects what's actually happening
  (thinking / generating / using tool: X / awaiting confirmation) instead
  of a silent hang.
- Visually distinct blocks per message kind (user / assistant / reasoning
  / tool call / tool result / error) so a long transcript doesn't run
  together into a wall of text.
- Ctrl+C interrupts an in-flight generation (cancels the request context,
  returns to the input prompt) rather than only being able to kill the
  whole process; Ctrl+D exits cleanly at any time.

Implementation note that cost real debugging time: `Model` fields must
never be a `strings.Builder` (or anything else with a no-copy guard) when
using Bubble Tea's standard value-receiver `Update(msg) (tea.Model, tea.Cmd)`
pattern — the whole struct gets copied by value on every message, and
`strings.Builder` panics the moment a copied instance is written to
("illegal use of non-zero Builder copied by value"). Streaming
content/reasoning accumulators are plain `string` with `+=` instead.

Streaming wire format notes (Ollama, confirmed by raw testing against
`qwen3:8b`): SSE `data: {...}` lines; each chunk's `choices[0].delta` can
carry `content` and/or `reasoning` independently; `tool_calls` deltas
carry an `index` to accumulate by (Ollama sends the full call in one
chunk in practice, but the accumulation logic handles fragmented
`arguments` strings across multiple chunks too, per the OpenAI streaming
spec, for backends that do fragment it); a final chunk with empty
`choices` carries `usage` when the request sets
`stream_options: {"include_usage": true}`.

## Ollama backend switched to its native /api/chat (2026-09-30)

The OpenAI-compatible `/v1/chat/completions` endpoint silently ignores a
`"think"` field — confirmed by live testing: setting `think:false` on
`/v1` still produced a full reasoning trace. Ollama's **native**
`/api/chat` honors `think` (bool, or `"low"`/`"medium"`/`"high"` for
models that support graded effort) and this is the only way to control
whether a hybrid-reasoning model like qwen3 generates a chain-of-thought
at all — which matters a lot for latency, since the reasoning trace is
often the majority of the tokens generated for a simple request.

Consequence: when `cfg.Backend == "ollama"`, offcode now always streams
via the native endpoint (`internal/backend/native.go`) instead of `/v1`.
LM Studio (and anything else) still goes through the OpenAI-compatible
path — `/v1` remains the portable common denominator for non-Ollama
backends, per the original design decision above; only Ollama gets the
native fast-path, since the think control is Ollama-specific anyway.

Wire format differences from `/v1` worth remembering: native streaming is
newline-delimited JSON (one full object per line), not SSE `data:` lines,
with no `[DONE]` sentinel (just EOF); the reasoning field is called
`thinking`, not `reasoning`; a tool call's `function.arguments` arrives as
a real JSON **object**, not a JSON-encoded string; tool result messages
sent back don't need a `tool_call_id` at all (verified by a round-trip
test — the model continued correctly without one). `/api/tags` lists
locally available models (used by the `/model` picker); `/api/show`
(already used for context-length lookup) is native too.

## Settings as an interactive picker, not typed flags (2026-09-30)

`/settings`, `/model`, and `/effort` open a real navigable list (↑/↓,
enter, esc) instead of requiring the user to remember flag syntax —
requested directly after the first round of slash commands felt like
"shortcuts, not a real choice." `/settings` nests `/effort` and a
permission-mode picker inside it; `/model` fetches the backend's actual
model list live (`Client.ListModels`) rather than a hardcoded guess.
Picking a different model triggers a background context-length refresh
so the status bar's `ctx X/Y` stays accurate.

Copy-paste note: Bubble Tea's mouse-motion capture was removed earlier
for breaking terminal-native text selection, but scrolling the viewport
still repaints the alt-screen and clears any selection the user made —
inherent to how terminal selection works, not fixable from inside the
app. Ctrl+Y sidesteps the whole problem instead of chasing it: it copies
the full transcript (ANSI-stripped) via an OSC 52 escape sequence written
directly to stdout, which works even over SSH and needs no selection at
all.

## Planning, clarifying questions, and web search (2026-09-30)

User's framing: the model would just build something (often low quality —
an HTML calculator with zero CSS) and stop, with no visibility into
whether it had a plan, no way for it to ask when genuinely unsure between
approaches, and no way to check anything against the live internet. Four
additions, all wired through new "meta" tools that the TUI intercepts
itself in `processNextToolCall` rather than routing through
`tools.Dispatch` (they mutate session/UI state, not the filesystem):

- **`plan_set(steps)` / `plan_update(index, status)`** (`internal/tui/plan.go`).
  The system prompt tells the model to plan first for non-trivial tasks and
  update status as it goes. Rendered as a persistent panel between the
  header and the transcript (`planView()` in view.go) — not a scrolling
  block, so it stays visible regardless of scroll position. Gotcha found
  live: building the panel's multi-line box with
  `someStyle.Width(w).Render(joinedLines)` silently reflowed the content
  and dropped the icon prefix from every line but the first — lipgloss's
  width-wrapping doesn't play well with pre-joined multi-line ANSI-styled
  text. Fixed by switching to the same pattern `headerView()` already
  used: build the separator line manually with `strings.Repeat` and just
  `strings.Join` the lines, no `Width().Render()` on the whole block.
  Worth remembering if another multi-line panel gets added later.

- **`ask_user(question, options, recommended_index)`** (also plan.go).
  Opens the existing picker UI (a new `pickerAskUser` kind) showing the
  question and options, recommended one pre-selected and labeled; the
  chosen option's text becomes the tool's result, so the model's next
  turn sees the actual answer and continues from there. No freeform
  text-entry escape hatch in v1 — options-only keeps the implementation
  and the model's usage of it simpler, and covers what was actually
  asked for (a real selection with a recommendation, not open Q&A).

- **Web search / fetch**, backed by a **self-hosted SearXNG** instance
  (user's explicit choice over Brave API or DuckDuckGo scraping — fits
  the project's local-first stance, and the user wanted to "not even
  know it's there"). Set up once, not part of offcode's own code:
  ```
  docker run -d --name offcode-searxng --restart unless-stopped \
    -p 127.0.0.1:8888:8080 \
    -v ~/.offcode/searxng:/etc/searxng \
    searxng/searxng:latest
  ```
  `--restart unless-stopped` is what makes it invisible going forward —
  survives reboots with no action needed. The default settings.yml only
  enables the `html` output format (public-instance anti-scraping
  posture); `~/.offcode/searxng/settings.yml` was patched to add
  `search.formats: [html, json]` and set `server.limiter: false` (fine
  for a single-user localhost-only instance, not appropriate for a
  public one). `internal/tools/web.go` hits `http://127.0.0.1:8888/search
  ?format=json` for `web_search`, and does a crude tag-strip for
  `fetch_url`. Both are read-only in `permission.go` — no confirmation
  needed in any mode, same tier as grep/glob.

- **Output quality policy**: a direct system-prompt instruction to give
  UI/web output real, modern styling rather than bare markup, plus a
  "web results didn't answer it? fetch_url the page, don't guess from
  training data" instruction (caught live: asked for the current Go
  version, got real search results back, and the model still answered
  from stale training-data memory instead of fetching go.dev/dl to
  check). Live-tested the styling instruction against qwen3:8b building
  an HTML calculator: dark card UI, CSS grid, hover states, proper color
  palette — a real, visible improvement over the unstyled output that
  prompted this whole round of changes.

## Four-tier model campaign: per-tier prompts, loop breaker, honest tok/s (2026-10-03)

Added a fourth and final tier below qwen3:8b and above it: `qwen3:1.7b`
(Spark, ultra-lite), `qwen3.5:4b` (Ember, lite), `qwen3:8b` (Forge,
medium — the existing validated default), `gemma4:12b` (Anvil, high — a
new model family for this project). Branded identity (Spark/Ember/Forge/
Anvil) lives in `internal/tiers`, shared by the chat TUI, `offcode
models`, `offcode doctor`, and `offcode monitor`.

Before writing per-tier prompts, ran a full autonomous test campaign (a
forked agent, ~75 tool calls, ~77 minutes) against the real offcode
binary in an isolated sandbox — the same 7-task battery against all four
models, nothing touched under the actual project. Full report kept in
session history; headline finding, and the reason this round of changes
exists:

**All four models failed the same class of task — "build a styled
single-file HTML page" — each in a different way**, which is exactly the
complaint that started this whole effort:
- **Spark** (1.7B): fabricated a complete fake success transcript on a
  moderate multi-file task — zero files created, bash never invoked —
  and did not self-correct when directly challenged by the plan-
  completion nudge.
- **Ember** (4B): correctly used `ask_user` (a real step up from Spark),
  but mid-session convinced itself it "doesn't have web access" despite
  `web_search` being in every request's tool list, and showed outright
  response truncation (stops mid-sentence, no tool call) on harder tasks.
- **Forge** (8B, our known baseline): three consecutive attempts —
  including two direct manual nudges — to build the HTML page all ended
  with detailed reasoning about the CSS approach followed by **no write
  call at all**. The turn just ended. This is the single most important
  finding of the campaign: our best-validated model silently fails on an
  extremely common request.
- **Anvil** (12B, first test): best metacognitive behavior of the four —
  when the plan-completion nudge fired, it explicitly reasoned "the
  system's warning was correct" and tried to fix itself, something no
  other tier did. But on the same large-write task, its `write` call's
  `path` argument came back silently empty/missing (confirmed via direct
  API testing: small payloads include `path` correctly; only large
  `content` values trigger the drop — looks like gemma4's special
  function-call-token mechanism loses an earlier argument while
  generating a long one), and it then **re-emitted the identical
  malformed call for 7+ minutes** without adapting, until manually
  interrupted.

### Mechanism fixes (not just prompt wording)

- **Repeated-identical-tool-call breaker** (`internal/tui/model.go`):
  tracks the last failed `(tool, arguments)` pair and a consecutive-
  failure count. A third identical attempt is denied outright before
  execution — "this exact call has already failed N times, change
  something or explain the problem instead" — rather than trusting the
  model to notice it's looping. Directly targets Anvil's 7-minute loop;
  applies to any tool, not just write.
- **Explicit empty-`path` error** (`internal/tools/tools.go`): write/edit
  now check for a missing `path` *before* attempting the OS call and
  return a pointed message naming exactly which field is missing and
  what the correct shape looks like, instead of the opaque OS-level
  `open : no such file or directory` that gave the model nothing to
  correct from.
- **Honest tok/s** (`internal/backend/native.go`, `types.go`): Ollama's
  native API reports `eval_duration` (pure generation time) separately
  from prompt-processing time; `Usage.EvalDurationNs` now carries it
  through, and both the live status bar and the persistent usage log
  prefer it over wall-clock, which previously understated speed
  (especially on short replies) by including the ~2K-token tool-schema
  reprocessing time on every request.
- Removing `ask_user` from Spark's tool list entirely (not just
  discouraging it in the prompt) — it never used the tool live, so
  cutting it from the schema reduces overhead for zero behavioral loss.

### Per-tier system prompts (`internal/tui/prompts.go`)

One shared base template (identity → environment → core policies → tool
philosophy → risk section), varying only in density/explicitness by
tier rather than structure — this matches what the leaked Claude Code
system prompts do between their smaller and flagship tiers (haiku vs
opus structural research, done before writing these): smaller tiers get
more explicit, imperative, step-by-step phrasing; larger tiers get
terser, more principle-based language. Per the same research, the model
is told its offcode identity as a fact for when it's asked, but never
instructed to volunteer its tier name unprompted — confirmed working
live: asked "what model or AI are you," qwen3:8b answered "I am Offcode
Forge... I don't rely on any external models," correctly declining to
name the underlying vendor.

Two universal additions directly from the campaign's cross-model
finding: "never narrate an action instead of taking it — if you catch
yourself writing 'let me write the code' as plain text, stop and emit
the actual call," and an explicit instruction to double-check a write/
edit call actually includes `"path"` before sending it, especially for
substantial content. Re-tested the exact scenario that failed three
times against qwen3:8b before this change (styled single-file HTML
note-taking app) — it now writes the file on the first attempt, with
genuinely good CSS, and still answers its own identity question
correctly.

Per-tier addenda encode the specific failure mode found for each: Spark
gets told to use tools literally rather than guessing (plus the ask_user
removal above); Ember gets told web_search is real "even if you don't
see yourself using it for a while" and to finish responses rather than
trailing off; Forge gets told explicitly to go from describing content
straight to the write call, nothing in between; Anvil gets its self-
correction instinct reinforced, told not to over-investigate trivial
ask_user-worthy questions, and warned specifically about the large-
payload path-dropping bug with instructions to rebuild (not resend) a
failed large write.

### Config/session/compaction/memory batch (2026-10-04)

Four ROADMAP items shipped together since they all touch the same
startup/persistence path: config file, session persistence, context
compaction, persistent memory.

**Config format: JSON, not TOML.** `~/.offcode/config.json` via
`encoding/json` avoids pulling in a TOML dependency for what's a flat
four-field struct (backend/model/mode/base-url) — not worth a new
import for this shape of data. CLI flags still win over the file
(`runChat` only uses a file value as the flag's *default*), so an
explicit flag always overrides a saved default rather than the reverse.

**Session keyed by SHA256(cwd), not a path-sanitized filename.**
Avoids ever worrying about filesystem-illegal characters, path length
limits, or case-sensitivity differences between Linux and the
eventually-planned Windows support — a hash is trivially portable where
a mangled path isn't. Deliberately only `Messages` round-trips through
a restore; `Model`/`Mode` are saved for the record but never fed back
into `config.Resolve` on load, so a resumed session can't fight with
whatever the current invocation's flags/config file actually resolved
to — the system prompt is always rebuilt fresh for the model actually
in use this run, not whatever model last saved the session.

**Compaction: mechanical, synchronous, not LLM-summarized.** Considered
an LLM-based summary (call the model to compress its own older
history) and rejected it for this pass — it would need a background
call racing against the user's next input, with real state-machine
work to handle "user submits while summarization is still in flight."
The mechanical version (extract requests/tools-used/files-touched into
a structured text block, replace everything but the system prompt and
the last 16 messages) is synchronous, has no race to get wrong, and is
explicitly scoped in ROADMAP.md as the cheap version — an LLM-based
"real" summarizer is left as a distinct, larger future task rather than
bolted on half-finished here.

**Memory: one flat append-only file, not typed entries.** ROADMAP.md
originally sketched project/feedback/user typed entries mirroring this
assistant's own memory system. Shipped the simpler flat-file fallback
design as the actual first version instead: smaller local tiers can't
be trusted to reliably notice "this is memory-worthy" on their own, so
the dependable path is the user explicitly typing `/remember <note>`,
with the `remember` meta-tool as a bonus for tiers that do use it well.
A single markdown list is nothing to get wrong on either path; splitting
it into typed categories can happen later if a flat list proves
limiting in practice.

**Verifying "it works" against the filesystem, not the terminal.**
While testing compaction with lowered thresholds, tmux's own scrollback
truncation plus a test-harness timing bug (sending the next message
while the model was still mid-generation — input is correctly disabled
then, so those keystrokes were silently dropped) together looked like
an app bug ("messages disappearing"). Re-verified by reading
`~/.offcode/sessions/*.json` directly instead of trusting tmux's capture
— the compaction logic was correct from the first implementation; the
false alarm was entirely in the test methodology, not the app.

### Web interface (2026-10-04)

**A parallel engine, not a refactor of the TUI.** `internal/tui`'s loop
is written in Bubble Tea's continuation-passing style — every step
(stream event, tool result, confirm answer) returns control to an event
loop via a `tea.Msg`/`tea.Cmd` pair. That shape doesn't translate to an
HTTP server: there's no outer event loop to hand control back to between
an HTTP request and the next one. Rather than bend the TUI's model to
serve both, or risk regressing a working TUI to extract a shared core,
`internal/webengine` is a clean second implementation: one dedicated
goroutine per session (`runTurn`) runs the whole agent loop as ordinary
**blocking, sequential** Go code — a tool confirmation or ask_user pause
is just `allow := <-s.confirmCh`, no state machine required, which is
actually simpler than the TUI's CPS version despite being a second copy
of the logic. The duplication this creates (prompts.go, metatools.go,
research.go each have a near-identical twin in internal/tui) is an
accepted, deliberate cost — a shared package would mean touching the
TUI's imports, and zero regression risk to the proven TUI was the
explicit priority for this item.

**State sync: full snapshot broadcast, not a diff/patch protocol.**
Every SSE push sends the *entire* current `State` (all transcript blocks,
plan, confirm/ask pending state) as one JSON blob, on every change —
including once per streamed token during generation. Bandwidth-wasteful
in the abstract, irrelevant in practice on localhost. In exchange, there
is no client/server drift to debug: the browser always renders exactly
what the server thinks is true, never a diff that could desync after a
dropped event or a reconnect. `EventSource`'s built-in auto-reconnect
plus a fresh full snapshot on every new connection means a page refresh
or a flaky connection just re-syncs for free.

**One server = one session, keyed like the TUI.** Rather than a
multi-tenant session manager with IDs, `offcode web` holds exactly one
`*webengine.Session`, created from the same `internal/session`
cwd-keyed store the TUI already uses. Multiple browser tabs against one
running server see and drive the same conversation — intentional,
matching "multiple terminals attached to one tmux pane" rather than
pretending the web UI is a separate, independent client. Running the
TUI and `offcode web` in the same directory at the same time will race
on the same session.json; acceptable for a personal local tool, not
engineered around.

**No new JS dependencies, no build step.** The frontend is hand-written
vanilla HTML/CSS/JS served via `go:embed` — no npm, no bundler, nothing
to install to run `offcode web` beyond the Go binary itself. Consistent
with the project's single-static-binary goal; pulling in a frontend
build pipeline for one page would work against that.

### MCP client + browser automation (2026-10-04)

Two real bugs caught by live testing before either feature shipped —
worth recording since both are the "looked right in the diff, broke in
actual use" kind.

**MCP subprocess killed by the wrong context.** The first working
version of `internal/mcpclient.Connect` ran the server subprocess via
`exec.CommandContext(ctx, ...)` where `ctx` was the *same* context used
to bound a 15s timeout around the initial handshake (`Connect`/
`ListTools`). That context gets canceled as soon as the handshake
finishes — which, via `exec.CommandContext`'s documented behavior, kills
the process it governs. Result: every MCP server would die moments
after connecting, silently, and the first real tool call afterward would
fail. Fixed by giving the subprocess its own `context.Background()`
(stopped only via `session.Close()`), while keeping the short timeout
context *only* around the handshake calls. Caught before it ever reached
live testing against offcode itself, because
`TestServerSurvivesConnectContextExpiry` was written specifically to
simulate "the caller's context dies right after Connect returns" — a
scenario the happy-path integration test alone would never have
exercised, since that test's own context stayed alive for its whole
60s budget.

**A full-resolution PNG screenshot broke the vision model's tokenizer.**
`preview_page`'s first version captured at 1280x800 and returned raw
PNG. Live-tested against gemma4:12b: the screenshot itself worked and
got attached to a tool-result message correctly, but the *next* turn
(after plan-nudging asked it to continue) failed with a backend error —
`"Failed to tokenize prompt"`. The base64-encoded PNG re-sent as part of
the full message history was large enough to break something in
Ollama's vision tokenization path. Fixed by switching to JPEG (quality
70, ~5-10x smaller than PNG for a UI screenshot) at a smaller default
viewport (1024x640 instead of 1280x800) — confirmed via a re-test of the
exact same live scenario, which then completed with an accurate
description and no error. Added `TestPreviewPageDefaultSizeIsReasonable`
asserting the default-viewport payload stays under 500KB, so a future
change can't silently regress this.

Both bugs share a lesson worth stating plainly: a tool that "worked" in
isolated unit tests or a single successful call isn't fully verified
until it's exercised across *multiple turns* of the real agent loop —
the failure in both cases only showed up one step after the tool itself
appeared to succeed.

**Unconfigured extension tools were offered to every model anyway.**
First live campaign against the real 5-tier Ollama setup (2026-10-10):
Offcode Spark (qwen3:1.7b), asked for a single HTML/CSS note-taking
page, called `generate_image` for a decorative mockup instead of just
writing the file. ComfyUI wasn't configured on this machine, so the
call was denied, Spark then poked at `browser_text` on a file that
didn't exist yet, and the turn ended without `notes.html` ever being
written. `extensionDefinitions()` (`internal/tools/extensions.go`) was
unconditionally listing `generate_image`/`generate_video`/
`generate_music`/`export_document`/`transcribe_audio`/
`synthesize_speech` regardless of whether ComfyUI/Pandoc/whisper-cli
were actually present — the same mistake vision-gating
(`view_image`/`preview_page`, only offered when `SupportsVision` says
yes) had already solved for a different tool. Fixed by adding
`extensionAvailability()`, checked once per tool-list build: each
gated tool only appears when its real dependency is there (binary on
PATH, `~/.offcode/media.json` configured with that specific kind's
workflow file present, `OFFCODE_WHISPER_MODEL` pointing at a real
file). `repo_map`/`git_summary` stay ungated — they only need `git`,
which is safe to assume present in a coding session. Covered by
`TestExtensionGatingHidesUnconfiguredTools` /
`TestExtensionGatingOffersConfiguredTools`, which fake PATH/HOME rather
than depending on what happens to be installed on the machine running
the test.

**Ollama's own tool-call template parser can fail on a model's malformed
tags, killing the whole turn.** Same campaign, Offcode Ember
(qwen3.5:4b): three different tasks (a plan-and-write task, the
ask-user-clarification task, and the web-search task) all ended with
`backend error: XML syntax error on line N: ...` — Ollama's native
`/api/chat` extracts `<function>`/`<parameter>`-style tool-call tags
out of the model's raw generation for this template, and when Ember's
tags came out unbalanced, Ollama returned a hard error instead of a
recoverable one. offcode treated any such error as fatal
(`finishRun`/`EventError` ended the turn immediately), even though the
model produced a real response — only Ollama's server-side extraction
of it failed, and a different sampling run of the same prompt has a
real chance of producing well-formed tags. Fixed with a bounded retry:
`backend.IsMalformedToolCallResponse` recognizes this specific error
signature, and `streamOnceWithParseRetry`
(`internal/webengine/loop.go`) / the mirrored `EventError` handling in
`internal/tui/model.go` retry up to `maxToolCallParseRetries` (2) times
before giving up — any other error (backend down, model missing,
canceled) still fails on the first attempt, unchanged. Covered by
`TestStreamOnceWithParseRetry*` in `internal/webengine` (fake HTTP
transport returning the error then a clean response) and
`TestIsMalformedToolCallResponse` in `internal/backend`.

**The same template fragility can corrupt an argument key instead of
erroring outright, and `write` silently wrote an empty file.** One
Ember (qwen3.5:4b) task never hit the XML error at all but still
failed: its `write` call arrived as
`{"path":"...","":"content>\n<!DOCTYPE html>..."}` — a `<parameter=
content>`-style tag the template failed to extract a name from, so the
value landed under an empty-string key instead of `"content"`. Before
this fix, `tools.Dispatch("write", ...)` only checked `path` was
non-empty and happily wrote `Content` as its Go zero value (`""`),
producing a real file on disk with zero bytes and no error — silent
data loss the model and user would only notice much later. Fixed by
unmarshaling into a `map[string]json.RawMessage` first: a missing
`"content"` key alongside any *other*, unexpected key is now treated
as a corrupted call and rejected with a message asking the model to
retry (`rawKeys` lists what it actually sent). A single-key
`{"path":"x.txt"}` call — no stray key, content genuinely omitted —
still succeeds and creates an empty file, since that's a legitimate,
deliberate request. Covered by
`TestWriteRejectsCorruptedContentKey`/`TestWriteAllowsDeliberateEmptyFile`.

**offcode never told Ollama how much context to actually use — Ollama's
own VRAM-based guess silently truncated real turns.** Live-tested
2026-10-10, auto mode (not the eval harness — full bash approval, no
allowlist restriction): Offcode Anvil (gemma4:12b), asked the same
"suggest a feature" question that looked stuck under eval, explored the
project fine (`ls -R` succeeded, no denial this time) but the turn
still ended idle with a dangling reasoning block and no final answer —
same symptom as the eval-harness run, so the earlier "Anvil loops on
denied bash" theory was wrong. Ollama's own log showed why:
`stop processing: n_tokens = 4095, truncated = 1`. `StreamChatNative`
(`internal/backend/native.go`) never set `options.num_ctx` in the
request, so Ollama fell back to its own VRAM-based heuristic — 4096
tokens on this 8GB card — regardless of the model's actual supported
context (gemma4:12b advertises up to 262144). offcode's own system
prompt plus ~20 tool schemas is already around 2K tokens before the
conversation even starts, so a reasoning-heavy turn had nowhere near
enough room. The UI's context-usage indicator made this worse, not
better: it's driven by `Client.ContextLength` (the model's advertised
max), so it kept showing lots of headroom while Ollama was silently
discarding tokens under the hood. Fixed by always sending an explicit
`num_ctx` (`internal/backend/native.go`'s `numCtxOption`) — default
8192 (a deliberate, modest bump over Ollama's 4096, chosen to stay
safe on small-VRAM hardware rather than requesting a model's full
advertised context and risking a load failure), overridable via
`OFFCODE_NUM_CTX`. Re-tested the identical live scenario after the fix:
Ollama's log confirmed `n_ctx_slot = 8192` and `truncated = 0` — the
retry still didn't get a final answer, but for an unrelated reason (an
AMD GPU compute-ring timeout mid-generation, `amdgpu ... ring comp_1.3.0
timeout` in `dmesg`, after ~2.5 hours of continuous heavy GPU use this
session — the driver reset and recovered fine; a hardware/driver
limitation of this specific low-end card under sustained load, not an
offcode bug). Covered by `TestStreamChatNativeSetsNumCtx` and
`TestStreamChatNativeNumCtxEnvOverride` in `internal/backend`.

**A turn could end with literally nothing shown, even after real work got
done.** Seen three separate times live with Offcode Anvil (gemma4:12b) —
most recently 2026-10-10 with a short, vague prompt ("Chci malou to-do
appku do prohlížeče") rather than a spelled-out requirements list: it
planned on its own (4 steps, single-file HTML+CSS+JS, correctly used
`lang="cs"` for the Czech prompt), wrote a genuinely working
`todo_app.html` (localStorage, add/complete/delete), but never called
`plan_update` to mark its own step done. The plan-nudge system message
fired twice (`maxPlanNudges`), and on the third attempt the model
returned empty content with no tool calls. `runTurn`
(`internal/webengine/loop.go`) hit `s.finishRun(nil)`, which — since
`err` was `nil` — added no block at all: the transcript just stopped,
with a real file sitting on disk that nothing in the UI pointed to.
Fixed by checking, right before that `finishRun(nil)`, whether plan
steps are still incomplete and content came back empty — if so, push an
explicit system block naming which steps are still open and how many
reminders were sent, instead of silent nothing. Doesn't change model
behavior (gemma4:12b still won't reliably call `plan_update`), just
makes the actual outcome visible so the user isn't left guessing whether
anything happened. Mirrored in `internal/tui/model.go`'s equivalent
path. Covered by `TestRunTurnExplainsExhaustedPlanNudges`
(`internal/webengine`), which fakes a model that always returns empty
content and asserts exactly one explanatory block appears after the
nudge budget is spent.

**offcode never tried to start Ollama itself, and "connection refused" is
not a self-explanatory error for most users.** User feedback, 2026-10-10
(new session, Ollama had been manually stopped since the prior session —
no `ollama serve` was running): "nějak se ollama nespustí když zapnu
offcode" (Ollama somehow doesn't start when I launch offcode) — the
expectation was that launching offcode would just work, the same way the
Ollama desktop app or its systemd service would normally keep it running
in the background. On this dev machine specifically, `ollama.service` is
installed but ships **disabled** (confirmed via `systemctl status
ollama` — Arch doesn't enable it by default), so nothing auto-starts it
between sessions; a user who manages Ollama manually (or whose systemd
unit isn't enabled) hit a raw Go `connection refused` error on every
tool call instead. Fixed with `ensureOllamaRunning`
(`cmd/offcode/ollamaboot.go`), called from both `runChat` and `runWeb`
right after config resolution: if the backend is Ollama and nothing
answers at the configured URL, best-effort `exec.LookPath("ollama")` and
spawn `ollama serve` detached (own session via `Setsid` on
Linux/macOS, `CREATE_NEW_PROCESS_GROUP` on Windows — split across
`ollamaboot_unix.go`/`ollamaboot_windows.go` the same way
`internal/sysmon` already does), then poll for up to 10s before
continuing regardless. Live-verified end to end: killed Ollama, launched
`offcode web`, watched it print "Ollama wasn't running — starting it
now..." and become reachable within the poll window; then confirmed
`Setsid` actually matters by sending SIGINT to offcode and checking
`ollama serve` was still running afterward (a plain `cmd.Start()`
without detaching would have taken it down too, since a terminal Ctrl+C
delivers SIGINT to the whole foreground process group). Does nothing
silently when the binary isn't on PATH, the backend is LM Studio, or
Ollama is already reachable — same connection-refused error as before
surfaces if auto-start doesn't help. Covered by
`cmd/offcode/ollamaboot_test.go`.

## 2026-10-11 — Installer preferences, Ollama preservation and startup updates

The Windows GPU report could not be reproduced on the user's Windows machine
from this Linux environment. Inspection found no `num_gpu=0` or forced CPU
flag, but found two concrete integration problems: `OLLAMA_HOST` was ignored,
and every native chat request overrode server/Modelfile context with 8192.
A larger context may alter VRAM placement. We cannot claim either was the
specific cause of the user's RX 6500 XT report.

All installers now call the same `offcode setup` implementation. Preserve is
the default: save only an allowlist of backend/GPU environment settings as a
fallback, honor current environment (plus missing Windows registry values),
and omit the native context override unless OFFCODE_NUM_CTX was explicitly
set. Auto retains the 8192 context choice; external never auto-starts Ollama.
No service, GPU driver, running Ollama or existing model files are modified.
Settings only present in a custom service/launcher must be kept by using that
service; Offcode cannot reconstruct an arbitrary launcher from its client
process. Autostart is restricted to local loopback HTTP, uses the selected
port, and reaps the detached child after it exits. Doctor now uses the selected
Ollama endpoint and reports loaded-model VRAM from `/api/ps`.

The setup command offers all five tiers, semantic-search embeddings and custom
tags. It lists installed models first, skips them without network pulls, streams
missing-model pull progress, handles API-level errors and truncated streams,
and sets a default only after a successful model selection. Existing prompts,
tier identities, skills and web assets already ship embedded; duplicating these
into Ollama Modelfiles would create drift and duplicate system instructions.
Optional media, voice, Pandoc and search services remain separate dependencies.

Startup updates run before the TUI or web server starts. They compare stable
numeric versions, timeout checks in five seconds, allow auto/notify/off, fetch
only official matching release assets, verify SHA256 and exact size, stage in
the installation directory, preserve permissions, rename the previous binary
with rollback on replacement failure, and relaunch with original arguments.
OS file locking handles simultaneous launches and releases locks on crashes.
A raw old binary is retained as .previous; no config/models/sessions are touched.
Release CI now injects the tag into AppVersion and publishes SHA256SUMS.
Development builds skip updates. Existing v0.2.1 installs require a one-time
new installation; no release has been published during this change.

Verification: full short Go suite with race detection, go vet, Linux build and
Windows amd64 cross-build. Added regressions cover host/flag precedence, saved
environment precedence/allowlist, context omission, model selection, successful
and failed/truncated pull streams, stable-version comparisons, HTTP/offline
errors, checksum mismatch, partial download, foreign URLs, rollback, concurrent
updater locks and replacement of an actual running child executable. Windows
native execution and this user's GPU configuration remain unverified locally.
References: https://docs.ollama.com/faq and https://docs.ollama.com/gpu.

Additional verification used the built CLI against an isolated local HTTP
fixture: it reused installed Spark/embedding models, pulled only missing Forge,
selected Spark as the default, saved the GPU environment and server address,
and reused that saved endpoint in a second process with no OLLAMA_HOST set.
Both workflow YAML files parsed successfully. Windows CI now also parses the
PowerShell script, compiles Inno Setup and runs the PowerShell local install /
setup / uninstall smoke path; those Windows checks have not run locally.

## 2026-10-11 — Granular preservation checkboxes

The user requested per-setting choices instead of preserving everything at
once. Seven independent groups now cover server address, GPU settings, model
storage, context, parallelism, Flash Attention/KV cache, and keep-alive. Inno
Setup renders native checkboxes; the shared cmd/PowerShell setup wizard uses
an arrow/Space checkbox picker, with a numbered fallback for redirected input.
Automation uses `-preserve=server,gpu` (or all/none); PowerShell forwards
`-Preserve`. Existing-server-only startup is independent of the selections.

A nil selection migrates previous settings (preserve/external = all, auto =
none), while an explicit empty array persists an unchecked selection. Capturing
or applying settings filters by the checked groups and drops unchecked saved
values. Main no longer applies old fallback settings before running setup,
preventing removed groups from leaking into the new selection. Current process,
Windows registry and running service configuration are never destructively
reset; unchecked means no stored fallback. Context unchecked restores Offcode's
normal 8192-token request default unless explicitly overridden by the user.

Regressions cover server-only restore, independent context/GPU selection,
dropping previously saved groups, current environment precedence, migration,
empty selection round-trip, invalid selection and repeated setup changes.

Validation: full short race suite and go vet passed; Linux and Windows amd64
binaries rebuilt. A real PTY run selected only GPU using arrow/Space keys and
verified the old saved server fallback disappeared; redirected-input setup
selected only the server. Windows CI additionally checks PowerShell forwarding
of the server-only selection. Native Windows GUI execution remains pending.

## 2026-10-14 — Preserved context only when one is actually configured

Live plan-reliability runs on the RX 6500 XT showed Ollama logging
`truncating input prompt limit=2050 prompt=4299` with `n_ctx_slot = 4096`:
qwen3.5:4b lost the user's request and hallucinated an unrelated project.
Cause: with no `~/.offcode/setup.json`, `setup.Load` defaults to preserve
mode, `Keeps("context")` is true, and `Apply` set `OFFCODE_NUM_CTX=0`, so
no `num_ctx` was sent and Ollama used its VRAM-based 4096 default — the
exact failure fixed on 2026-10-10, back for every user without a custom
context.

The 2026-10-11 intent (do not override a context the user chose) is kept,
but only applies when such a choice exists: `Apply` now disables the
request context only if `OLLAMA_CONTEXT_LENGTH` is set (current or saved),
and the native client skips `num_ctx` when the model's Modelfile parameters
already set it (`/api/show`, cached per model). An explicit
`OFFCODE_NUM_CTX` still wins over both. Regressions:
`TestContextPreservedOnlyWhenConfigured`,
`TestStreamChatNativeRespectsModelfileNumCtx`.

## 2026-10-15 — Incomplete native streams are backend failures

During the resumed Gemma plan campaign, the Vulkan runner reported
`vk::Queue::submit: ErrorDeviceLost` while processing the initial prompt.
Offcode's first transcript showed an empty-reply reminder and then stopped,
so this run cannot be used to judge model quality. CPU validation is tracked
separately in HANDOFF.md.

Inspection found that the native client accepted EOF without ever receiving
Ollama's `done: true` chunk. It now returns an unexpected-EOF error instead
of finalizing an empty/partial answer or handing partial tool calls to the
agent loop. Regression coverage includes an empty body, partial text and
partial tool calls. This fixes the incomplete-stream case; the raw failing
GPU response was not captured, so it does not establish that every Vulkan
failure takes this path or repair the underlying backend/driver failure.

## 2026-10-15 — Out-of-order plan_update points at the open steps

In the CPU long-suite run, qwen3:8b did the work for every step of the
inventory task but called `plan_update` only for the last step. The handler
then auto-promoted the first pending step, so steps 0 and 1 were both
`in_progress` and the result text gave no hint that four steps were still
open. Only the end-of-turn reminder recovered it.

Marking a step done or skipped now lists any earlier unfinished steps and asks
the model to mark them done if the work is finished, or to do it first. No step
is auto-promoted while earlier steps are open or another step is already
`in_progress`. In-order completion behaves as before. Web engine and TUI are
both changed; regression: `TestPlanUpdateOutOfOrderPointsAtEarlierSteps`.
The same run also had qwen3:8b write `\n` literally (double-escaped JSON) into
`inventory.csv` and then claim it verified the file; that is model quality, the
write tool stored exactly what it received.
Live-confirmed afterwards on GPU qwen3:8b (settings, repeat 2): after marking
step 2 done before step 1 and receiving the hint, it marked step 1 done next.
