# Repeatable model/backend evaluation

Build offcode, start the chosen backend, and run the ten-task campaign:

```sh
python3 scripts/eval/run.py --output /tmp/offcode-campaign
python3 scripts/eval/run.py --backend lmstudio --models YOUR_LOADED_MODEL \
  --output /tmp/offcode-lmstudio-campaign
```

The default covers all **five** current tiers (including Foundry). `--models`
can select a subset; `--base-url` overrides the backend endpoint. Output must
be a new directory. Each case runs the real web agent engine in a fresh child
process, working directory and HOME/USERPROFILE, and saves the transcript,
files, errors and observations. The model cannot read earlier test cases
through conversation history. An unreachable backend produces a `blocked`
report and nonzero exit immediately, not fifty misleading failed scores.

File creation/edit approvals are restricted to local paths. Shell approval is
limited to `ls -lS`, `ls -laS`, `ls -S`, or `dir /o-s`. Other mutations are denied.
This is not an operating-system sandbox: read tools and other built-in
capabilities still run under the process account. Use a disposable OS/container
sandbox when evaluating untrusted models. No global user hooks/MCP servers or
memory are loaded because HOME is isolated.

The runner observes completion/errors/asking/timeout, checks expected files,
and checks exact contents for the simple text tasks. It does **not** turn the
existence of HTML into a claim of good design or a working calculator. Review
visual quality, functional JavaScript, semantic correctness, clarification
behavior and repeated calls in the saved artifacts/transcripts. The ask-user
case stops at the question rather than picking an answer for the user.

`go test -short -race ./...` checks offline behavior, including OpenAI-compatible
streamed tool-call assembly, tool integration, queue/history/download protocol,
index invalidation, cancellation and output preservation. These tests are not
a substitute for running the real Ollama/LM Studio/media services.

## Plan reliability

`scripts/eval/plan_reliability.py` uses the same isolated `eval-case` engine.
Its default `--suite short` measures four short tasks without an explicit
planning request. `--suite long` requests a plan and step updates for two
longer file workflows; `--suite all` runs both. For example:

```sh
python3 scripts/eval/plan_reliability.py --suite long \
  --models qwen3:8b gemma4:12b --output /tmp/offcode-plan-long
```

Each task runs twice by default. Reports record file checks, tool errors,
plan creation/update calls, final plan status and engine reminders. The long
suite checks exact CSV/JSON values and required documentation references.
Read the transcript to distinguish updates during work from updates made only
after a reminder; a completed plan alone does not prove the work is correct.
Short tasks that never create a plan do not measure plan-update reliability.
