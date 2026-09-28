#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/repo/scripts" "$tmp/repo/desktop" "$tmp/bin"
cp "$ROOT/scripts/desktop-build.sh" "$tmp/repo/scripts/desktop-build.sh"
printf '%s\n' '{"info":{"productVersion":"0.0.0"},"sentinel":"keep exact bytes"}' >"$tmp/repo/desktop/wails.json"
cp "$tmp/repo/desktop/wails.json" "$tmp/original-wails.json"

cat >"$tmp/bin/node" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '{"info":{"productVersion":"9.9.9"},"sentinel":"mutated"}' >wails.json
EOF

cat >"$tmp/bin/wails" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$MOCK_WAILS_LOG"
if [ "${MOCK_WAILS_FAIL:-}" = "1" ]; then
	exit 23
fi
mkdir -p build/bin
printf 'desktop binary' >build/bin/kwu-desktop
EOF

cat >"$tmp/bin/nfpm" <<'EOF'
#!/usr/bin/env bash
target=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--target" ]; then
		target="$2"
		break
	fi
	shift
done
[ -n "$target" ] || exit 2
printf 'deb package' >"$target"
EOF
chmod +x "$tmp/bin/node" "$tmp/bin/wails" "$tmp/bin/nfpm" "$tmp/repo/scripts/desktop-build.sh"
export MOCK_WAILS_LOG="$tmp/wails.log"

if PATH="$tmp/bin:$PATH" MOCK_WAILS_FAIL=1 "$tmp/repo/scripts/desktop-build.sh" linux/amd64 v9.9.9 test >/dev/null 2>&1; then
	echo "expected mocked build failure" >&2
	exit 1
fi
cmp "$tmp/original-wails.json" "$tmp/repo/desktop/wails.json"

PATH="$tmp/bin:$PATH" "$tmp/repo/scripts/desktop-build.sh" linux/amd64 v9.9.9 test >/dev/null
cmp "$tmp/original-wails.json" "$tmp/repo/desktop/wails.json"
if grep -F -- '-o clouvia-desktop' "$MOCK_WAILS_LOG" >/dev/null; then
	echo 'Linux build unexpectedly received the macOS executable override' >&2
	exit 1
fi

grep -F '[ "$os" = darwin ] && build_args+=(-o "$MAC_BINNAME")' "$ROOT/scripts/desktop-build.sh" >/dev/null
grep -F 'cp -R "build/bin/${BINNAME}.app" "$app"' "$ROOT/scripts/desktop-build.sh" >/dev/null
grep -F 'Set :CFBundleExecutable $MAC_BINNAME' "$ROOT/scripts/desktop-build.sh" >/dev/null

echo "desktop-build preserves Linux identity and restores wails.json after failed and successful builds"
