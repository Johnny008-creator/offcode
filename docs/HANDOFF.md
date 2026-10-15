# Development handoff

Updated: 2026-10-15. Read this file first when continuing development, then
ROADMAP.md and IMPLEMENTATION.md. Update this after each completed item and
before stopping. Keep private paths, transcripts and credentials out of Git.

## Current authorization and constraints

- Continue roadmap work autonomously, sequentially, starting with the web UI.
- Windows manual testing belongs to the owner; do not spend this campaign on it.
- Do not publish a release or create release tags.
- LM Studio is only a small-model trial; do not download/load a large model.
- Preserve user data and settings. Use isolated test homes/workspaces.
- Commit and push completed, validated chunks. Record real failures and skips;
  offline tests are not proof of live model/service quality.

## Queue

1. **DONE: web UI reliability pass.** Mobile layout, session switching across tabs,
   disconnected/error behavior, confirmation/questions, drafts, previews, voice.
   Add repeatable browser regression coverage and fix confirmed defects.
2. **DONE: live semantic search** with local nomic-embed-text.
3. **DONE: document export and voice** with real local tools.
4. **DONE: small-model LM Studio trial** (llmster 0.0.25, qwen3-1.7b, CPU).
5. **DONE (2026-10-15): model/tool/plan reliability campaign.** Results in
   "Completed: plan reliability" below; earlier paused instructions are historical.
6. Media workflow validation and training pipeline validation. Do not invent
   successful video/music generations or training-quality improvements.
7. Reconcile historical roadmap notes with verified results; final checks.

## Current checkpoint

Clean starting commit: ffa49f4. Public-repository preparation is complete;
Linux/Windows CI and security checks passed for the implementation commit
f0b9955. No new release. Own-code licensing decision remains open.

### Completed: web UI reliability

- Reproduced loss of composer text after rejected HTTP requests in Chromium.
- Preserve drafts on failed send; prevent duplicate pending sends and IME Enter
  submission. Show disconnected state and block submission until a fresh SSE state.
- Discard stale artifact, speech and transcription results after session changes;
  stop recording when switching conversations.
- Add custom text answers and an explicit Stop action to the question dialog.
- Browser regression test covers 360/390/768/1024/1440 widths, mobile sidebar,
  real two-tab session switching, HTTP rejection, custom answer and offline state.
- Browser test passes against the real HTTP/SSE server and a controlled backend;
  added to Linux CI. No live-model quality claim from this test.
- Next: start local Ollama if absent and run real semantic ranking tests.

Local environment discovery (not an installation request): previous validation
artifacts exist under `~/.local/share/offcode-validation/2026-10-11/`, including
Pandoc/Whisper tools, a small training model and adapter evaluation. Inspect and
revalidate those before claiming dependencies are absent or repeating downloads.
Do not commit their raw transcripts, machine paths or generated datasets.

### Completed: semantic search

`go test -v ./internal/semsearch -run 'TestSearchFindsSemanticMatch|TestSearchIncrementalReindex' -count=1`
passed both tests against real local Ollama/nomic-embed-text, no skips. No code
change was needed. Ollama was initially stopped and was started for validation.
It should be stopped at campaign end if no other user started relying on it.

### Completed: real documents and voice

Pandoc exported a Czech-text/table/code fixture to DOCX (10,827 bytes) and PDF
(9,373 bytes; WeasyPrint engine). Both reject overwrite without altering the
existing file. Real readback exposed missing DOCX table/link content: replaced
top-level-only XML mapping with bounded streaming body extraction, covered by
`TestExtractDOCXIncludesTablesLinksAndBreaks`. Both document readbacks now pass.
eSpeak produced a 185,670-byte WAV; whisper.cpp/base transcribed the sentence
successfully but misrecognized the product name. This is an integration pass,
not a claim of perfect speech recognition or a physical microphone test.
Evidence: local validation directory `2026-10-14/documents/validation.json`.

### Completed: LM Studio small-model trial

