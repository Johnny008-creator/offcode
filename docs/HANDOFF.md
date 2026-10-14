# Development handoff

Updated: 2026-10-14. Read this file first when continuing development, then
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
5. **NEXT:** Model/tool/plan reliability campaign, bounded by available hardware.
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
