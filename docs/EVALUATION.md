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
