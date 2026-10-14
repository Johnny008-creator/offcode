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

1. **IN PROGRESS: web UI.** Mobile layout, session switching across tabs,
   disconnected/error behavior, confirmation/questions, drafts, previews, voice.
   Add repeatable browser regression coverage and fix confirmed defects.
2. Live semantic search with a local embedding model.
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

Initial inspection: frontend has no disconnected-state UI and clears the
composer before send succeeds; async voice/artifact results can outlive a
session change. These are candidates to reproduce, not verified fixes yet.
Next: establish isolated browser tests against the real HTTP server and
controlled backend, reproduce mobile/multi-tab/error cases, then fix.
