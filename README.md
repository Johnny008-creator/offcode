# offcode

A Claude-Code-like agentic coding CLI built exclusively for local models —
Ollama, LM Studio, and anything else that speaks the OpenAI-compatible
`chat/completions` API. Cross-platform: Linux and Windows.

Design rationale and what's deliberately out of scope for v0.1 live in
[docs/DECISIONS.md](docs/DECISIONS.md).

## Status

v0.1: a terminal UI (Bubble Tea) with streaming responses, a live activity
indicator (thinking / generating / using tool: X), a status bar (elapsed
time, context usage, tokens/sec), eight filesystem/shell/web tools (read,
write, edit, bash, grep, glob, web_search, fetch_url), three permission
modes, Ollama (native API) and LM Studio (OpenAI-compatible) backends, and
a fallback parser for models that emit tool calls as plain-text JSON
instead of using the structured `tool_calls` field (confirmed necessary in
practice against `qwen2.5-coder:7b`). No context compaction yet.

The model plans non-trivial tasks up front and tracks progress (a
persistent checklist panel above the transcript), can ask you to pick
between options when genuinely unsure (with a recommendation), and can
search the web / fetch a page when it needs current information instead
of guessing from training data. A write to a file that exists but hasn't
been read this session always asks for confirmation first, even in `auto`
mode — `write` replaces the whole file, so writing one the model hasn't
actually seen is a guess, not an edit (found the hard way: it once
silently gutted this repo's own `go.mod`).

**Keys:** Ctrl+D exit · Ctrl+C interrupt generation (or exit if idle) ·
↑/↓/PgUp/PgDn scroll transcript · Ctrl+T toggle the thinking/reasoning
trace · Ctrl+Y copy the whole transcript to clipboard (OSC 52, works over
SSH, no text selection needed).

**Slash commands:** `/settings` — interactive menu (thinking display,
reasoning effort, permission mode, model) · `/model` — pick a different
model from what the backend has loaded · `/effort` — set reasoning effort
(`default`/`off`/`low`/`medium`/`high`, Ollama only) · `/thinking [on|off]`
— quick toggle without opening the menu.

## Build

```sh
go build -o bin/offcode ./cmd/offcode
```

## Run

Requires Ollama or LM Studio running locally with a tool-calling-capable
model loaded.

```sh
./bin/offcode
```

No flags needed — leaving out `-model` opens the model picker on startup
instead of erroring, so this just works as long as something's pulled in
Ollama. Flags are there for skipping straight past that when you already
know what you want:

```sh
./bin/offcode -backend ollama -model qwen3:8b
./bin/offcode -backend lmstudio -model devstral-small
```

Flags:

- `-backend` — `ollama` (default) or `lmstudio`
- `-model` — optional; omit it to pick from a live list on startup
- `-base-url` — override the backend's default OpenAI-compatible URL
- `-mode` — `plan` (read-only), `default` (confirm writes/bash), or `auto`

`web_search`/`fetch_url` need a local SearXNG instance at
`127.0.0.1:8888` — see [docs/DECISIONS.md](docs/DECISIONS.md) for the
one-time `docker run` setup (`--restart unless-stopped`, so once it's up
it just keeps running). Without it those two tools return an error; the
rest of offcode works fine either way.
