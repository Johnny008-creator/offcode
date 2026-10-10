# offcode

A Claude-Code-like agentic coding CLI built exclusively for local models —
Ollama, LM Studio, and anything else that speaks the OpenAI-compatible
`chat/completions` API. Cross-platform: Linux and Windows.

Design rationale and what's deliberately out of scope live in
[docs/DECISIONS.md](docs/DECISIONS.md).

## Download

Grab the latest build from [GitHub Releases](https://github.com/Johnny008-creator/offcode/releases/latest):
Linux binary, Windows `.exe`, or the Windows installer (GUI/cmd/PowerShell —
see "Windows installer" below). Or build from source, see "Build".

## Status

v0.2: a terminal UI (Bubble Tea) with streaming responses, a live activity
indicator (thinking / generating / using tool: X), a status bar (elapsed
time, context usage, tokens/sec), filesystem/shell/web tools (read, write,
edit, bash, grep, glob, web_search, fetch_url, browser_text), three
permission modes, Ollama (native API) and LM Studio (OpenAI-compatible)
backends, and a fallback parser for models that emit tool calls as
plain-text JSON instead of using the structured `tool_calls` field
(confirmed necessary in practice against `qwen2.5-coder:7b`). Includes
context compaction, opt-in session resume, persistent memory, a web UI
(`offcode web`), MCP client support, browser automation, multi-agent
subagents, vision, hooks, a 20-topic on-demand skills library, semantic
codebase search, Windows cross-build with native CPU/memory monitoring, a
Windows GUI/cmd/PowerShell installer, and local extensions for document
export, generative media, and voice (see "Extended capabilities").
Five real bugs found through live multi-tier Ollama testing — silently
truncated context, a swallowed Ollama tool-call-template error, an
unconfigured-tool trap, a silently-empty file write, and a silently-ended
turn — are fixed and documented in [docs/DECISIONS.md](docs/DECISIONS.md).

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
model loaded. For the Ollama backend (the default), offcode auto-starts
`ollama serve` if it isn't already running and the `ollama` binary is on
PATH — it waits up to 10s for it to come up, then continues either way;
if that doesn't help or `ollama` isn't on PATH, start it yourself
(`ollama serve`, or `systemctl start ollama` if installed as a service —
on Arch it ships disabled by default). LM Studio must always be started
manually first; offcode doesn't attempt to launch it.

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

## Windows installer

Three ways to install on Windows, all installing to
`%LocalAppData%\Programs\offcode` and adding it to the current user's PATH
(no admin rights needed):

- **GUI installer** — `offcode-setup-<version>.exe`
  (`scripts/windows/offcode.iss`, built by Inno Setup). Double-click for the
  normal wizard window, or run it unattended straight from `cmd`/a script
  with Inno Setup's own silent switches:
  ```bat
  offcode-setup-0.2.0.exe /VERYSILENT /SUPPRESSMSGBOXES
  ```
- **Batch installer** — `scripts/windows/install.bat`, plain `cmd`, no
  PowerShell: downloads the latest release (via `certutil`) or installs an
  exe you already built (`install.bat path\to\offcode.exe`).
  `install.bat /uninstall` removes it.
- **PowerShell installer** — `scripts/windows/install.ps1`:
  ```powershell
  irm https://raw.githubusercontent.com/Johnny008-creator/offcode/master/scripts/windows/install.ps1 | iex
  ```
  or `.\install.ps1 -LocalExe ..\..\bin\offcode.exe`; `-Uninstall` to remove.

All three work equally from `cmd` or PowerShell once installed — they
write the same `HKCU\Environment\Path` registry entry Windows itself
exposes to every shell — and all three also broadcast `WM_SETTINGCHANGE`
so a **new** terminal opened right after (Start Menu, Win+R) picks up the
PATH immediately. None of them can refresh a terminal that was already
open before you ran the installer — Windows consoles only read their
environment once, at launch — so open a fresh one either way.

Tagged releases (`.github/workflows/release.yml`) build `offcode.exe` and
`offcode-setup-<version>.exe` on `windows-latest` (Inno Setup ships
preinstalled on that runner image) and attach both, plus the Linux binary,
to the GitHub release. None of the three installers have been run on a real
Windows machine yet — only cross-built/vetted and reviewed on Linux, per
[docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md).

## Semantic code search

With an Ollama backend, `semantic_search` finds code by meaning and returns
file paths, line numbers, and matching excerpts. Install its local embedding
model once:

```sh
ollama pull nomic-embed-text
```

Then ask offcode, in the terminal or web UI, to find the code responsible for
a behavior using semantic search. The first call indexes supported text
files; later calls re-embed changed files and remove deleted entries. The
cache lives in `.offcode/semindex.json` in the project (add `.offcode/` to
your project's `.gitignore`). Hidden and common generated directories,
symlinks, and files larger than 300 KB are skipped; indexing is limited to
3,000 files. LM Studio is not supported by this tool yet.

## Validation

Run the offline suite (including the race detector), static checks, and build:

```sh
go test -short -race ./...
go vet ./...
go build -o bin/offcode ./cmd/offcode
```

`-short` skips integration tests requiring Ollama, Chromium, or the external
MCP filesystem server. Run `go test ./...` in an environment where these
services can run; the MCP tests use `npx` and may require network access.
Live semantic ranking can be checked separately with:

```sh
go test -v ./internal/semsearch -run 'TestSearchFindsSemanticMatch|TestSearchIncrementalReindex'
```

These two live tests report `SKIP` when the embedding model is unavailable;
a skipped test does not confirm semantic ranking quality. Offline tests use
synthetic vectors to verify protocol handling, indexing, cache invalidation,
and the TUI/web tool integration.

## Extended capabilities

- `generate_image`, `generate_video`, `generate_music`: configured local ComfyUI workflows.
- `export_document`: Markdown to DOCX/PDF through local Pandoc.
- `transcribe_audio`, `synthesize_speech`: local Whisper/FFmpeg and eSpeak NG;
  the web UI has microphone dictation and response playback.
- `repo_map`, `git_summary`: bounded repository navigation and review context.
- `offcode learn add` and `scripts/training/`: reviewed datasets, LoRA training,
  periodic retraining and model deployment (`offcode learn deploy`).
- `scripts/eval/run.py`: isolated ten-task campaigns for five tiers or LM Studio.
- Windows build with native CPU/memory monitoring, a Linux/Windows CI matrix,
  and a GUI/cmd/PowerShell installer (see "Windows installer" above).

Run `offcode doctor -extensions` to inspect optional dependencies. These
capabilities do not install or bundle large models and services. Setup and
limits are in [EXTENSIONS](docs/EXTENSIONS.md), [TRAINING](docs/TRAINING.md),
and [EVALUATION](docs/EVALUATION.md). The current implementation/validation
checklist is [IMPLEMENTATION](docs/IMPLEMENTATION.md).
