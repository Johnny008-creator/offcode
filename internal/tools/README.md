# Tools

## read
Read the contents of a file at the given path. Requires a file path as input.

## write
Create or overwrite a file with the given content. Creates parent directories as needed. Requires a file path and content string.

## edit
Replace an exact, unique occurrence of old_string with new_string in a file. Requires a file path, old string, and new string.

## bash
Execute a shell command and return its combined stdout/stderr. Requires a command string.

## grep
Search for a regular expression pattern across files under a directory. Requires a pattern and optional path. Returns matching lines.

## glob
Find files under a directory whose relative path matches a glob pattern (e.g. *.go, cmd/*/main.go). Requires a pattern and optional path.

Each tool enables programmatic file manipulation and system interaction within the codebase.