llmster 0.0.25 (headless LM Studio, CPU-only here) served qwen3-1.7b GGUF,
reused from the existing Ollama blob, at 16384 context, ~6 tok/s. Ten-task
`scripts/eval/run.py --backend lmstudio` campaign plus a rerun of tasks
6/7/10 after harness fixes. Evidence: local validation directories
`2026-10-14/lmstudio-campaign` and `lmstudio-campaign-rerun`.

Real defects found and fixed (commits 7b8f622, 9fb493a):
- LM Studio streams thinking as `reasoning_content`; only Ollama's `reasoning`
  was read, so the trace was silently dropped. Now both; reasoning blocks
  confirmed in live transcripts after the fix.
- Model list included LM Studio's embedding model; `ContextLength` returned 0.
  Both now use LM Studio `/api/v0/models` (type, `loaded_context_length`),
  with fallback to `/v1/models` for other OpenAI-compatible servers.
- `eval-case` denied writes given as absolute paths inside the case
  directory, failing tasks 6/7 for harness reasons. Fixed and tested.

Results (manual review): 1 tools listed; 2 hello.txt correct; 3 a/b/c correct
with a plan; 4 asked for a choice as requested; 5 SearXNG search summarized
correctly; 6 notes.html written (rerun); 7 config.json written, then repeated
edits with nonexistent old_string, overwrote it as non-JSON and falsely claimed
success (model quality, tools reported errors correctly); 8 a custom `find`
command was denied by the harness bash allowlist, so the model only described
the command; 9 three linked files written; 10 timed out at 300 s, rerun at 600 s
wrote calculator.html, but two buttons have broken quotes and `clearDisplay`
is undefined, so it does not fully work. Conclusion: the LM Studio path is
functional end-to-end; 1.7B quality is the limit, not the backend.

### Paused: plan reliability (item 5)

Paused on the owner's request mid-campaign. All work below is committed
locally on master; **not pushed** — the owner wants one push once the whole
batch is finished (do not push per chunk, overriding the line above).

Fixes found by this campaign (all with regression tests):
- 204709a: a model ending a round with empty content and no tool calls (no
  plan) ended the turn silently. Web engine and TUI now nudge once, then show
  "stopped without a final answer".
- 412ac99 (important): preserve mode is the default even without
  `~/.offcode/setup.json` and set `OFFCODE_NUM_CTX=0`, so no `num_ctx` was
  sent; Ollama used 4096 on the 8GB card and truncated offcode's ~4.3K-token
  prompt to 2050 (`truncating input prompt` in the Ollama log). Now the
  request context is omitted only when `OLLAMA_CONTEXT_LENGTH` is set or the
  model's Modelfile sets `num_ctx`. Live-verified: `n_ctx_slot = 8192`,
  `truncated = 0`. See DECISIONS.md 2026-10-14.

Tool: `scripts/eval/plan_reliability.py` (4 short multi-step tasks with
mechanical checks; records plan_set/plan_update use, nudges, unfinished
stops). Results so far, valid runs with the fixed context, 2 repeats each:

| model | checks passed | plan_set used | notes |
|---|---|---|---|
| qwen3:1.7b | 5/8 | 0/8 | notes: mangled absolute paths outside cwd (correctly denied) then looped; config: 1/2 |
| qwen3:8b | 8/8 | 0/8 | clean |
| gemma4:12b | not run | — | stopped during its first case |
| qwen3.5:4b | blocked | — | at 8192 context it reliably crashes the RX 6500 XT Vulkan driver (`vk::Queue::submit: ErrorDeviceLost`, kernel `amdgpu ring comp timeout`); qwen3 models are fine. Driver/hardware, not offcode |
| gpt-oss:20b | skipped | — | needs ~14.9GB of 15GB RAM; earlier driver crash |

Earlier runs in `plan-reliability-aborted` and `plan-reliability-truncated-ctx`
(local validation dir) predate these fixes and are not valid evidence.

Observation, not yet a decision: no model called plan_set on these 2–3 step
tasks, so plan_update reliability is still unmeasured. Gemma4:12b is the tier
with the known "does work but never marks steps done" pattern.

