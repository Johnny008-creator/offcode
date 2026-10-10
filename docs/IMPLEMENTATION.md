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
- [x] Windows installer: Inno Setup GUI wizard (also runnable unattended from
  `cmd` via its own `/VERYSILENT` switch), a plain batch installer, and a
  PowerShell installer — all three install to `%LocalAppData%\Programs\offcode`
  and add it to the user PATH without admin rights, plus a tag-triggered
  release workflow that builds and attaches both the raw exe and the
  compiled installer. Added 2026-10-10.

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
- **Real image generation (2026-10-10).** Installed the AUR `comfyui`
  package (CPU-only PyTorch — this machine's GPU has no ROCm support),
  the fp16 SD1.5 checkpoint, and `~/.offcode/media.json` from
  `examples/media/`; `offcode tool generate_image '{"prompt":"a small
  red cube on a white background","path":"/tmp/offcode-test-image.png"}'`
  returned a real, valid 512x512 PNG matching the prompt. See
  docs/EXTENSIONS.md's "Images, video and music" section for the setup
  (including a `--cpu` systemd drop-in needed to stop ComfyUI crashing
  on a GPU-less torch install). Video/music remain unconfigured — no
  workflow files for those kinds.
- **Live 5-tier Ollama campaign (2026-10-10), in progress.** Ran
  `scripts/eval/run.py` against this machine's real Ollama (AMD RX
  6500 XT, Vulkan backend) across all five tiers. Found and fixed three
  real bugs this way (see docs/DECISIONS.md): extension tools offered
  without their dependencies configured, Ollama's native tool-call
  template parser erroring out the whole turn on malformed tags instead
  of being retried, and `write` silently producing an empty file when a
  tool call's `content` argument arrived under a corrupted JSON key.

## Still blocked / not runtime-verified

- [ ] Live LM Studio campaign. Never exercised against a real LM Studio
  instance — every live model test so far has been Ollama.
- [ ] Live semantic ranking (`internal/semsearch`). The 2026-10-09 socket
  restriction that blocked this no longer applies in a session with real
  network access (confirmed 2026-10-10 — Ollama, SearXNG and ComfyUI are
  all reachable here), but `semantic_search` specifically hasn't been
  exercised against a real embedding model yet, just offline/mocked.
- [ ] Real speech transcription. FFmpeg is installed, but `whisper-cli` and
  `OFFCODE_WHISPER_MODEL` are missing.
- [ ] Actual PDF/DOCX conversions. Pandoc and the default PDF engine XeLaTeX
  are missing from PATH.
- [ ] Actual LoRA training and model import/evaluation. PyTorch, Transformers and
  PEFT are missing from the active Python environment; suitable local base
  weights and reviewed real examples have not been selected for a run. No personal adapter has been trained or activated.
- [ ] Windows runtime and UI behavior. Cross-build/static checks passed; no
  Windows runtime or CI execution was available in this session.
- [ ] The three Windows installers (`scripts/windows/offcode.iss`/`.bat`/`.ps1`)
  and the `release.yml` Inno Setup build step. No `iscc`/`wine` is available in
  this sandbox, so none have actually been compiled or run — only
  hand-reviewed against Inno Setup/batch/PowerShell syntax rules.
- [ ] Chromium/MCP integration tests and browser microphone/visual testing.
  External processes/network/browser sandbox restrictions remain.

Setup: [extensions](EXTENSIONS.md), [training](TRAINING.md),
[evaluation](EVALUATION.md). Run `bin/offcode doctor -extensions` in the target
runtime to see the concrete dependency/service status. Completion of every live
validation checkbox requires those external prerequisites; the overall project
must not yet be described as fully deployed and verified.
