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

In: single-binary CLI, REPL loop, Ollama + LM Studio backends (both via
OpenAI-compatible endpoint), six tools (Read/Write/Edit/Bash/Grep/Glob),
three permission modes (plan/default/auto) with a static deny-list for
obviously destructive shell commands.

Out (deliberately, for later): TUI (Bubble Tea), streaming responses,
multi-agent/subagents, MCP, plugins/hooks, repo-map context budgeting,
config file (flags only for now).
