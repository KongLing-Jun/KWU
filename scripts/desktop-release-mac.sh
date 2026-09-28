#!/usr/bin/env bash
set -euo pipefail

VERSION=""
SCRIPT_ROOT="$(cd "$(dirname "$0")" && pwd)"
REPOSITORY_ROOT="$(cd "$SCRIPT_ROOT/.." && pwd)"
RELEASE_WORK_ROOT="${HOME}/suiyuan-release-work"
RELEASE_REPOSITORY="xiaoxiaotest9527-bit/KWU-release"
DRY_RUN=false
ALLOW_NON_MAIN_V3=false
EMERGENCY_REASON=""

usage() {
	echo "usage: desktop-release-mac.sh --version vX.Y.Z [--repository-root PATH] [--release-work-root PATH] [--release-repository OWNER/REPO] [--allow-non-main-v3 --emergency-reason TEXT] [--dry-run]" >&2
	exit 2
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version) VERSION="${2:-}"; shift 2 ;;
	--repository-root) REPOSITORY_ROOT="${2:-}"; shift 2 ;;
	--release-work-root) RELEASE_WORK_ROOT="${2:-}"; shift 2 ;;
	--release-repository) RELEASE_REPOSITORY="${2:-}"; shift 2 ;;
	--allow-non-main-v3) ALLOW_NON_MAIN_V3=true; shift ;;
	--emergency-reason) EMERGENCY_REASON="${2:-}"; shift 2 ;;
	--dry-run) DRY_RUN=true; shift ;;
	*) usage ;;
	esac
done

VERSION="${VERSION#v}"
if [[ ! "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
	echo "stable release version must be vX.Y.Z" >&2
	exit 1
fi
IFS=. read -r major minor patch <<<"$VERSION"
for segment in "$major" "$minor" "$patch"; do
	if [ "$segment" -gt 999 ]; then
		echo "version segments must be between 0 and 999" >&2
		exit 1
	fi
done
if [ "$major" -eq 0 ] && [ "$minor" -eq 0 ] && [ "$patch" -eq 0 ]; then
	echo "v0.0.0 cannot produce a positive version_code" >&2
	exit 1
fi
VERSION="v$VERSION"
SOURCE_TAG="desktop-$VERSION"
LEGACY_SOURCE_TAG="manual-desktop-$VERSION"
RELEASE_TAG="desktop-$VERSION"

for tool in git gh; do
	command -v "$tool" >/dev/null || { echo "required tool is missing: $tool" >&2; exit 1; }
done

resolve_annotated_tag() {
	local tag="$1" lines direct peeled
	lines=$(git -C "$REPOSITORY_ROOT" ls-remote origin "refs/tags/$tag" "refs/tags/$tag^{}")
	direct=$(printf '%s\n' "$lines" | awk -v ref="refs/tags/$tag" '$2 == ref {print $1; exit}')
	peeled=$(printf '%s\n' "$lines" | awk -v ref="refs/tags/${tag}^{}" '$2 == ref {print $1; exit}')
	if [ -z "$direct" ]; then
		return 1
	fi
	if [ -z "$peeled" ]; then
		echo "remote source tag must be annotated: $tag" >&2
		return 2
	fi
	printf '%s %s\n' "$direct" "$peeled"
}

assert_remote_tag_unchanged() {
	local current current_object current_commit
	if ! current=$(resolve_annotated_tag "$SOURCE_TAG"); then
		echo "remote source tag disappeared during release: $SOURCE_TAG" >&2
		return 1
	fi
	read -r current_object current_commit <<<"$current"
	if [ "$current_object" != "$source_tag_object" ] || [ "$current_commit" != "$source_commit" ]; then
		echo "remote source tag moved during release: $SOURCE_TAG" >&2
		return 1
	fi
}

assert_worktree_pristine() {
	local path="$1" status
	status=$(git -C "$path" status --porcelain=v1 --untracked-files=all --ignored=matching)
	if [ -n "$status" ]; then
		echo "frozen macOS build worktree contains files outside the source commit; remove and recreate it: $path" >&2
		printf '%s\n' "$status" >&2
		return 1
	fi
}

assert_worktree_tracked_clean() {
	local path="$1" status
	status=$(git -C "$path" status --porcelain=v1 --untracked-files=no)
	if [ -n "$status" ]; then
		echo "macOS build changed tracked source files; refusing to publish frozen-commit artifacts: $path" >&2
		printf '%s\n' "$status" >&2
		return 1
	fi
}

standard_identity=''
standard_commit=''
standard_object=''
legacy_identity=''
legacy_commit=''
legacy_object=''
if standard_identity=$(resolve_annotated_tag "$SOURCE_TAG"); then
	read -r standard_object standard_commit <<<"$standard_identity"
else
	status=$?
	[ "$status" -eq 1 ] || exit "$status"
	standard_commit=''
fi
if [ -n "$standard_commit" ]; then
	source_commit="$standard_commit"
else
	if legacy_identity=$(resolve_annotated_tag "$LEGACY_SOURCE_TAG"); then
		read -r legacy_object legacy_commit <<<"$legacy_identity"
	else
		status=$?
		[ "$status" -eq 1 ] || exit "$status"
		legacy_commit=''
	fi
	if [ -z "$legacy_commit" ]; then
		echo "remote annotated source tag does not exist: $SOURCE_TAG (legacy fallback: $LEGACY_SOURCE_TAG)" >&2
		exit 1
	fi
	SOURCE_TAG="$LEGACY_SOURCE_TAG"
	source_tag_object="$legacy_object"
	source_commit="$legacy_commit"
	echo "warning: using legacy source tag $SOURCE_TAG" >&2
fi
if [ -n "$standard_commit" ]; then
	source_tag_object="$standard_object"
fi

# Fetches below only populate the local object database and origin/main-v3; they
# never create or update a remote tag, including in DryRun mode.
git -C "$REPOSITORY_ROOT" fetch --no-tags origin "refs/tags/$SOURCE_TAG" >/dev/null
[ "$(git -C "$REPOSITORY_ROOT" cat-file -t "$source_tag_object")" = tag ] || {
	echo "remote source tag object is not annotated: $SOURCE_TAG" >&2
	exit 1
}
[ "$(git -C "$REPOSITORY_ROOT" rev-parse "$source_tag_object^{}")" = "$source_commit" ] || {
	echo "remote source tag object no longer peels to $source_commit" >&2
	exit 1
}
git -C "$REPOSITORY_ROOT" fetch --force --no-tags origin \
	'+refs/heads/main-v3:refs/remotes/origin/main-v3' >/dev/null
if git -C "$REPOSITORY_ROOT" merge-base --is-ancestor "$source_commit" origin/main-v3; then
	is_main_v3=true
else
	is_main_v3=false
fi

is_draft=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json isDraft --jq '.isDraft')
if [ "$is_draft" != "true" ]; then
	echo "GitHub release is not a draft: $RELEASE_TAG" >&2
	exit 1
fi
release_body=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json body --jq '.body')
case "$release_body" in
*"$SOURCE_TAG"*"$source_commit"*|*"$source_commit"*"$SOURCE_TAG"*) ;;
*) echo "Draft Release body does not match $SOURCE_TAG at $source_commit" >&2; exit 1 ;;
esac
if [ "$SOURCE_TAG" = "desktop-$VERSION" ]; then
	case "$release_body" in
	*"Source tag object: \`$source_tag_object\`"*) ;;
	*) echo "Draft Release body does not match annotated tag object $source_tag_object" >&2; exit 1 ;;
	esac
