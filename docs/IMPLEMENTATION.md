# Roadmap implementation status — 2026-10-09

The user requested the remaining roadmap capabilities. Implementation and live
verification are tracked separately: a mock protocol test is not a successful
model run, and a skipped integration test is not a pass.

## Implemented in the working tree

- [x] Semantic search in TUI/web, incremental indexing, cache invalidation and
  offline regression coverage.
- [x] Windows cross-build, native CPU/memory counters and Linux/Windows CI jobs.
- [x] PDF/DOCX export from Markdown through Pandoc; preserve existing files and
  do not publish failed output.
- [x] Local image/video/music integration through configured ComfyUI API graphs,
  polling, error/cancellation reporting, and artifact download.
- [x] Local speech transcription and synthesis, web microphone input and playback.
- [x] Reviewed per-tier training examples, LoRA training, optional merged weights,
  periodic retraining on changed data, and `offcode learn deploy`.
- [x] Repeatable ten-task runner for five tiers or explicit LM Studio model tags.
- [x] Web image/audio/video previews and downloads, diff confirmation preview,
  token-usage charts, and correct redraws when text changes without changing length.
- [x] Bounded Go-AST repository map and git change summary tools.
- [x] Optional dependency diagnostics through `offcode doctor -extensions`.
- [x] Cancellable long tools and web confirmation waits; serialize final session
  persistence before publishing idle.
- [x] Setup documentation, example image workflow, offline tests and builds.

These implementations retain explicit limits: the repository map is not a
multi-language tree-sitter/PageRank graph; Windows GPU monitoring is unavailable;
media needs a working installed workflow/model; video/music workflows are
user-supplied; training is not automatically assumed to improve quality;
artifact previews are not restored from saved sessions. No cloud media service
or automatic model promotion is installed.

## Verified here

- `go test -short -race ./...`: passed.
- `go vet ./...`: passed.
- `GOOS=windows GOARCH=amd64 go vet ./...`: passed.
- Linux build and Windows amd64 cross-build: passed.
- `node --check internal/webserver/static/app.js`: passed.
- Training dataset/token-masking tests: passed without ML dependencies.
- Real Czech eSpeak NG synthesis: produced a WAV, FFprobe duration 3.743447 s.
- CLI repository map and git summary: exercised on this checkout.
- ComfyUI queue/history/download and failure/cancellation paths: tested with
  an in-memory HTTP transport, not a running diffusion model.
- OpenAI-compatible streamed tool-call assembly: tested offline, not LM Studio.

## Still blocked / not runtime-verified

- [ ] Live Ollama tier campaign and semantic ranking. Socket creation is denied
  (`Operation not permitted`), including localhost. The evaluation preflight
  wrote a blocked report at `/tmp/offcode-eval-20261009-ollama/report.json`.
- [ ] Live LM Studio campaign. Same socket restriction; blocked report at
  `/tmp/offcode-eval-20261009-lmstudio/report.json`. A real installed model tag
  must replace the placeholder used for the connectivity-only probe.
- [ ] Real image/video/music generation. No `~/.offcode/media.json` is configured;
  installed workflows/checkpoints and a reachable ComfyUI server are required.
- [ ] Real speech transcription. FFmpeg is installed, but `whisper-cli` and
  `OFFCODE_WHISPER_MODEL` are missing.
- [ ] Actual PDF/DOCX conversions. Pandoc and the default PDF engine XeLaTeX
  are missing from PATH.
- [ ] Actual LoRA training and model import/evaluation. PyTorch, Transformers and
  PEFT are missing from the active Python environment; suitable local base
  weights and reviewed real examples have not been selected for a run. No personal adapter has been trained or activated.
- [ ] Windows runtime and UI behavior. Cross-build/static checks passed; no
  Windows runtime or CI execution was available in this session.
- [ ] Chromium/MCP integration tests and browser microphone/visual testing.
  External processes/network/browser sandbox restrictions remain.

Setup: [extensions](EXTENSIONS.md), [training](TRAINING.md),
[evaluation](EVALUATION.md). Run `bin/offcode doctor -extensions` in the target
runtime to see the concrete dependency/service status. Completion of every live
validation checkbox requires those external prerequisites; the overall project
must not yet be described as fully deployed and verified.
