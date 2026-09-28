#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

repo="$tmp/repo"
origin="$tmp/origin.git"
bin="$tmp/bin"
mkdir -p "$repo" "$bin"
git init -b main-v3 "$repo" >/dev/null
git -C "$repo" config user.name 'KWU Test'
git -C "$repo" config user.email 'kwu-test@example.invalid'
printf 'fixture\n' >"$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -m fixture >/dev/null
git init --bare "$origin" >/dev/null
git -C "$repo" remote add origin "$origin"
git -C "$repo" push -u origin main-v3 >/dev/null
commit=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" tag -a desktop-v1.0.11 "$commit" -m 'release test tag'
git -C "$repo" tag -a manual-desktop-v1.0.12 "$commit" -m 'legacy release test tag'
git -C "$repo" push origin refs/tags/desktop-v1.0.11 refs/tags/manual-desktop-v1.0.12 >/dev/null
git -C "$repo" switch -c emergency >/dev/null
printf 'emergency fixture\n' >"$repo/EMERGENCY.md"
git -C "$repo" add EMERGENCY.md
git -C "$repo" commit -m 'emergency fixture' >/dev/null
emergency_commit=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" tag -a desktop-v1.0.13 "$emergency_commit" -m 'emergency release test tag'
git -C "$repo" push origin refs/tags/desktop-v1.0.13 >/dev/null
git -C "$repo" switch main-v3 >/dev/null

cat >"$bin/gh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$GH_TEST_LOG"
case "$*" in
*"--json isDraft"*) printf 'true\n' ;;
*"--json body"*)
	case "$*" in
	*desktop-v1.0.12*) printf '%s\n' '- Source tag: `manual-desktop-v1.0.12`' "- Source commit: \`$GH_TEST_COMMIT\`" ;;
	*desktop-v1.0.13*) printf '%s\n' '- Source tag: `desktop-v1.0.13`' "- Source tag object: \`$GH_TEST_EMERGENCY_TAG_OBJECT\`" "- Source commit: \`$GH_TEST_EMERGENCY_COMMIT\`" '- Emergency: `true`' '- Emergency reason: urgent test fix' ;;
	*) printf '%s\n' '- Source tag: `desktop-v1.0.11`' "- Source tag object: \`$GH_TEST_TAG_OBJECT\`" "- Source commit: \`$GH_TEST_COMMIT\`" '- Emergency: `false`' ;;
	esac
	;;
*"--json assets"*) ;;
*) echo "unexpected gh command: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$bin/gh"

work_root="$tmp/release-work"
export GH_TEST_LOG="$tmp/gh.log"
export GH_TEST_COMMIT="$commit"
export GH_TEST_TAG_OBJECT="$(git -C "$repo" rev-parse refs/tags/desktop-v1.0.11)"
export GH_TEST_EMERGENCY_COMMIT="$emergency_commit"
export GH_TEST_EMERGENCY_TAG_OBJECT="$(git -C "$repo" rev-parse refs/tags/desktop-v1.0.13)"
remote_tags_before=$(git -C "$repo" ls-remote --tags origin)
PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" \
	--version v1.0.11 \
	--repository-root "$repo" \
	--release-work-root "$work_root" \
	--release-repository example/release \
	--dry-run >"$tmp/output.log"

grep -F 'DryRun macOS' "$tmp/output.log" >/dev/null
[ ! -e "$work_root" ] || { echo "macOS DryRun created a work directory" >&2; exit 1; }
if grep -F 'release upload' "$GH_TEST_LOG" >/dev/null; then
	echo "macOS DryRun uploaded an asset" >&2
	exit 1
fi

PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" \
	--version v1.0.12 \
	--repository-root "$repo" \
	--release-work-root "$work_root" \
	--release-repository example/release \
	--dry-run >"$tmp/legacy-output.log" 2>"$tmp/legacy-error.log"
grep -F 'manual-desktop-v1.0.12' "$tmp/legacy-output.log" >/dev/null
grep -F 'using legacy source tag' "$tmp/legacy-error.log" >/dev/null

if PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" \
	--version v1.0.13 \
	--repository-root "$repo" \
	--release-work-root "$work_root" \
	--release-repository example/release \
	--dry-run >"$tmp/emergency-rejected.log" 2>&1; then
	echo 'macOS release script accepted an emergency Draft without acknowledgement' >&2
	exit 1
fi
grep -F 'requires --allow-non-main-v3 and --emergency-reason' "$tmp/emergency-rejected.log" >/dev/null
PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" \
	--version v1.0.13 \
	--repository-root "$repo" \
	--release-work-root "$work_root" \
	--release-repository example/release \
	--allow-non-main-v3 \
	--emergency-reason 'urgent test fix' \
	--dry-run >"$tmp/emergency-output.log"
grep -F 'DryRun macOS' "$tmp/emergency-output.log" >/dev/null

if PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" --version v0.0.0 --repository-root "$repo" --dry-run >/dev/null 2>&1; then
	echo 'macOS release script accepted v0.0.0' >&2
	exit 1
fi
if PATH="$bin:$PATH" "$ROOT/scripts/desktop-release-mac.sh" --version v1.0.11-beta.1 --repository-root "$repo" --dry-run >/dev/null 2>&1; then
	echo 'macOS release script accepted a prerelease version' >&2
	exit 1
fi
if grep -F -- '--clobber' "$ROOT/scripts/desktop-release-mac.sh" >/dev/null; then
	echo 'macOS release script still allows unconditional Draft asset overwrite' >&2
	exit 1
fi

mac_script="$ROOT/scripts/desktop-release-mac.sh"
pristine_line=$(grep -nF 'assert_worktree_pristine "$source_directory"' "$mac_script" | cut -d: -f1)
build_line=$(grep -nF 'scripts/desktop-build.sh darwin/universal' "$mac_script" | cut -d: -f1)
grep -F 'assert-macos-release-architecture.sh" "$dmg" "$VERSION"' "$mac_script" >/dev/null || {
	echo 'macOS release script does not bind DMG identity checks to the release version' >&2
	exit 1
}
tracked_line=$(grep -nF 'assert_worktree_tracked_clean "$source_directory"' "$mac_script" | cut -d: -f1)
upload_line=$(grep -nF 'upload_draft_asset_safely "$output_dmg"' "$mac_script" | cut -d: -f1)
tag_recheck_line=$(awk -v start="$build_line" 'NR > start && /^assert_remote_tag_unchanged$/ { print NR; exit }' "$mac_script")
if [ -z "$pristine_line" ] || [ -z "$tracked_line" ] || [ -z "$tag_recheck_line" ] || \
	[ "$pristine_line" -ge "$build_line" ] || [ "$tracked_line" -le "$build_line" ] || \
	[ "$tag_recheck_line" -le "$build_line" ] || [ "$tag_recheck_line" -ge "$upload_line" ]; then
	echo 'macOS release script does not bind a clean worktree and unchanged remote tag to the uploaded DMG' >&2
	exit 1
fi
if grep -E 'release (create|edit|upload)' "$GH_TEST_LOG" >/dev/null; then
	echo 'macOS DryRun performed a remote GitHub write' >&2
	exit 1
fi
[ "$(git -C "$repo" ls-remote --tags origin)" = "$remote_tags_before" ] || {
	echo 'macOS DryRun changed remote tags' >&2
	exit 1
}

echo "desktop-release-mac canonical/legacy/emergency DryRun tests passed"
