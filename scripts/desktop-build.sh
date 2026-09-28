#!/usr/bin/env bash
# Build and package the Wails desktop app for one platform. Wails cannot
# cross-compile a CGO+webview binary, so this runs on a native runner per target
# (see .github/workflows/release-desktop.yml) and is invoked once per matrix entry.
#
# Output lands in <repo>/dist/ with stable, platform-keyed names that
# desktop/cmd/sign's `manifest` subcommand maps back to update.PlatformKey:
#   macOS:   Clouvia-darwin-<arch>.zip                  (manual download archive)
#            Clouvia-darwin-universal.dmg               (drag-to-install; human download)
#   Windows: Clouvia-windows-<arch>-installer.exe       (NSIS per-user installer; updater channel)
#            Clouvia-windows-<arch>.zip                 (portable human download)
#   Linux:   Clouvia-linux-<arch>.tar.gz                (manual download archive)
#            Clouvia-linux-<arch>.deb                   (Debian/Ubuntu package; human download)
#
# Usage: scripts/desktop-build.sh <os/arch> <version> [channel] [reuse-frontend]
#   e.g. scripts/desktop-build.sh darwin/arm64 v1.1.0
#        scripts/desktop-build.sh darwin/arm64 v1.5.0-canary.20260608.42 canary
set -euo pipefail

PLATFORM="${1:?usage: desktop-build.sh <os/arch> <version> [channel]}"
VERSION="${2:?usage: desktop-build.sh <os/arch> <version> [channel]}"
CHANNEL="${3:-stable}"
BUILD_MODE="${4:-full}"

os="${PLATFORM%/*}"
arch="${PLATFORM#*/}"

case "$BUILD_MODE" in
full) ;;
reuse-frontend)
	[ "$os" = windows ] || {
		echo "reuse-frontend is supported only for a second Windows build" >&2
		exit 1
	}
	;;
*)
	echo "unsupported build mode: $BUILD_MODE" >&2
	exit 1
	;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARTIFACT_NAME="Clouvia"        # ASCII release artifact prefix (GitHub strips non-ASCII filename prefixes)
APP_BUNDLE_NAME="云智企办"     # app bundle / portable executable display name
BINNAME="kwu-desktop"     # wails.json outputfilename -> linux binary name
WINDOWS_BINNAME="clouvia-desktop"
MAC_BINNAME="clouvia-desktop"
MAC_BUNDLE_ID="ai.clouvia.desktop"

cd "$ROOT/desktop"

windows_signing=false
if [ "$os" = windows ] && [ "${KWU_WINDOWS_RELEASE_SIGNING:-}" = "1" ]; then
	windows_signing=true
	evidence_path="${KWU_WINDOWS_SIGN_EVIDENCE_PATH:-}"
	[ -n "$evidence_path" ] || {
		echo "KWU_WINDOWS_SIGN_EVIDENCE_PATH is required when release signing is enabled" >&2
		exit 1
	}
	[ ! -e "$evidence_path" ] || {
		echo "refusing to reuse existing Windows signing evidence: $evidence_path" >&2
		exit 1
	}
	mkdir -p "$(dirname "$evidence_path")"
	signing_helper_win=$(cygpath -w "$ROOT/scripts/desktop-windows-authenticode.ps1")
	export KWU_WINDOWS_SIGN_EVIDENCE_PATH
	KWU_WINDOWS_SIGN_EVIDENCE_PATH=$(cygpath -w "$evidence_path")
	pwsh.exe -NoLogo -NoProfile -File "$signing_helper_win" -Action Preflight
fi

# Wails reads version metadata from this tracked file. Preserve its exact
# pre-build contents so both successful builds and early failures leave the
# source worktree clean.
wails_config="wails.json"
wails_config_backup=$(mktemp)
cp "$wails_config" "$wails_config_backup"
restore_wails_config() {
	status=$?
	trap - EXIT
	if ! cp "$wails_config_backup" "$wails_config"; then
		echo "failed to restore $ROOT/desktop/$wails_config" >&2
		status=1
	fi
	rm -f "$wails_config_backup"
	exit "$status"
}
trap restore_wails_config EXIT