fi

if printf '%s\n' "$release_body" | grep -Fxq -- '- Emergency: `true`'; then
	body_emergency=true
else
	body_emergency=false
fi
if printf '%s\n' "$release_body" | grep -Fxq -- '- Emergency: `false`'; then
	body_normal=true
else
	body_normal=false
fi
if [ "$body_emergency" = true ] && [ "$body_normal" = true ]; then
	echo 'Draft Release body contains conflicting emergency flags' >&2
	exit 1
fi
if [ "$is_main_v3" = false ] || [ "$body_emergency" = true ]; then
	if [ "$ALLOW_NON_MAIN_V3" != true ]; then
		echo 'emergency release takeover requires --allow-non-main-v3 and --emergency-reason' >&2
		exit 1
	fi
	if [ -z "$EMERGENCY_REASON" ] || [ "${#EMERGENCY_REASON}" -gt 500 ] || [[ "$EMERGENCY_REASON" == *$'\n'* || "$EMERGENCY_REASON" == *$'\r'* ]]; then
		echo 'emergency reason must be a non-empty single line of at most 500 characters' >&2
		exit 1
	fi
	if ! printf '%s\n' "$release_body" | grep -Fxq -- "- Emergency reason: $EMERGENCY_REASON"; then
		echo 'Draft Release emergency reason does not match the approved takeover reason' >&2
		exit 1
	fi
else
	if [ "$ALLOW_NON_MAIN_V3" = true ] || [ -n "$EMERGENCY_REASON" ]; then
		echo 'normal main-v3 release takeover must not provide emergency acknowledgement parameters' >&2
		exit 1
	fi
	if [ "$SOURCE_TAG" = "desktop-$VERSION" ] && [ "$body_normal" != true ]; then
		echo 'Draft Release body is missing the explicit normal-release provenance flag' >&2
		exit 1
	fi
fi

assert_primary_assets_only() {
	local name
	while IFS= read -r name; do
		[ -z "$name" ] && continue
		case "$name" in
		Clouvia-windows-amd64-installer.exe|Clouvia-windows-arm64-installer.exe|Clouvia-darwin-universal.dmg) ;;
		*) echo "Draft contains an unexpected pre-finalization asset: $name" >&2; return 1 ;;
		esac
	done
}
gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json assets --jq '.assets[].name' | assert_primary_assets_only

if [ "$DRY_RUN" = true ]; then
	echo "DryRun macOS: $SOURCE_TAG ($source_commit)"
	echo "DryRun macOS: would build, verify, and upload Clouvia-darwin-universal.dmg"
	exit 0
