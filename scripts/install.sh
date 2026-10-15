#!/bin/sh
# Install offcode on Linux from GitHub Releases:
#   curl -fsSL https://raw.githubusercontent.com/Johnny008-creator/offcode/master/scripts/install.sh | sh
# Downloads offcode-linux-amd64, verifies it against the release SHA256SUMS,
# installs it to ~/.local/bin (override with OFFCODE_INSTALL_DIR) and runs
# `offcode setup`. OFFCODE_VERSION=v0.3.0 pins a release instead of latest.
# `sh install.sh --uninstall` removes the binary (settings and models stay).
set -eu

repo=https://github.com/Johnny008-creator/offcode
dir=${OFFCODE_INSTALL_DIR:-$HOME/.local/bin}
exe=$dir/offcode

if [ "${1:-}" = "--uninstall" ]; then
	rm -f "$exe" "$exe.previous" "$exe.update-lock"
	echo "Removed $exe"
	exit 0
fi

case "$(uname -s)/$(uname -m)" in
Linux/x86_64 | Linux/amd64) ;;
*)
	echo "offcode releases are built for Linux x86_64 only; build from source instead." >&2
	exit 1
	;;
esac

if [ -n "${OFFCODE_VERSION:-}" ]; then
	base=$repo/releases/download/$OFFCODE_VERSION
else
	base=$repo/releases/latest/download
fi

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL --retry 2 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	echo "Need curl or wget to download offcode." >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading $base/offcode-linux-amd64"
fetch "$base/offcode-linux-amd64" "$tmp/offcode-linux-amd64"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"
(cd "$tmp" && grep ' offcode-linux-amd64$' SHA256SUMS | sha256sum -c -) || {
	echo "Checksum verification failed; nothing was installed." >&2
	exit 1
}

mkdir -p "$dir"
chmod +x "$tmp/offcode-linux-amd64"
mv -f "$tmp/offcode-linux-amd64" "$exe"
echo "offcode installed to $exe"

# Piped installs have no stdin terminal; ask setup questions on the real one.
if (: </dev/tty) 2>/dev/null; then
	"$exe" setup -interactive </dev/tty >/dev/tty 2>&1 ||
		echo "Setup failed; rerun 'offcode setup -interactive' to retry." >&2
else
	"$exe" setup || echo "Setup failed; rerun 'offcode setup' to retry." >&2
fi

case ":$PATH:" in
*":$dir:"*) echo "Run 'offcode' to get started." ;;
*) echo "Add $dir to your PATH (e.g. in ~/.bashrc), then run 'offcode'." ;;
esac
