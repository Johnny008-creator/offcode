# offcode

A Claude-Code-like agentic coding CLI built exclusively for local models —
Ollama, LM Studio, and anything else that speaks the OpenAI-compatible
`chat/completions` API. Cross-platform: Linux and Windows.

Design rationale and what's deliberately out of scope for v0.1 live in
[docs/DECISIONS.md](docs/DECISIONS.md).

## Status

v0.1 skeleton: a working REPL agent loop with six tools (read, write, edit,
bash, grep, glob), three permission modes, and Ollama/LM Studio backends.
No streaming, no TUI, no context compaction yet.

## Build

```sh
go build -o bin/offcode ./cmd/offcode
```

## Run

Requires Ollama or LM Studio running locally with a tool-calling-capable
model loaded (e.g. `qwen2.5-coder:32b`, `qwen3-coder`, `devstral`).

```sh
./bin/offcode -backend ollama -model qwen2.5-coder:32b
./bin/offcode -backend lmstudio -model devstral-small
```

Flags:

- `-backend` — `ollama` (default) or `lmstudio`
- `-model` — required, model name as known to the backend
- `-base-url` — override the backend's default OpenAI-compatible URL
- `-mode` — `plan` (read-only), `default` (confirm writes/bash), or `auto`

Type `/exit` to quit.