# Stamp the version resource (Windows file properties, macOS CFBundleVersion) from
# the tag. Wails feeds info.productVersion into goversioninfo and NSIS's
# VIFileVersion, both of which demand a strictly numeric X.X.X, so strip the
# leading "v" AND any prerelease suffix (a `-rc1` tag would otherwise abort the
# installer build). The full tag still rides in ldflags for the in-app version.
numver="${VERSION#v}"; numver="${numver%%-*}"
node -e 'const fs=require("fs"),f="wails.json",j=JSON.parse(fs.readFileSync(f,"utf8"));j.info.productVersion=process.argv[1];fs.writeFileSync(f,JSON.stringify(j,null,2)+"\n")' "$numver"

# NSIS installer is Windows-only (Wails requires a single windows target for -nsis).
ldflags="-X main.version=$VERSION -X main.channel=$CHANNEL"
# Keep the native webview inspector available in packaged production builds.
# Wails wires F12 to DevTools on Windows only when this build tag is enabled.
build_args=(-clean -devtools -platform "$PLATFORM" -ldflags "$ldflags")
[ "$BUILD_MODE" = reuse-frontend ] && build_args+=(-s -skipbindings -skipembedcreate)
[ "$os" = windows ] && build_args+=(-o "${WINDOWS_BINNAME}.exe" -nsis -webview2 embed)
[ "$os" = darwin ] && build_args+=(-o "$MAC_BINNAME")
# Link cgo against WebKitGTK 4.1: 4.0 (libwebkit2gtk-4.0.so.37) is gone on
# Ubuntu 24.04+/Fedora 40+, while 4.1 ships from Ubuntu 22.04 onward.
[ "$os" = linux ] && build_args+=(-tags webkit2_41)

if [ "$os" = windows ]; then
	python_runtime_prepare_win=$(cygpath -w "$ROOT/scripts/prepare-python-runtime.ps1")
	pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$python_runtime_prepare_win" \
		-Architecture "$arch"
fi

echo "==> wails build ${build_args[*]}"
wails build "${build_args[@]}"

mkdir -p "$ROOT/dist"

