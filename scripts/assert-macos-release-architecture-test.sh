#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"
mkdir -p "$bin"
touch "$tmp/test.dmg" "$tmp/payload"

cat >"$bin/hdiutil" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$KWU_TEST_HDIUTIL_LOG"
case "$1" in
attach)
	[ "${KWU_TEST_ATTACH_FAIL:-}" != 1 ] || exit 1
	while [ "$#" -gt 0 ]; do
		if [ "$1" = -mountpoint ]; then mount="$2"; break; fi
		shift
	done
	[ -n "${mount:-}" ] || exit 2
	mkdir -p "$mount/云智企办.app/Contents/MacOS"
	touch "$mount/云智企办.app/Contents/Info.plist"
	cp "$KWU_TEST_PAYLOAD" "$mount/云智企办.app/Contents/MacOS/clouvia-desktop"
	if [ "${KWU_TEST_DMG_MODE:-}" = legacy-binary ]; then
		cp "$KWU_TEST_PAYLOAD" "$mount/云智企办.app/Contents/MacOS/kwu-desktop"
	fi
	if [ "${KWU_TEST_DMG_MODE:-}" = duplicate ]; then
		mkdir -p "$mount/Second.app/Contents/MacOS"
		cp "$KWU_TEST_PAYLOAD" "$mount/Second.app/Contents/MacOS/clouvia-desktop"
	elif [ "${KWU_TEST_DMG_MODE:-}" = wrong-name ]; then
		mv "$mount/云智企办.app" "$mount/OldName.app"
	fi
	;;
detach) [ "${KWU_TEST_DETACH_FAIL:-}" != 1 ] ;;
*) exit 2 ;;
esac
EOF
cat >"$bin/lipo" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$KWU_TEST_ARCHES"
EOF
cat >"$bin/codesign" <<'EOF'
#!/usr/bin/env bash
[ "${KWU_TEST_CODESIGN_FAIL:-}" != 1 ]
EOF
cat >"$bin/PlistBuddy" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
key="${2#Print :}"
case "$key" in
CFBundleIdentifier) value=ai.clouvia.desktop ;;
CFBundleName|CFBundleDisplayName) value=云智企办 ;;
CFBundleExecutable) value=clouvia-desktop ;;
CFBundleShortVersionString|CFBundleVersion) value=1.0.3 ;;
*) exit 1 ;;
esac
if [ "${KWU_TEST_BAD_PLIST_KEY:-}" = "$key" ]; then
	value=wrong
fi
printf '%s\n' "$value"
EOF
chmod +x "$bin/hdiutil" "$bin/lipo" "$bin/codesign" "$bin/PlistBuddy"

export PATH="$bin:$PATH"
export PLIST_BUDDY="$bin/PlistBuddy"
export KWU_TEST_HDIUTIL_LOG="$tmp/hdiutil.log"
export KWU_TEST_PAYLOAD="$tmp/payload"
export KWU_TEST_ARCHES='x86_64 arm64'

"$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >"$tmp/success.log"
grep -F 'Verified macOS universal DMG identity and version 1.0.3: x86_64 arm64' "$tmp/success.log" >/dev/null
grep -F 'detach ' "$KWU_TEST_HDIUTIL_LOG" >/dev/null

export KWU_TEST_ARCHES='arm64'
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG missing x86_64 was accepted' >&2
	exit 1
fi
tail -n1 "$KWU_TEST_HDIUTIL_LOG" | grep -F 'detach ' >/dev/null

export KWU_TEST_ARCHES='x86_64'
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG missing arm64 was accepted' >&2
	exit 1
fi

export KWU_TEST_ARCHES='x86_64 arm64'
export KWU_TEST_ATTACH_FAIL=1
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG attach failure was ignored' >&2
	exit 1
fi
unset KWU_TEST_ATTACH_FAIL

export KWU_TEST_DETACH_FAIL=1
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG detach failure was ignored' >&2
	exit 1
fi
unset KWU_TEST_DETACH_FAIL

export KWU_TEST_CODESIGN_FAIL=1
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG with invalid code signature was accepted' >&2
	exit 1
fi
unset KWU_TEST_CODESIGN_FAIL

export KWU_TEST_DMG_MODE=duplicate
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG with multiple app bundles was accepted' >&2
	exit 1
fi
unset KWU_TEST_DMG_MODE

for key in CFBundleIdentifier CFBundleName CFBundleDisplayName CFBundleExecutable CFBundleShortVersionString CFBundleVersion; do
	export KWU_TEST_BAD_PLIST_KEY="$key"
	if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
		echo "DMG with invalid $key was accepted" >&2
		exit 1
	fi
done
unset KWU_TEST_BAD_PLIST_KEY

export KWU_TEST_DMG_MODE=wrong-name
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG with wrong app bundle name was accepted' >&2
	exit 1
fi

export KWU_TEST_DMG_MODE=legacy-binary
if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" v1.0.3 >/dev/null 2>&1; then
	echo 'DMG containing legacy executable was accepted' >&2
	exit 1
fi
unset KWU_TEST_DMG_MODE

if "$ROOT/scripts/assert-macos-release-architecture.sh" "$tmp/test.dmg" invalid-version >/dev/null 2>&1; then
	echo 'nonnumeric expected version was accepted' >&2
	exit 1
fi

echo 'macOS universal DMG identity, version, architecture, and code-signature tests passed'