fi

for tool in wails go node pnpm create-dmg hdiutil lipo codesign shasum; do
	command -v "$tool" >/dev/null || { echo "required macOS build tool is missing: $tool" >&2; exit 1; }
done

work_directory="$RELEASE_WORK_ROOT/$RELEASE_TAG"
source_directory="$work_directory/mac-source"
output_directory="$work_directory/mac"
mkdir -p "$work_directory" "$output_directory"
if [ -d "$source_directory" ]; then
	existing_commit=$(git -C "$source_directory" rev-parse HEAD)
	if [ "$existing_commit" != "$source_commit" ]; then
		echo "existing macOS build worktree is $existing_commit, expected $source_commit" >&2
		exit 1
	fi
else
	git -C "$REPOSITORY_ROOT" worktree add --detach "$source_directory" "$source_commit"
fi
assert_worktree_pristine "$source_directory"

(cd "$source_directory" && scripts/desktop-build.sh darwin/universal "$VERSION" stable)
assert_worktree_tracked_clean "$source_directory"
dmg="$source_directory/dist/Clouvia-darwin-universal.dmg"
[ -f "$dmg" ] || { echo "macOS build did not produce $dmg" >&2; exit 1; }

"$SCRIPT_ROOT/assert-macos-release-architecture.sh" "$dmg" "$VERSION"
assert_remote_tag_unchanged

output_dmg="$output_directory/Clouvia-darwin-universal.dmg"
cp "$dmg" "$output_dmg"
sha256_file() {
	shasum -a 256 "$1" | awk '{print tolower($1)}'
}

upload_draft_asset_safely() {
	local path="$1" name existing_dir existing_path
	name=$(basename "$path")
	if printf '%s\n' "$asset_names" | grep -Fx "$name" >/dev/null; then
		existing_dir=$(mktemp -d)
		if ! gh release download "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --pattern "$name" --dir "$existing_dir"; then
			rm -rf "$existing_dir"
			return 1
		fi
		existing_path="$existing_dir/$name"
		if [ ! -f "$existing_path" ]; then
			echo "existing Draft asset download is missing: $name" >&2
			rm -rf "$existing_dir"
			return 1
		fi
		if [ "$(sha256_file "$existing_path")" != "$(sha256_file "$path")" ]; then
			echo "existing Draft asset differs from local output; refusing to overwrite: $name" >&2
			rm -rf "$existing_dir"
			return 1
		fi
		rm -rf "$existing_dir"
		echo "reused matching Draft asset: $name"
		return
	fi
	assert_remote_tag_unchanged
	gh release upload "$RELEASE_TAG" "$path" --repo "$RELEASE_REPOSITORY"
}

asset_names=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json assets --jq '.assets[].name')
upload_draft_asset_safely "$output_dmg"
asset_names=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json assets --jq '.assets[].name')
printf '%s\n' "$asset_names" | assert_primary_assets_only
printf '%s\n' "$asset_names" | grep -Fx 'Clouvia-darwin-universal.dmg' >/dev/null || {
	echo "Draft Release is missing Clouvia-darwin-universal.dmg after upload" >&2
	exit 1
}

dmg_sha256=$(sha256_file "$output_dmg")
dmg_size=$(stat -f '%z' "$output_dmg")
metadata_prefix='- `Clouvia-darwin-universal.dmg`:'
metadata_line="${metadata_prefix} size=${dmg_size} sha256=\`${dmg_sha256}\` architecture=x86_64+arm64 payload"
release_body=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json body --jq '.body')
existing_metadata=$(printf '%s\n' "$release_body" | grep -F "$metadata_prefix" || true)
if [ -n "$existing_metadata" ] && [ "$existing_metadata" != "$metadata_line" ]; then
	echo 'Draft Release already has different provenance for Clouvia-darwin-universal.dmg' >&2
	exit 1
fi
if [ -z "$existing_metadata" ]; then
	assert_remote_tag_unchanged
	body_file=$(mktemp)
	printf '%s\n' "$release_body" | awk -v line="$metadata_line" '
		$0 == "(Pending manual builds.)" { next }
		$0 == "## Verified primary assets" { saw_assets=1 }
		$0 == "## 更新说明" && !inserted {
			if (!saw_assets) { print "## Verified primary assets"; print "" }
			print line; print ""; inserted=1
		}
		{ print }
		END { if (!inserted) exit 42 }
	' >"$body_file" || {
		status=$?
		rm -f "$body_file"
		echo "failed to update Draft Release asset provenance (status $status)" >&2
		exit 1
	}
	gh release edit "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --notes-file "$body_file"
	rm -f "$body_file"
fi
release_body=$(gh release view "$RELEASE_TAG" --repo "$RELEASE_REPOSITORY" --json body --jq '.body')
printf '%s\n' "$release_body" | grep -Fx "$metadata_line" >/dev/null || {
	echo 'Draft Release did not retain verified macOS asset provenance' >&2
	exit 1
}

echo "Built, verified, and staged $output_dmg"