case "$os" in
darwin)
	# macOS has a release-only executable identity. Keep wails.json unchanged so
	# Linux and local development continue using kwu-desktop. Wails' -o flag
	# renames the executable inside the bundle, but the .app directory keeps the
	# wails.json application name.
	staging=$(mktemp -d)
	app="$staging/${APP_BUNDLE_NAME}.app"
	cp -R "build/bin/${BINNAME}.app" "$app"
	plist="$app/Contents/Info.plist"
	/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier $MAC_BUNDLE_ID" "$plist"
	/usr/libexec/PlistBuddy -c "Delete :CFBundleName" "$plist" 2>/dev/null || true
	/usr/libexec/PlistBuddy -c "Add :CFBundleName string $APP_BUNDLE_NAME" "$plist"
	/usr/libexec/PlistBuddy -c "Delete :CFBundleDisplayName" "$plist" 2>/dev/null || true
	/usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string $APP_BUNDLE_NAME" "$plist"
	/usr/libexec/PlistBuddy -c "Set :CFBundleExecutable $MAC_BINNAME" "$plist"

	# Two signing paths, selected by HAS_APPLE_CERT (set by release-desktop.yml when
	# the APPLE_* secrets are present). With a real Developer ID cert + notarization
	# key we sign with a hardened runtime, notarize, and staple — a downloaded build
	# then opens with no Gatekeeper prompt. Without it we ad-hoc sign as before (still
	# un-notarized; users clear the quarantine attribute per desktop/README.md). The
	# fallback keeps fork/local builds working with no secrets configured.
	if [ "${HAS_APPLE_CERT:-}" = "true" ]; then
		identity="$(security find-identity -v -p codesigning | awk -F'"' '/Developer ID Application/{print $2; exit}')"
		[ -n "$identity" ] || { echo "HAS_APPLE_CERT=true but no 'Developer ID Application' identity found in the keychain" >&2; exit 1; }
		echo "==> codesign (Developer ID): $identity"
		codesign --force --deep --timestamp --options runtime \
			--entitlements "$ROOT/desktop/build/darwin/entitlements.plist" \
			-s "$identity" "$app"
		# notarytool wants an archive, not a bare bundle: zip the .app, submit, wait,
		# then staple the ticket back onto the bundle so it verifies offline.
		ditto -c -k --keepParent "$app" "$staging/notarize.zip"
		echo "==> notarytool submit (app)"
		xcrun notarytool submit "$staging/notarize.zip" \
			--key "$APPLE_API_KEY_PATH" --key-id "$APPLE_API_KEY_ID" \
			--issuer "$APPLE_API_ISSUER_ID" --wait
		xcrun stapler staple "$app"
	else
		# Ad-hoc cuts the "is damaged" error somewhat but is NOT notarized; users may
		# still need `xattr -dr com.apple.quarantine` (see desktop/README.md).
		codesign --force --deep -s - "$app"
	fi

	if [ "$arch" = universal ]; then
		# One universal .app covers Intel + Apple Silicon; publish it under both
		# manifest keys so the updater's darwin-arm64/darwin-amd64 lookup finds it
		# (avoids a scarce macos-13 Intel runner).
		ditto -c -k --keepParent "$app" "$ROOT/dist/${ARTIFACT_NAME}-darwin-arm64.zip"
		ditto -c -k --keepParent "$app" "$ROOT/dist/${ARTIFACT_NAME}-darwin-amd64.zip"
	else
		ditto -c -k --keepParent "$app" "$ROOT/dist/${ARTIFACT_NAME}-darwin-${arch}.zip"
	fi
	# A drag-to-Applications .dmg for first-time human download. Named -universal so
	# cmd/sign's substring match (darwin-arm64/darwin-amd64) skips it: the .zip stays
	# the updater channel, the .dmg is release-page only. create-dmg can exit nonzero
	# while still writing the image, so gate on the file existing, not the exit code.
	dmgsrc=$(mktemp -d)
	cp -R "$app" "$dmgsrc/${APP_BUNDLE_NAME}.app"
	dmg="$ROOT/dist/${ARTIFACT_NAME}-darwin-universal.dmg"
	create-dmg \
		--volname "$APP_BUNDLE_NAME" \
		--window-size 540 380 \
		--icon-size 110 \
		--icon "${APP_BUNDLE_NAME}.app" 150 190 \
		--app-drop-link 390 190 \
		--no-internet-enable \
		"$dmg" "$dmgsrc" || true
	[ -f "$dmg" ] || { echo "create-dmg did not produce $dmg" >&2; exit 1; }
	# The .dmg is a separately-downloaded artifact, so sign + notarize + staple the
	# disk image itself too — the stapled .app inside isn't enough for the image.
	if [ "${HAS_APPLE_CERT:-}" = "true" ]; then
		codesign --force --timestamp -s "$identity" "$dmg"
		echo "==> notarytool submit (dmg)"
		xcrun notarytool submit "$dmg" \
			--key "$APPLE_API_KEY_PATH" --key-id "$APPLE_API_KEY_ID" \
			--issuer "$APPLE_API_ISSUER_ID" --wait
		xcrun stapler staple "$dmg"
	fi
	rm -rf "$staging" "$dmgsrc"
	;;
