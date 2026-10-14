# Contributing

Offcode is an experimental local coding agent for Linux and Windows. Start with
[README.md](README.md) and [docs/ROADMAP.md](docs/ROADMAP.md). Keep changes focused;
explain the user-visible problem, the fix, and what you actually tested.

## Development

Install Go 1.27.2 or newer, Git, and Python 3 for the training utility tests.
Clone the repository and run:

```sh
go mod download
go test -short -race ./...
go vet ./...
go build -o bin/offcode ./cmd/offcode
python3 -m unittest discover -s scripts/training -p 'test_*.py'
```

Format changed Go files with `gofmt`. Run `go mod tidy` when changing imports or
dependencies. JavaScript is embedded directly; no frontend build is required.
Use Node to syntax-check files in `internal/webserver/static/js/`.
The offline suite does not need models or API credentials. Integration tests
may launch Chromium, download an MCP server with npx, or call local Ollama;
run them separately and report skips honestly. Windows installer changes also
need the native Windows CI job, which compiles Inno Setup and exercises setup.

## Reports and pull requests

Include your OS, `offcode version`, backend/model, reproduction steps, expected
behavior and actual behavior. Remove prompts, source code, home paths, tokens
and private server addresses from logs before posting. Security issues belong
in the private channel described in [SECURITY.md](SECURITY.md).

Add regression coverage for behavioral fixes. Keep offline tests isolated from
real home directories, services and secrets; use temporary directories and
portable JSON/path handling. Do not commit `.offcode/`, `.env`, model weights,
conversation exports, evaluation results or credentials. Example configuration
must use placeholders. Do not run project hooks from untrusted repositories.

See [LICENSING.md](LICENSING.md) before contributing. Do not submit code or
assets you are not entitled to share. Preserve dependency copyright notices.
