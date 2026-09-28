#!/usr/bin/env bash
set -euo pipefail

image="${1:?usage: assert-macos-release-architecture.sh <universal.dmg> <version>}"
version="${2:?usage: assert-macos-release-architecture.sh <universal.dmg> <version>}"
[ -f "$image" ] || { echo "DMG does not exist: $image" >&2; exit 1; }

numeric_version="${version#v}"
numeric_version="${numeric_version%%-*}"
case "$numeric_version" in
	''|*[!0-9.]*|.*|*..*|*.) echo "expected version must resolve to a numeric dotted version: $version" >&2; exit 1 ;;
esac

plist_buddy="${PLIST_BUDDY:-/usr/libexec/PlistBuddy}"
[ -x "$plist_buddy" ] || { echo "required macOS verification tool is missing: $plist_buddy" >&2; exit 1; }
for tool in hdiutil lipo codesign; do
	command -v "$tool" >/dev/null || { echo "required macOS verification tool is missing: $tool" >&2; exit 1; }
done

mount=$(mktemp -d)
attached=false
cleanup() {
	status=$?
	trap - EXIT
	if [ "$attached" = true ] && ! hdiutil detach "$mount" >/dev/null; then
		echo "failed to detach DMG mount: $mount" >&2
		status=1
	fi
	rm -rf "$mount"
	exit "$status"
}
trap cleanup EXIT

hdiutil attach -readonly -nobrowse -mountpoint "$mount" "$image" >/dev/null
attached=true

apps=("$mount"/*.app)
if [ "${#apps[@]}" -ne 1 ] || [ ! -d "${apps[0]}" ]; then
	echo "DMG must contain exactly one top-level .app bundle" >&2
	exit 1
fi
app="${apps[0]}"
[ "$(basename "$app")" = "云智企办.app" ] || { echo "DMG app bundle must be named 云智企办.app" >&2; exit 1; }

plist="$app/Contents/Info.plist"
[ -f "$plist" ] || { echo "DMG app does not contain Contents/Info.plist" >&2; exit 1; }
assert_plist_value() {
	key="$1"
	expected="$2"
	actual=$("$plist_buddy" -c "Print :$key" "$plist") || {
		echo "DMG app Info.plist is missing $key" >&2
		exit 1
	}
	[ "$actual" = "$expected" ] || {
		echo "DMG app Info.plist $key is '$actual', expected '$expected'" >&2
		exit 1
	}
}
assert_plist_value CFBundleIdentifier ai.clouvia.desktop
assert_plist_value CFBundleName 云智企办
assert_plist_value CFBundleDisplayName 云智企办
assert_plist_value CFBundleExecutable clouvia-desktop
assert_plist_value CFBundleShortVersionString "$numeric_version"
assert_plist_value CFBundleVersion "$numeric_version"

binary="$app/Contents/MacOS/clouvia-desktop"
[ -f "$binary" ] || { echo "DMG does not contain Contents/MacOS/clouvia-desktop" >&2; exit 1; }
[ ! -e "$app/Contents/MacOS/kwu-desktop" ] || { echo "DMG must not contain Contents/MacOS/kwu-desktop" >&2; exit 1; }

arches=$(lipo -archs "$binary")
case " $arches " in *" x86_64 "*) ;; *) echo "DMG app is missing x86_64" >&2; exit 1 ;; esac
case " $arches " in *" arm64 "*) ;; *) echo "DMG app is missing arm64" >&2; exit 1 ;; esac
codesign --verify --deep --strict "$app"

echo "Verified macOS universal DMG identity and version $numeric_version: x86_64 arm64 ($image)"