Resume:
1. `go build -o bin/offcode ./cmd/offcode`; start Ollama if it is not running.
2. `python3 scripts/eval/plan_reliability.py --models gemma4:12b --output <new dir>`
   (directory must not exist; keep it under the local validation dir).
3. Add one or two longer tasks (4+ steps) so models actually plan, rerun on
   qwen3:8b and gemma4:12b, then judge plan_update reliability.
4. Record results here, stop Ollama if you started it, commit.
LM Studio's llmster daemon (started by Codex) is still running with no model
loaded; `~/.lmstudio/bin/lms daemon down` stops it if unneeded.

### Continuation — 2026-10-15

- Resumed Gemma's short suite, but Vulkan failed during initial prompt processing
  with `ErrorDeviceLost`; the kernel recorded an amdgpu compute-ring timeout.
  Stopped that series. Its empty/error outcomes are not model-quality evidence.
- Started a temporary user-owned CPU-only Ollama process; no service/config
  changes. Stop this process after validation.
- Added two explicit four-step planning workflows (`--suite long`) with exact
  data checks. CPU run uses one repetition per model/task and a 900-second
  per-case limit. This is a small diagnostic sample, not a quality benchmark.
- First Gemma inventory case passed all output checks. It updated three steps
  during work, missed the final verification step, then marked it done after
  one engine reminder. All four steps finished without tool errors.
- Native streams ending without `done: true` now report unexpected EOF instead
  of finalizing partial answers/tool calls. Empty/text/tool-call regressions,
  full short race suite, Linux/Windows vet and builds passed.
- Real Chromium UI, rendered text/JPEG, and MCP filesystem/lifetime tests passed
  without skips. Previous LM Studio, semantic, document and voice results are
  now reflected in IMPLEMENTATION.md.

Local evidence: `2026-10-15/plan-short` (aborted GPU run), `plan-long-cpu`,
`plan-long-gpu-qwen`, `ollama-cpu.log`, `ollama-gpu-qwen.log` and
`kernel-vulkan.log` in the validation directory.

### Completed: plan reliability (item 5)

Long suite (`--suite long`, explicit four-step planning request, 900 s limit).
"Before/after" counts `plan_update` calls before and after the engine's
unfinished-plan reminder.

| run | case | files | plan_update before/after | finished plan |
|---|---|---|---|---|
| gemma4:12b CPU | inventory | pass | 3 / 1 | yes |
| gemma4:12b CPU | settings | pass | 5 / 0 | yes, no reminder |
| qwen3:8b CPU | inventory | fail: literal `\n` in CSV | 1 / 4 | yes, then timeout |
| qwen3:8b CPU | settings | incomplete | 0 / 0 | timeout mid-work |
| qwen3:8b GPU | inventory 1 | pass | 1 / 3 | yes |
| qwen3:8b GPU | inventory 2 | fail: README omits `inventory.csv` name | 4 / 1 | yes |
| qwen3:8b GPU | settings 1 | pass | 4 / 0 | 4/5, timeout on final step |
| qwen3:8b GPU | settings 2 | pass (1 recovered edit error) | 3 / 2 | yes |

qwen3:8b ran fully on the RX 6500 XT (37/37 layers, 8192 context, no
`ErrorDeviceLost`); Gemma's GPU run is still blocked by the Vulkan crash.

Conclusions: both models create a plan when asked and keep it roughly in sync,
but usually lag by a step and often close the last steps only after the
reminder, so the reminder remains necessary. No case ended with an unfinished
plan and no answer. Settings-1 found and fixed its own missed `debug` edit during
verification. qwen3:8b repeatedly double-escapes newlines (`\\n`) in write
content and then reports the file as verified — model quality, not tool bugs.

Fix from this campaign (2fc2006): out-of-order `plan_update` used to auto-promote
another step, leaving several `in_progress` with no hint about open earlier
steps (CPU qwen run). It now names the open steps. Live-confirmed in GPU
settings-2: the model marked step 2 done, got the hint, and immediately marked
step 1 done. Sample is small (1–2 runs per case); not a quality benchmark.
Ollama started for validation has been stopped.
