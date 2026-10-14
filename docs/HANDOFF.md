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
2. **NEXT: live semantic search** with a local embedding model.
3. Document export and speech transcription/synthesis with real local tools.
4. Small-model LM Studio trial if available; record concrete blockers otherwise.
5. Model/tool/plan reliability campaign, bounded by available hardware.
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
