# offcode

A Claude-Code-like agentic coding CLI built exclusively for local models —
Ollama, LM Studio, and anything else that speaks the OpenAI-compatible
`chat/completions` API. Cross-platform: Linux and Windows.

Design rationale and what's deliberately out of scope for v0.1 live in
[docs/DECISIONS.md](docs/DECISIONS.md).

## Status

v0.1: a terminal UI (Bubble Tea) with streaming responses, a live activity
indicator (thinking / generating / using tool: X), a status bar (elapsed
time, context usage, tokens/sec), six tools (read, write, edit, bash, grep,
glob), three permission modes, Ollama/LM Studio backends, and a fallback
parser for models that emit tool calls as plain-text JSON instead of using
the structured `tool_calls` field (confirmed necessary in practice against
`qwen2.5-coder:7b`). No context compaction yet.

Ctrl+C interrupts an in-flight generation; Ctrl+D exits.

## Build

```sh
go build -o bin/offcode ./cmd/offcode
```

## Run

Requires Ollama or LM Studio running locally with a tool-calling-capable
model loaded.

Recommended default: `qwen3:8b` — validated against offcode with reliable
native tool-calling and good speed on modest hardware (see
[docs/DECISIONS.md](docs/DECISIONS.md) for benchmarks and AMD/Vulkan GPU
setup notes).

```sh
ollama pull qwen3:8b
./bin/offcode -backend ollama -model qwen3:8b
./bin/offcode -backend lmstudio -model devstral-small
```

Flags:

- `-backend` — `ollama` (default) or `lmstudio`
- `-model` — required, model name as known to the backend
- `-base-url` — override the backend's default OpenAI-compatible URL
- `-mode` — `plan` (read-only), `default` (confirm writes/bash), or `auto`

Type `/exit` to quit.
