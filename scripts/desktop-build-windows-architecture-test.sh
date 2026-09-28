#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

repo="$tmp/repo"
bin="$tmp/bin"
mkdir -p "$repo/scripts" "$repo/desktop" "$bin"
cp "$ROOT/scripts/desktop-build.sh" "$repo/scripts/desktop-build.sh"
printf '{"info":{"productVersion":"0.0.0"}}\n' >"$repo/desktop/wails.json"

cat >"$bin/node" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$bin/wails" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$MOCK_WAILS_LOG"
platform=''
output=''
while [ "$#" -gt 0 ]; do
	if [ "$1" = -platform ]; then
		platform="$2"
		shift 2
		continue
	fi
	if [ "$1" = -o ]; then
		output="$2"
		shift 2
		continue
	fi
	shift
done
arch="${platform#windows/}"
[ "$output" = clouvia-desktop.exe ] || {
	echo "unexpected Windows output name: $output" >&2
	exit 1
}
rm -rf build/bin
mkdir -p build/bin
printf 'application fixture\n' >"build/bin/$output"
printf 'x86 NSIS launcher fixture\n' >"build/bin/kwu-desktop-${arch}-installer.exe"
EOF
cat >"$bin/cygpath" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "${@: -1}"
EOF
cat >"$bin/pwsh.exe" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
command_line="$*"
printf '%s\n' "$command_line" >>"$KWU_TEST_POWERSHELL_LOG"
if printf '%s\n' "$command_line" | grep -F 'prepare-python-runtime.ps1' >/dev/null; then
	arch=''
	while [ "$#" -gt 0 ]; do
		if [ "$1" = -Architecture ]; then
			arch="$2"
			break
		fi
		shift
	done
	[ -n "$arch" ] || exit 1
	runtime="build/windows/python-runtime/$arch"
	mkdir -p "$runtime"
	printf 'python fixture\n' >"$runtime/python.exe"
	printf 'license fixture\n' >"$runtime/LICENSE.txt"
	printf 'pth fixture\n' >"$runtime/python312._pth.kwu-disabled"
	printf '{"schema":1,"version":"3.12.10","arch":"%s","source_sha256":"fixture","source_size":1}\n' "$arch" >"$runtime/.kwu-runtime.json"
fi
if printf '%s\n' "$command_line" | grep -F 'Compress-Archive' >/dev/null; then
	source_app=$(printf '%s' "$command_line" | cut -d"'" -f2)
	source_runtimes=$(printf '%s' "$command_line" | cut -d"'" -f4)
	archive=$(printf '%s' "$command_line" | cut -d"'" -f6)
	mkdir -p "$(dirname "$archive")" "$archive.contents"
	cp "$source_app" "$archive.contents/"
	cp -R "$source_runtimes" "$archive.contents/"
	printf 'portable archive fixture\n' >"$archive"
fi
if printf '%s\n' "$command_line" | grep -F 'Expand-Archive' >/dev/null; then
	archive=$(printf '%s' "$command_line" | cut -d"'" -f2)
	destination=$(printf '%s' "$command_line" | cut -d"'" -f4)
	mkdir -p "$destination"
	cp -R "$archive.contents/." "$destination/"
	rm -rf "$archive.contents"
