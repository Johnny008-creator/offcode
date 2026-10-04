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