windows)
	# `wails build -nsis` writes the installer under build/bin; its exact name
	# varies. The NSIS launcher itself can be x86 even when its application
	# payload is ARM64, so validate the real Wails executable and the extracted
	# clouvia-desktop.exe payload instead of inspecting the installer PE header.
	shopt -s nullglob
	installers=(build/bin/*installer*.exe)
	shopt -u nullglob
	[ "${#installers[@]}" -eq 1 ] || {
		echo "expected exactly one NSIS installer in build/bin, found ${#installers[@]}" >&2
		exit 1
	}
	installer="${installers[0]}"
	portable="build/bin/${WINDOWS_BINNAME}.exe"
	[ -f "$portable" ] || { echo "no Windows application found at $portable" >&2; exit 1; }
	verifier_win=$(cygpath -w "$ROOT/scripts/assert-windows-release-architecture.ps1")
	portable_win=$(cygpath -w "$ROOT/desktop/$portable")
	installer_win=$(cygpath -w "$ROOT/desktop/$installer")
	verification_args=(
		-NoLogo -NoProfile -ExecutionPolicy Bypass -File "$verifier_win"
		-Architecture "$arch" -ApplicationPath "$portable_win" -InstallerPath "$installer_win" \
		-ExpectedVersion "$numver" -ExpectedProductName "$APP_BUNDLE_NAME" -ExpectedCompanyName "Clouvia"
	)
	if [ "$windows_signing" = true ]; then
		verification_args+=(
			-RequireSignature
			-ExpectedSignerThumbprint "$WINDOWS_SIGN_CERT_THUMBPRINT"
			-ExpectedSignerSubject "$WINDOWS_SIGN_CERT_SUBJECT"
			-SignToolPath "$WINDOWS_SIGNTOOL_PATH"
		)
	fi
	pwsh.exe "${verification_args[@]}"
	final_installer="$ROOT/dist/${ARTIFACT_NAME}-windows-${arch}-installer.exe"
	cp "$installer" "$final_installer"
	if [ "$windows_signing" = true ]; then
		final_installer_win=$(cygpath -w "$final_installer")
		pwsh.exe -NoLogo -NoProfile -File "$signing_helper_win" \
			-Action AssertEvidence -Architecture "$arch" \
			-EvidencePath "$KWU_WINDOWS_SIGN_EVIDENCE_PATH" -Path "$final_installer_win"
	fi
	portable_zip="$ROOT/dist/${ARTIFACT_NAME}-windows-${arch}.zip"
	(
		portable_work=$(mktemp -d)
		archive_verified=false
		cleanup_portable_work() {
			rm -rf "$portable_work"
			[ "$archive_verified" = true ] || rm -f "$portable_zip"
		}
		trap cleanup_portable_work EXIT

		staging="$portable_work/staging"
		archive_extract="$portable_work/extracted"
		mkdir -p "$staging/runtimes/python/3.12.10/$arch" "$archive_extract"
		cp "$portable" "$staging/${WINDOWS_BINNAME}.exe"
		cp -R "build/windows/python-runtime/$arch/." "$staging/runtimes/python/3.12.10/$arch/"
		src_win=$(cygpath -w "$staging/${WINDOWS_BINNAME}.exe")
		runtime_win=$(cygpath -w "$staging/runtimes")
		zip_win=$(cygpath -w "$portable_zip")
		archive_extract_win=$(cygpath -w "$archive_extract")
		pwsh.exe -NoLogo -NoProfile -Command "Compress-Archive -Force -LiteralPath '$src_win','$runtime_win' -DestinationPath '$zip_win'"
		pwsh.exe -NoLogo -NoProfile -Command "Expand-Archive -Force -LiteralPath '$zip_win' -DestinationPath '$archive_extract_win'"

		archive_application_win=$(cygpath -w "$archive_extract/${WINDOWS_BINNAME}.exe")
		archive_verification_args=(
			-NoLogo -NoProfile -ExecutionPolicy Bypass -File "$verifier_win"
			-Architecture "$arch" -ApplicationPath "$archive_application_win" \
			-PayloadDirectory "$archive_extract_win" -ExpectedVersion "$numver" \
			-ExpectedProductName "$APP_BUNDLE_NAME" -ExpectedCompanyName "Clouvia"
		)
		if [ "$windows_signing" = true ]; then
			archive_verification_args+=(
				-RequireSignature
				-ExpectedSignerThumbprint "$WINDOWS_SIGN_CERT_THUMBPRINT"
				-ExpectedSignerSubject "$WINDOWS_SIGN_CERT_SUBJECT"
				-SignToolPath "$WINDOWS_SIGNTOOL_PATH"
			)
		fi
		pwsh.exe "${archive_verification_args[@]}"
		archive_verified=true
	)
	;;
linux)
	tar -czf "$ROOT/dist/${ARTIFACT_NAME}-linux-${arch}.tar.gz" -C build/bin "$BINNAME"
	# Also build a .deb for Debian/Ubuntu users (goreleaser/nfpm; see
	# desktop/build/linux/nfpm.yaml). Human-download only: the Linux updater channel
	# stays the tarball and cmd/sign's manifest skips .deb files. nfpm reads
	# $DEB_VERSION/$DEB_ARCH — dpkg wants a strict numeric version, so reuse numver.
	DEB_VERSION="$numver" DEB_ARCH="$arch" \
		nfpm package --config build/linux/nfpm.yaml --packager deb \
		--target "$ROOT/dist/${ARTIFACT_NAME}-linux-${arch}.deb"
	;;
*)
	echo "unsupported os: $os" >&2
	exit 1
	;;
esac

echo "==> packaged into dist/:"
ls -la "$ROOT/dist"