fi
if printf '%s\n' "$command_line" | grep -F 'assert-windows-release-architecture.ps1' >/dev/null &&
	printf '%s\n' "$command_line" | grep -F -- '-PayloadDirectory ' >/dev/null; then
	application=''
	payload_directory=''
	while [ "$#" -gt 0 ]; do
		case "$1" in
		-ApplicationPath) application="$2"; shift 2 ;;
		-PayloadDirectory) payload_directory="$2"; shift 2 ;;
		*) shift ;;
		esac
	done
	[ -f "$application" ]
	[ -f "$payload_directory/runtimes/python/3.12.10"/*/python.exe ]
	[ "${KWU_TEST_FAIL_ARCHIVE_VERIFY:-0}" != 1 ]
fi
exit 0
EOF
cp "$bin/pwsh.exe" "$bin/powershell.exe"
chmod +x "$bin/node" "$bin/wails" "$bin/cygpath" "$bin/pwsh.exe" "$bin/powershell.exe" "$repo/scripts/desktop-build.sh"

export KWU_TEST_POWERSHELL_LOG="$tmp/powershell.log"
export MOCK_WAILS_LOG="$tmp/wails.log"
export TMPDIR="$tmp/build-tmp"
mkdir -p "$TMPDIR"
for arch in amd64 arm64; do
	: >"$KWU_TEST_POWERSHELL_LOG"
	PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" "windows/$arch" v1.0.11 stable >/dev/null
	prepare=$(grep -F 'prepare-python-runtime.ps1' "$KWU_TEST_POWERSHELL_LOG" | tail -n1)
	printf '%s\n' "$prepare" | grep -F -- "-Architecture $arch" >/dev/null
	validation=$(grep -F 'assert-windows-release-architecture.ps1' "$KWU_TEST_POWERSHELL_LOG" | head -n1)
	printf '%s\n' "$validation" | grep -F -- "-Architecture $arch" >/dev/null
	printf '%s\n' "$validation" | grep -F -- '-ApplicationPath ' >/dev/null
	printf '%s\n' "$validation" | grep -F -- '/desktop/build/bin/clouvia-desktop.exe' >/dev/null
	printf '%s\n' "$validation" | grep -F -- '-InstallerPath ' >/dev/null
	printf '%s\n' "$validation" | grep -F -- "/desktop/build/bin/kwu-desktop-${arch}-installer.exe" >/dev/null
	printf '%s\n' "$validation" | grep -F -- '-ExpectedVersion 1.0.11' >/dev/null
	printf '%s\n' "$validation" | grep -F -- '-ExpectedProductName 云智企办' >/dev/null
	printf '%s\n' "$validation" | grep -F -- '-ExpectedCompanyName Clouvia' >/dev/null
	[ -f "$repo/dist/Clouvia-windows-${arch}-installer.exe" ] || {
		echo "Windows $arch installer was not packaged" >&2
		exit 1
	}
	archive=$(grep -F 'Compress-Archive' "$KWU_TEST_POWERSHELL_LOG")
	printf '%s\n' "$archive" | grep -F -- '/clouvia-desktop.exe' >/dev/null
	printf '%s\n' "$archive" | grep -F -- '/runtimes' >/dev/null
	grep -F 'Expand-Archive' "$KWU_TEST_POWERSHELL_LOG" >/dev/null
	archive_validation=$(grep -F 'assert-windows-release-architecture.ps1' "$KWU_TEST_POWERSHELL_LOG" | tail -n1)
	printf '%s\n' "$archive_validation" | grep -F -- '-PayloadDirectory ' >/dev/null
	printf '%s\n' "$archive_validation" | grep -F -- '/extracted/clouvia-desktop.exe' >/dev/null
	compress_line=$(grep -n -F 'Compress-Archive' "$KWU_TEST_POWERSHELL_LOG" | cut -d: -f1)
	extract_line=$(grep -n -F 'Expand-Archive' "$KWU_TEST_POWERSHELL_LOG" | cut -d: -f1)
	verify_line=$(grep -n -F -- '-PayloadDirectory ' "$KWU_TEST_POWERSHELL_LOG" | cut -d: -f1)
	[ "$compress_line" -lt "$extract_line" ] && [ "$extract_line" -lt "$verify_line" ]
	[ -f "$repo/dist/Clouvia-windows-${arch}.zip" ]
	[ -z "$(find "$TMPDIR" -mindepth 1 -print -quit)" ]
done

if KWU_TEST_FAIL_ARCHIVE_VERIFY=1 PATH="$bin:$PATH" \
	"$repo/scripts/desktop-build.sh" windows/amd64 v1.0.11 stable >/dev/null 2>&1; then
	echo 'Windows build accepted a failed final portable archive verification' >&2
	exit 1
fi
[ ! -f "$repo/dist/Clouvia-windows-amd64.zip" ]
[ -z "$(find "$TMPDIR" -mindepth 1 -print -quit)" ]

full_build=$(head -n1 "$MOCK_WAILS_LOG")
printf '%s\n' "$full_build" | grep -F -- '-clean' >/dev/null
if printf '%s\n' "$full_build" | grep -F -- '-skipbindings' >/dev/null; then
	echo 'a full single-target Windows build unexpectedly skipped frontend work' >&2
	exit 1
fi
PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" windows/arm64 v1.0.11 stable reuse-frontend >/dev/null
reused_build=$(tail -n1 "$MOCK_WAILS_LOG")
for required in -clean -s -skipbindings -skipembedcreate '-platform windows/arm64' -nsis; do
	printf '%s\n' "$reused_build" | grep -F -- "$required" >/dev/null
done
if PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" linux/amd64 v1.0.11 stable reuse-frontend >/dev/null 2>&1; then
	echo 'reuse-frontend unexpectedly accepted a non-Windows build' >&2
	exit 1
fi

if KWU_WINDOWS_RELEASE_SIGNING=1 \
	WINDOWS_SIGN_CERT_THUMBPRINT=0123456789abcdef0123456789abcdef01234567 \
	WINDOWS_SIGN_CERT_SUBJECT='CN=Example' \
	WINDOWS_SIGNTOOL_PATH='C:\tools\signtool.exe' \
	PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" windows/amd64 v1.0.11 stable >/dev/null 2>&1; then
	echo 'release signing unexpectedly accepted a missing evidence path' >&2
	exit 1
fi

: >"$KWU_TEST_POWERSHELL_LOG"
evidence="$tmp/amd64-evidence.json"
KWU_WINDOWS_RELEASE_SIGNING=1 \
	KWU_WINDOWS_SIGN_EVIDENCE_PATH="$evidence" \
	WINDOWS_SIGN_CERT_THUMBPRINT=0123456789abcdef0123456789abcdef01234567 \
	WINDOWS_SIGN_CERT_SUBJECT='CN=Example' \
	WINDOWS_SIGNTOOL_PATH='C:\tools\signtool.exe' \
	PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" windows/amd64 v1.0.11 stable >/dev/null
grep -F -- 'desktop-windows-authenticode.ps1 -Action Preflight' "$KWU_TEST_POWERSHELL_LOG" >/dev/null
grep -F -- '-RequireSignature -ExpectedSignerThumbprint' "$KWU_TEST_POWERSHELL_LOG" >/dev/null
grep -F -- '-Action AssertEvidence -Architecture amd64' "$KWU_TEST_POWERSHELL_LOG" >/dev/null

printf 'stale evidence\n' >"$evidence"
if KWU_WINDOWS_RELEASE_SIGNING=1 \
	KWU_WINDOWS_SIGN_EVIDENCE_PATH="$evidence" \
	WINDOWS_SIGN_CERT_THUMBPRINT=0123456789abcdef0123456789abcdef01234567 \
	WINDOWS_SIGN_CERT_SUBJECT='CN=Example' \
	WINDOWS_SIGNTOOL_PATH='C:\tools\signtool.exe' \
	PATH="$bin:$PATH" "$repo/scripts/desktop-build.sh" windows/amd64 v1.0.11 stable >/dev/null 2>&1; then
	echo 'release signing unexpectedly reused existing evidence' >&2
	exit 1
fi

echo 'desktop-build validates Windows architecture and gates release-only signing evidence'
