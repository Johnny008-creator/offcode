# Local data and network behavior

Offcode defaults to local inference through Ollama or LM Studio. Requests
include conversation content, selected file contents and tool results. A custom
backend address sends that information to the configured server instead.

User data is stored under `~/.offcode/` (the user profile on Windows), including
preferences, saved sessions, persistent memory, usage records and optional
extension/training configuration. Project `.offcode/` directories may contain
hooks and semantic indexes with source excerpts. These files are not suitable
for public commits. Browser theme preferences are stored in localStorage.
Deleting a checkout does not delete the user-level data directory.

Network access can occur when:

- downloading models through Ollama or installing optional tools;
- checking/downloading releases from GitHub (released builds default to auto-update);
- searching through local SearXNG, whose configured engines contact search providers;
- fetching a URL or opening a page with browser tools;
- loading remote images referenced in rendered Markdown or content in previews;
- contacting configured MCP servers, backends or media services.

There is no analytics upload in the application. Local inference does not mean
all features work offline or that web content is private. To disable startup
update traffic, use `offcode setup -updates=off` or `OFFCODE_SKIP_UPDATE=1`.
Inspect logs and evaluation/training exports before sharing them; they can
contain real prompts, code, model output and filesystem paths. Remove local
credentials before attaching diagnostic output to an issue.
