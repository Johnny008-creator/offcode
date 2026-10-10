# Optional local capabilities

All extensions run locally. They are available as tools in both the TUI and
web agent; operations which create files require the normal permission check.
They are denied in plan mode. Existing output files are never overwritten by
media, speech or document exports. No models/services are downloaded silently.

For explicit scripted use, `offcode tool NAME '{JSON}'` executes the named tool
directly, with the authority of the invoking shell command (no interactive
confirmation). The examples below use POSIX quoting. PowerShell users should
pass JSON as a single argument using their shell's native quoting rules.

## Documents

Install Pandoc and, for PDF, XeLaTeX (or a Pandoc-compatible engine selected by
`OFFCODE_PDF_ENGINE`). Keep the original Markdown report, then export:

```sh
offcode tool export_document '{"source":"report.md","path":"report.docx"}'
offcode tool export_document '{"source":"report.md","path":"report.pdf"}'
```

Rendering capabilities and supported PDF engines follow the
[Pandoc manual](https://pandoc.org/MANUAL.html). DOCX needs Pandoc but no PDF
engine. Missing executables return an error; a failed conversion does not
publish a partial output. Layout fidelity must be checked on real outputs.

## Images, video and music

Run ComfyUI separately with the models and nodes required by your chosen
workflow. Offcode submits an API-format graph to `/prompt`, polls
`/history/{prompt_id}`, then downloads the selected artifact from `/view`.
The flow follows the [upstream API example](https://github.com/comfyanonymous/ComfyUI/blob/master/script_examples/websockets_api_example.py).
No cloud fallback is used.

1. Make the workflow generate and save the desired media in ComfyUI itself.
2. Export the workflow in **API format**, not the visual editor format.
3. Store the workflow and configuration in `~/.offcode/`.
4. Set its prompt node/input and output node/key in `media.json`.

`examples/media/` contains a complete basic image workflow and matching
configuration. It expects `v1-5-pruned-emaonly.safetensors` in ComfyUI's
checkpoints directory; change `ckpt_name` to your compatible installed
checkpoint. This is an API example, not a downloaded model or a recommendation
that this checkpoint is the best choice for your hardware.

```sh
mkdir -p ~/.offcode
# Review/merge these files if you already have a media configuration.
cp examples/media/image.workflow.json ~/.offcode/
cp examples/media/media.json ~/.offcode/
offcode tool generate_image '{"prompt":"A quiet mountain lake at dawn","path":"lake.png"}'
```

Live-tested 2026-10-10 on the AUR `comfyui` package (`/opt/comfyui`, systemd
service) with the fp16 `v1-5-pruned-emaonly-fp16.safetensors` checkpoint
(~2.1GB — the full fp32 version is ~4.3GB, fine on more free disk) and
CPU-only PyTorch, `COMFYUI_GPU=cpu` at install time: a real 512x512 PNG came
back from `generate_image`. CPU-only because this machine's GPU (RX 6500 XT
/ Navi 24, gfx1034) has no official ROCm support — the same reason Ollama
runs it via Vulkan rather than ROCm. If your GPU *is* ROCm/CUDA-supported,
the package's auto-detection should pick that up at install time instead
(`COMFYUI_GPU=rocm`/`cuda`), but budget real disk space: the ROCm torch
wheel alone is ~6.2GB.

One gotcha regardless of backend: this AUR package's systemd service
(`comfyui.service`) doesn't pass a GPU-selection flag, and ComfyUI's own
startup code calls `torch.cuda.current_device()` unconditionally — on a
CPU-only torch install (no CUDA, no ROCm) that crashes with "Found no
NVIDIA driver" instead of falling back cleanly. Fix with a systemd
drop-in rather than editing the package-owned unit file:
```sh
sudo systemctl edit comfyui
# [Service]
# ExecStart=
# ExecStart=/usr/bin/comfyui --listen 127.0.0.1 --cpu
sudo systemctl restart comfyui
```

Add `video` and `music` entries to `workflows` using API exports which already
work in your ComfyUI installation. Each entry has the same shape:

```json
{
  "file": "my-video-api.json",
  "prompt_node": "12",
  "prompt_input": "text",
  "output_node": "40",
  "output_key": "gifs"
}
```

The node IDs above are illustrative: use your workflow's actual IDs.
`output_key` is the key in the selected node's **history output**, whose value
must be a list of `{filename, subfolder, type}` records. Standard saved images
use `images`; audio often uses `audio`; video nodes may use `gifs` or `videos`.
Read the real history output for custom nodes. Nodes returning only inline
bytes or local absolute paths require a save node before they can be used.

```sh
offcode tool generate_video '{"prompt":"A slow pan across a forest","path":"forest.mp4"}'
offcode tool generate_music '{"prompt":"Calm instrumental piano","path":"piano.flac"}'
```

The output extension must match the selected artifact. Offcode retrieves the
first file from the configured output node, limited to 512 MiB, and waits up
to 20 minutes. Model/checkpoint choice, duration, dimensions, sampler, seed and
other controls live in the workflow. Cancelling offcode stops local waiting;
an already submitted ComfyUI job may still run. Its job ID is in the error so
you can inspect/cancel it in ComfyUI. Offcode does not interrupt unrelated jobs
by calling ComfyUI's global interrupt endpoint.

The web UI previews generated images/audio/video and offers a download link.
Only files registered by successful artifact-producing tools in the current
session are exposed; the endpoint does not accept arbitrary file paths.
Previews are session-local and currently are not restored after a restart.

## Voice

Speech output uses eSpeak NG. It is lightweight synthesized speech, not voice
cloning. In the web UI, **read aloud** on an assistant response generates and
plays a WAV locally. The voice uses the browser's language. CLI example:

```sh
offcode tool synthesize_speech '{"text":"Ahoj světe","language":"cs","path":"hello.wav"}'
```

Speech input requires FFmpeg, `whisper-cli` from
[whisper.cpp](https://github.com/ggml-org/whisper.cpp), and a local model:

```sh
export OFFCODE_WHISPER_MODEL=/absolute/path/to/ggml-base.bin
offcode tool transcribe_audio '{"path":"recording.webm","language":"auto"}'
```

The web **record** button asks the browser for microphone access, records up
to two minutes, sends at most 25 MiB to the local server, and puts the
transcription in the composer for review before sending. No browser/cloud
speech recognition API is used. FFmpeg converts input to 16 kHz mono PCM WAV;
Whisper writes plain text. Microphone recording needs browser support and a
secure context (localhost normally qualifies).

## Repository tools

`repo_map` lists up to 200 files and Go function/type symbols with line
numbers; `git_summary` shows branch/status, staged and unstaged diff summaries,
and recent commits. Both are available in plan mode. This is a bounded Go-AST
map, not a multi-language tree-sitter/PageRank graph. Other languages are
listed by filename and remain searchable with grep or semantic search.

## Windows

Build natively with `go build -o bin/offcode.exe ./cmd/offcode`, or cross-build
with `GOOS=windows GOARCH=amd64 go build -o bin/offcode.exe ./cmd/offcode` on Linux.
CPU and memory use native Win32 APIs; vendor-specific GPU telemetry currently
shows unavailable. Shell tools/hooks use `cmd /C`, so commands must use Windows
syntax. Install Ollama or LM Studio for Windows; optional executables such as
Pandoc, FFmpeg, whisper-cli, espeak-ng and Chromium must be on PATH.

Configuration is under `%USERPROFILE%\.offcode`. In PowerShell set the model
with `$env:OFFCODE_WHISPER_MODEL = 'C:\models\ggml-base.bin'`. For SearXNG,
use a locally configured Docker Desktop/WSL deployment on port 8888; offcode's
web search endpoint remains `http://127.0.0.1:8888`. ComfyUI can run natively or
in WSL, with `media.json` pointing at its reachable localhost endpoint.

The CI workflow includes Linux and Windows build/test jobs. A successful
cross-build is not evidence of a real Windows runtime test. Win32 references:
[CPU](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getsystemtimes),
[memory](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-globalmemorystatusex).
