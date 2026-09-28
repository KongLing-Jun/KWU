// Run: tsx src/__tests__/installer-contract.test.ts

import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

let passed = 0;
let failed = 0;

function ok(cond: boolean, label: string) {
  if (cond) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

const here = dirname(fileURLToPath(import.meta.url));
const projectNsiBytes = readFileSync(resolve(here, "../../../build/windows/installer/project.nsi"));
const projectNsi = projectNsiBytes.toString("utf8").replace(/\r\n/g, "\n");
const installerStrings = readFileSync(
  resolve(here, "../../../build/windows/installer/strings_zh.nsh"),
  "utf8",
);
const versionInfo = JSON.parse(readFileSync(resolve(here, "../../../build/windows/info.json"), "utf8")) as {
  fixed?: { file_version?: string; product_version?: string };
  info?: Record<string, Record<string, string>>;
};
const desktopBuildScript = readFileSync(resolve(here, "../../../../scripts/desktop-build.sh"), "utf8");
const pythonRuntimePrepareScript = readFileSync(
  resolve(here, "../../../../scripts/prepare-python-runtime.ps1"),
  "utf8",
);
const pythonRuntimeLock = JSON.parse(
  readFileSync(resolve(here, "../../../build/windows/python-runtime.lock.json"), "utf8"),
) as {
  schema?: number;
  version?: string;
  artifacts?: Record<
    string,
    { url?: string; bytes?: number; sha256?: string; payload_files?: number; payload_sha256?: string }
  >;
};
const wailsConfig = JSON.parse(readFileSync(resolve(here, "../../../wails.json"), "utf8")) as {
  "frontend:dev:watcher"?: string;
};
const frontendPackage = JSON.parse(readFileSync(resolve(here, "../../package.json"), "utf8")) as {
  scripts?: Record<string, string>;
};
const connectorNodeRuntimeScriptPath = resolve(
  here,
  "../../../../scripts/desktop-connector-node-runtime.mjs",
);
const connectorNodeVersionPath = resolve(here, "../../../internal/connector/assets/node-version.txt");

console.log("\ninstaller contract");

ok(
  projectNsiBytes[0] === 0xef && projectNsiBytes[1] === 0xbb && projectNsiBytes[2] === 0xbf,
  "main NSIS source declares UTF-8 through a byte-order mark",
);

ok(
  /!define PRODUCT_EXECUTABLE "clouvia-desktop\.exe"/.test(projectNsi) &&
    /!define UNINST_KEY_NAME "ClouviaCloudOffice"/.test(projectNsi),
  "Windows runtime filename and uninstall identity use stable ASCII values",
);

ok(
  /nsExec::Exec '"\$SYSDIR\\taskkill\.exe" \/IM "\$\{PRODUCT_EXECUTABLE\}" \/T \/F'\s+Pop \$0/.test(
    projectNsi,
  ),
  "uninstaller hides taskkill while preserving its arguments and consuming the exit result",
);

ok(
  !/ExecWait\s+'"[^']*taskkill\.exe[^']*'/.test(projectNsi),
  "uninstaller does not launch taskkill directly through ExecWait",
);

ok(
  /Delete "\$INSTDIR\\\$\{PRODUCT_EXECUTABLE\}"[\s\S]*IfErrors[\s\S]*IfFileExists "\$INSTDIR\\\$\{PRODUCT_EXECUTABLE\}"[\s\S]*Abort /.test(projectNsi),
  "uninstaller aborts instead of reporting success when the executable remains on disk",
);

ok(
  /!define KWU_DEFAULT_INSTALLDIR "\$LOCALAPPDATA\\Programs\\\$\{INFO_PRODUCTNAME\}"/.test(projectNsi),
  "default install path reuses the generated Unicode product name",
);

ok(
  /Abort "[^"]*Close \$\{INFO_PRODUCTNAME\} and run uninstall again\."/.test(projectNsi),
  "uninstaller messages reuse the generated Unicode product name",
);

ok(
  /IfFileExists "\$INSTDIR\\uninstall\.exe" 0 unsupportedInstalledUpdate[\s\S]*StrCmp "\$KwuUpdatePID" "" done[\s\S]*Abort/.test(
    projectNsi,
  ),
  "an UPDATEPID portable launch is rejected instead of being converted into an installed app",
);

ok(
  /checkUpdateKwu:[\s\S]*KWU_LEGACY_UNINST_KEY[\s\S]*checkUpdateMojibake:[\s\S]*KWU_MOJIBAKE_UNINST_KEY[\s\S]*checkUpdateSuiYuan:[\s\S]*SUIYUAN_UNINST_KEY[\s\S]*checkUpdateReasonix:[\s\S]*REASONIX_UNINST_KEY/.test(
    projectNsi,
  ),
  "brand migration recognizes every verified legacy uninstall identity",
);

ok(
  /updateWaitDone:[\s\S]*IfFileExists "\$INSTDIR\\uninstall\.exe"[\s\S]*legacyDetected:[\s\S]*StrCpy \$KwuLegacyInstallDir "\$INSTDIR"[\s\S]*StrCpy \$INSTDIR "\$\{KWU_DEFAULT_INSTALLDIR\}"[\s\S]*SetSilent normal/.test(
    projectNsi,
  ),
  "v1.0.0/v1.0.1 /D updates and v1.0.2 /S /UPDATEPID /D updates become interactive migrations",
);

ok(
  /!define UNINST_KEY_NAME "ClouviaCloudOffice"[\s\S]*InstallDirRegKey HKCU "\$\{UNINST_KEY\}" "InstallLocation"/.test(
    projectNsi,
  ) &&
    /StrCmp "\$KwuLegacyInstallDir" "" 0 sectionDone/.test(projectNsi),
  "canonical v1.0.3 updates keep the registered directory and silent relaunch path",
);

ok(
  (projectNsi.match(/!insertmacro wails\.files/g) ?? []).length === 1,
  "installer declares the Wails payload once so NSIS does not double-count required space",
);

ok(
  pythonRuntimeLock.schema === 1 &&
    pythonRuntimeLock.version === "3.12.10" &&
    pythonRuntimeLock.artifacts?.amd64?.url ===
      "https://www.python.org/ftp/python/3.12.10/python-3.12.10-embed-amd64.zip" &&
    pythonRuntimeLock.artifacts.amd64.bytes === 11133606 &&
    pythonRuntimeLock.artifacts.amd64.sha256 ===
      "4acbed6dd1c744b0376e3b1cf57ce906f9dc9e95e68824584c8099a63025a3c3" &&
    pythonRuntimeLock.artifacts.amd64.payload_files === 36 &&
    pythonRuntimeLock.artifacts.amd64.payload_sha256 ===
      "37d3f0b2de372d2cebf88012cbed9eca64c9a59e3fe15c75787805067e253629" &&
    pythonRuntimeLock.artifacts?.arm64?.url ===
      "https://www.python.org/ftp/python/3.12.10/python-3.12.10-embed-arm64.zip" &&
    pythonRuntimeLock.artifacts.arm64.bytes === 10413299 &&
    pythonRuntimeLock.artifacts.arm64.sha256 ===
      "3065efc3d382d1cda66757ac71ade11904fa6e350f5a97eb74811acd71ba5532" &&
    pythonRuntimeLock.artifacts.arm64.payload_files === 36 &&
    pythonRuntimeLock.artifacts.arm64.payload_sha256 ===
      "ad17c601d2ca8359e07a6e80790dfd00c4b9b913bad667fe8d0558d81fc99515",
  "managed Python archives and transformed payload trees are pinned by size and SHA-256",
);

ok(
  pythonRuntimePrepareScript.includes("ConvertFrom-Json") &&
    pythonRuntimePrepareScript.includes("Get-FileHash") &&
    pythonRuntimePrepareScript.includes("Expand-Archive") &&
    pythonRuntimePrepareScript.includes("python312._pth.kwu-disabled") &&
    pythonRuntimePrepareScript.includes("Get-PayloadReceipt") &&
    pythonRuntimePrepareScript.includes("payload_sha256") &&
    pythonRuntimePrepareScript.includes("Test-RuntimeDirectoryMatches") &&
    pythonRuntimePrepareScript.includes("[System.IO.File]::WriteAllText") &&
    !/Mutex|lockfile/i.test(pythonRuntimePrepareScript),
  "runtime preparation validates the structured lock and publishes a lock-free extracted runtime",
);

type OwnerLifecycle = {
  owner: string | null;
  handles: Set<string>;
};

function acquireOwner(state: OwnerLifecycle, process: string) {
  if (state.handles.has(process)) return "reentrant";
  if (state.owner !== null) return "rejected";
  state.owner = process;
  state.handles.add(process);
  return "acquired";
}

function closeOwnerHandle(state: OwnerLifecycle, process: string) {
  if (!state.handles.delete(process)) return false;
  if (state.owner === process) state.owner = null;
  return true;
}

const ownerLifecycle: OwnerLifecycle = { owner: null, handles: new Set<string>() };
ok(
  acquireOwner(ownerLifecycle, "installer-a") === "acquired" &&
    acquireOwner(ownerLifecycle, "installer-a") === "reentrant" &&
    acquireOwner(ownerLifecycle, "installer-b") === "rejected" &&
    closeOwnerHandle(ownerLifecycle, "installer-a") &&
    acquireOwner(ownerLifecycle, "installer-b") === "acquired" &&
    closeOwnerHandle(ownerLifecycle, "installer-b") &&
    ownerLifecycle.owner === null &&
    ownerLifecycle.handles.size === 0,
  "process-lifetime ownership rejects concurrent installers and clears on close or crash",
);

const ownerFunctionsStart = projectNsi.indexOf("!macro kwu.defineTransactionOwnerFunctions PREFIX");
const ownerFunctionsEnd = projectNsi.indexOf("!macroend", ownerFunctionsStart);
const ownerFunctions = projectNsi.slice(ownerFunctionsStart, ownerFunctionsEnd);
const ownerCreate = ownerFunctions.indexOf("CreateEventW");
const ownerLastError = ownerFunctions.indexOf("Pop $1", ownerCreate);
const ownerAlreadyExists = ownerFunctions.indexOf(
  "IntCmp $1 183 ownerAcquireAlreadyExists ownerAcquireStore ownerAcquireStore",
  ownerLastError,
);
const ownerContenderClose = ownerFunctions.indexOf("CloseHandle(p r0)", ownerAlreadyExists);
const ownerHandleStore = ownerFunctions.indexOf("StrCpy $KwuOwnerHandle $0", ownerContenderClose);
const ownerReleaseStart = ownerFunctions.indexOf("Function ${PREFIX}KwuReleaseTransactionOwner");
const ownerReleaseClose = ownerFunctions.indexOf(
  "CloseHandle(p $KwuOwnerHandle)",
  ownerReleaseStart,
);
const ownerReleaseClear = ownerFunctions.indexOf(
  'StrCpy $KwuOwnerHandle ""',
  ownerReleaseClose,
);
ok(
  projectNsi.includes(
    '!define KWU_TRANSACTION_OWNER_NAME "Global\\ClouviaCloudOfficeInstallerTransactionOwnerV1"',
  ) &&
    ownerFunctionsStart >= 0 &&
    ownerFunctionsEnd > ownerFunctionsStart &&
    (ownerFunctions.match(/CreateEventW/g) ?? []).length === 1 &&
    ownerFunctions.includes('w "${KWU_TRANSACTION_OWNER_NAME}") p .r0 ?e\'') &&
    ownerLastError > ownerCreate &&
    ownerAlreadyExists > ownerLastError &&
    ownerContenderClose > ownerAlreadyExists &&
    ownerHandleStore > ownerContenderClose &&
    ownerReleaseStart > ownerHandleStore &&
    ownerReleaseClose > ownerReleaseStart &&
    ownerReleaseClear > ownerReleaseClose &&
    !/Mutex|CreateMutex|OpenMutex|WaitFor|WaitForSingleObject|\bSleep\b|SetEvent|ResetEvent|Semaphore|FileOpen|MoveFileEx|tombstone|owner\.claim/i.test(
      ownerFunctions,
    ),
  "installer ownership is an atomic named-event lifetime claim without waits or persistent state",
);

ok(
  ownerFunctions.includes("Function ${PREFIX}KwuAcquireTransactionOwner") &&
    ownerFunctions.includes("Function ${PREFIX}KwuReleaseTransactionOwner") &&
    projectNsi.includes('!insertmacro kwu.defineTransactionOwnerFunctions ""') &&
    projectNsi.includes('!insertmacro kwu.defineTransactionOwnerFunctions "un."'),
  "installer and uninstaller share the same process-lifetime owner protocol",
);

const ownerInstallSectionStart = projectNsi.indexOf("\nSection\n");
const ownerInstallSectionEnd = projectNsi.indexOf("SectionEnd", ownerInstallSectionStart);
const ownerInstallSection = projectNsi.slice(ownerInstallSectionStart, ownerInstallSectionEnd);
const ownerInstallAcquire = ownerInstallSection.indexOf("Call KwuAcquireTransactionOwner");
const ownerInstallRelease = ownerInstallSection.lastIndexOf("Call KwuReleaseTransactionOwner");
ok(
  ownerInstallAcquire >= 0 &&
    ownerInstallAcquire < ownerInstallSection.indexOf("!insertmacro wails.webview2runtime") &&
    ownerInstallAcquire < ownerInstallSection.indexOf("!insertmacro wails.files") &&
    ownerInstallAcquire < ownerInstallSection.indexOf('RMDir /r "$INSTDIR\\.kwu-update-staging"') &&
    ownerInstallRelease > ownerInstallSection.indexOf('Exec \'"$INSTDIR\\${PRODUCT_EXECUTABLE}"\'') &&
    ownerInstallRelease < ownerInstallSection.length,
  "installer holds process-lifetime ownership across every shared mutation",
);

const ownerUninstallSectionStart = projectNsi.indexOf('Section "uninstall"');
const ownerUninstallSectionEnd = projectNsi.indexOf("SectionEnd", ownerUninstallSectionStart);
const ownerUninstallSection = projectNsi.slice(
  ownerUninstallSectionStart,
  ownerUninstallSectionEnd,
);
const ownerUninstallAcquire = ownerUninstallSection.indexOf(
  "Call un.KwuAcquireTransactionOwner",
);
const ownerUninstallRelease = ownerUninstallSection.lastIndexOf(
  "Call un.KwuReleaseTransactionOwner",
);
ok(
  ownerUninstallAcquire >= 0 &&
    ownerUninstallAcquire <
      ownerUninstallSection.indexOf('RMDir /r "$AppData\\${PRODUCT_EXECUTABLE}"') &&
    ownerUninstallRelease > ownerUninstallSection.indexOf('RMDir $INSTDIR') &&
    ownerUninstallRelease < ownerUninstallSection.length,
  "uninstaller holds the same owner through its final shared mutation",
);

const installPayloadStart = projectNsi.indexOf("installPayload:", projectNsi.indexOf("\nSection\n"));
const applicationStageStart = projectNsi.indexOf(
  'SetOutPath "$INSTDIR\\.kwu-update-staging"',
  installPayloadStart,
);
const applicationPayloadStart = projectNsi.indexOf("!insertmacro wails.files", applicationStageStart);
const runtimeStageStart = projectNsi.indexOf(
  'SetOutPath "$INSTDIR\\runtimes\\python\\3.12.10\\.${ARCH}.staging"',
  applicationPayloadStart,
);
const backupStageStart = projectNsi.indexOf(
  'CreateDirectory "$INSTDIR\\.kwu-update-backup.staging"',
  runtimeStageStart,
);
const backupActiveStart = projectNsi.indexOf(
  'Rename "$INSTDIR\\.kwu-update-backup.staging" "$INSTDIR\\.kwu-update-backup.active"',
  backupStageStart,
);
const runtimePublishStart = projectNsi.indexOf(
  'Rename "$INSTDIR\\runtimes\\python\\3.12.10\\.${ARCH}.staging" "$INSTDIR\\runtimes\\python\\3.12.10\\${ARCH}"',
  backupActiveStart,
);
const applicationPublishStart = projectNsi.indexOf(
  'Rename "$INSTDIR\\.kwu-update-staging\\${PRODUCT_EXECUTABLE}" "$INSTDIR\\${PRODUCT_EXECUTABLE}"',
  runtimePublishStart,
);
const payloadValidationStart = projectNsi.indexOf(
  'IfFileExists "$INSTDIR\\${PRODUCT_EXECUTABLE}" 0 updateFailed',
  applicationPublishStart,
);
const backupCommitStart = projectNsi.indexOf(
  'Rename "$INSTDIR\\.kwu-update-backup.active" "$INSTDIR\\.kwu-update-backup.committed"',
  payloadValidationStart,
);
const backupCleanupStart = projectNsi.indexOf(
  'RMDir /r "$INSTDIR\\.kwu-update-backup.committed"',
  backupCommitStart,
);
ok(
  applicationStageStart > installPayloadStart &&
    applicationPayloadStart > applicationStageStart &&
    runtimeStageStart > applicationPayloadStart &&
    backupStageStart > runtimeStageStart &&
    backupActiveStart > backupStageStart &&
    runtimePublishStart > backupActiveStart &&
    applicationPublishStart > runtimePublishStart &&
    payloadValidationStart > applicationPublishStart &&
    backupCommitStart > payloadValidationStart &&
    backupCleanupStart > backupCommitStart &&
    /File \/r "\.\.\\python-runtime\\\$\{ARCH\}\\\*\.\*"/.test(projectNsi) &&
    projectNsi.includes('WriteUninstaller "$INSTDIR\\.kwu-update-staging\\uninstall.exe"') &&
    projectNsi.includes(
      'Rename "$INSTDIR\\runtimes\\python\\3.12.10\\${ARCH}" "$INSTDIR\\.kwu-update-backup.staging\\python-runtime"',
    ) &&
    !projectNsi.includes(".previous") &&
    /Exec '\"\$INSTDIR\\\$\{PRODUCT_EXECUTABLE\}\"'[\s\S]*IfErrors postCommitFailed sectionDone/.test(projectNsi) &&
    (projectNsi.match(/RMDir \/r "\$INSTDIR\\runtimes\\python"/g) ?? []).length === 1 &&
    projectNsi.indexOf('RMDir /r "$INSTDIR\\runtimes\\python"') >
      projectNsi.indexOf('Section "uninstall"'),
  "installer stages one backup transaction and commits it only after validating every new payload",
);

const committedRecoveryStart = projectNsi.indexOf(
  'IfFileExists "$INSTDIR\\.kwu-update-backup.committed" 0 recoverInterruptedActive',
  installPayloadStart,
);
const activeRecoveryStart = projectNsi.indexOf("recoverInterruptedActive:", committedRecoveryStart);
const stagingRecoveryStart = projectNsi.indexOf("recoverInterruptedStaging:", activeRecoveryStart);
ok(
  committedRecoveryStart > installPayloadStart &&
    activeRecoveryStart > committedRecoveryStart &&
    stagingRecoveryStart > activeRecoveryStart &&
    projectNsi
      .slice(committedRecoveryStart, activeRecoveryStart)
      .includes('IfFileExists "$INSTDIR\\.kwu-update-backup.committed" transactionRecoveryFailed') &&
    projectNsi.includes('StrCpy $KwuBackupRoot "$INSTDIR\\.kwu-update-backup.active"') &&
    projectNsi.includes('StrCpy $KwuBackupRoot "$INSTDIR\\.kwu-update-backup.staging"') &&
    projectNsi.includes("StrCpy $KwuBackupMissingDeletes 1") &&
    projectNsi.includes("StrCpy $KwuBackupMissingDeletes 0") &&
    projectNsi.includes(
      'CopyFiles /SILENT "$KwuBackupRoot\\${PRODUCT_EXECUTABLE}" "$INSTDIR"',
    ) &&
    projectNsi.includes('CopyFiles /SILENT "$KwuBackupRoot\\uninstall.exe" "$INSTDIR"') &&
    projectNsi.includes(
      'CopyFiles /SILENT "$KwuBackupRoot\\python-runtime\\*.*" "$INSTDIR\\runtimes\\python\\3.12.10\\${ARCH}"',
    ) &&
    /updateFailed:[\s\S]*IfFileExists "\$INSTDIR\\\.kwu-update-backup\.active"[\s\S]*updateFailedCheckStagingBackup:[\s\S]*IfFileExists "\$INSTDIR\\\.kwu-update-backup\.staging"[\s\S]*restoreBackupApplication:/.test(
      projectNsi,
    ),
  "installer retries committed cleanup and recovers active or partially collected backups deterministically",
);

ok(
  projectNsi.includes(
    'Rename "$INSTDIR\\.kwu-update-backup.active" "$INSTDIR\\.kwu-update-backup.committed"',
  ) &&
    projectNsi.includes(
      'IfFileExists "$INSTDIR\\.kwu-update-backup.committed" postCommitFailed',
    ) &&
    /postCommitFailed:[\s\S]*updateFailed:[\s\S]*transactionRecoveryFailed:/.test(projectNsi),
  "backup commit marker precedes cleanup and cleanup failure cannot enter pre-commit rollback",
);

const uninstallSectionStart = projectNsi.indexOf('Section "uninstall"');
const uninstallSectionEnd = projectNsi.indexOf("SectionEnd", uninstallSectionStart);
const uninstallSection = projectNsi.slice(uninstallSectionStart, uninstallSectionEnd);
const installerOwnedTransactionPaths = [
  ".kwu-update-staging",
  ".kwu-update-backup.staging",
  ".kwu-update-backup.active",
  ".kwu-update-backup.committed",
  "runtimes\\python\\3.12.10\\.${ARCH}.staging",
];
ok(
  uninstallSectionStart >= 0 &&
    uninstallSectionEnd > uninstallSectionStart &&
    installerOwnedTransactionPaths.every((path) =>
      uninstallSection.includes(`RMDir /r "$INSTDIR\\${path}"`),
    ) &&
    uninstallSection.includes('RMDir /r "$INSTDIR\\runtimes\\python"') &&
    installerOwnedTransactionPaths
      .slice(0, 4)
      .every((path) =>
        uninstallSection.includes(
          `IfFileExists "$INSTDIR\\${path}" uninstallTransactionCleanupFailed`,
        ),
      ) &&
    uninstallSection.includes(
      'IfFileExists "$INSTDIR\\runtimes\\python" uninstallTransactionCleanupFailed',
    ) &&
    uninstallSection.includes("uninstallTransactionCleanupFailed:") &&
    !uninstallSection.includes('RMDir /r "$INSTDIR"'),
  "uninstall removes every known installer transaction state without recursively deleting the install root",
);

const portableArchiveCreate = desktopBuildScript.indexOf("Compress-Archive");
const portableArchiveExtract = desktopBuildScript.indexOf("Expand-Archive", portableArchiveCreate);
const portableArchiveVerify = desktopBuildScript.indexOf(
  'archive_verification_args=(',
  portableArchiveExtract,
);
ok(
  desktopBuildScript.indexOf("prepare-python-runtime.ps1") <
    desktopBuildScript.indexOf('wails build "${build_args[@]}"') &&
    desktopBuildScript.includes('build/windows/python-runtime/$arch/.') &&
    desktopBuildScript.includes("'$src_win','$runtime_win'") &&
    desktopBuildScript.includes("runtimes/python/3.12.10/$arch") &&
    portableArchiveCreate >= 0 &&
    portableArchiveExtract > portableArchiveCreate &&
    portableArchiveVerify > portableArchiveExtract &&
    desktopBuildScript.includes('-PayloadDirectory "$archive_extract_win"') &&
    desktopBuildScript.includes("portable_work=$(mktemp -d)") &&
    desktopBuildScript.includes("trap cleanup_portable_work EXIT") &&
    desktopBuildScript.includes('[ "$archive_verified" = true ] || rm -f "$portable_zip"'),
  "Windows builds verify the extracted final portable ZIP with the production architecture and runtime verifier",
);

ok(
  wailsConfig["frontend:dev:watcher"] === "pnpm run dev:wails" &&
    frontendPackage.scripts?.dev === "vite" &&
    frontendPackage.scripts?.["dev:wails"] === "pnpm dev",
  "Wails development starts Vite without preparing a managed connector runtime",
);

ok(
  !existsSync(connectorNodeRuntimeScriptPath) &&
    !existsSync(connectorNodeVersionPath) &&
    !desktopBuildScript.includes("desktop-connector-node-runtime.mjs") &&
    !projectNsi.includes("KWU_CONNECTOR_NODE") &&
    !projectNsi.includes("runtime\\node\\runtime.nsh"),
  "development and release builds do not generate or carry a managed connector Node runtime",
);

ok(
  (projectNsi.match(/RMDir \/r "\$INSTDIR\\runtimes\\node"/g) ?? []).length === 2 &&
    (projectNsi.match(/RMDir \/r "\$INSTDIR\\runtimes\\feishu"/g) ?? []).length === 2 &&
    /newInstallCommitted:[\s\S]*RMDir \/r "\$INSTDIR\\runtimes\\node"[\s\S]*RMDir \/r "\$INSTDIR\\runtimes\\feishu"/.test(projectNsi) &&
    /Section "uninstall"[\s\S]*RMDir \/r "\$INSTDIR\\runtimes\\node"[\s\S]*RMDir \/r "\$INSTDIR\\runtimes\\feishu"/.test(projectNsi),
  "successful upgrades and uninstall remove only the two historical managed runtime directories",
);

const closeApplicationsStart = projectNsi.indexOf("Function KwuDetectRunningInstalledApplications");
const closeApplicationsEnd = projectNsi.indexOf("\nFunction .onInit", closeApplicationsStart);
const closeApplicationsBlock =
  closeApplicationsStart >= 0 && closeApplicationsEnd > closeApplicationsStart
    ? projectNsi.slice(closeApplicationsStart, closeApplicationsEnd)
    : "";
const installSectionStart = projectNsi.indexOf("\nSection\n");
const payloadStart = projectNsi.indexOf("!insertmacro wails.files", installSectionStart);
const installBeforePayload =
  installSectionStart >= 0 && payloadStart > installSectionStart
    ? projectNsi.slice(installSectionStart, payloadStart)
    : "";

ok(
  closeApplicationsBlock.includes(
    '!insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\\${PRODUCT_EXECUTABLE}"',
  ) &&
    closeApplicationsBlock.includes(
      '!insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\\runtimes\\python\\3.12.10\\${ARCH}\\python.exe"',
    ) &&
    closeApplicationsBlock.includes(
      '!insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\\runtimes\\python\\3.12.10\\${ARCH}\\python312.dll"',
    ) &&
    closeApplicationsBlock.includes(
      '!insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$KwuLegacyInstallDir\\kwu-desktop.exe"',
    ),
  "running-app detection covers the app, managed Python, and authenticated legacy path",
);

ok(
  /RmGetList\([\s\S]*StrCmp \$0 234 detectRunningFound/.test(closeApplicationsBlock),
  "Restart Manager detects a process that is using either trusted executable",
);

ok(
  !closeApplicationsBlock.includes("RmShutdown") &&
    !closeApplicationsBlock.includes("RmForceShutdown") &&
    !installBeforePayload.includes("taskkill.exe"),
  "installer detects occupancy without requesting or forcing application shutdown",
);

ok(
  !/KwuCloseElapsed|GetTickCount|Sleep 500/.test(closeApplicationsBlock),
  "running-app detection does not hide the window or wait through an automatic close attempt",
);

ok(
  /closeApplicationsDetect:[\s\S]*Call KwuDetectRunningInstalledApplications[\s\S]*closeApplicationsRunning:[\s\S]*Call KwuEndRestartManagerSession[\s\S]*IfSilent closeApplicationsRunningSilentAbort[\s\S]*MB_RETRYCANCEL[\s\S]*KWU_CLOSE_RUNNING_PROMPT[\s\S]*IDRETRY closeApplicationsDetect/.test(
    installBeforePayload,
  ),
  "interactive installs require a manual full exit and re-detect on Retry",
);

ok(
  /closeApplicationsRunningSilentAbort:[\s\S]*DetailPrint "\$\{KWU_CLOSE_RUNNING_SILENT_MESSAGE\}"[\s\S]*SetErrorLevel 1[\s\S]*Quit[\s\S]*closeApplicationsDetectionSilentAbort:[\s\S]*DetailPrint "\$\{KWU_CLOSE_DETECTION_FAILURE_MESSAGE\}"[\s\S]*SetErrorLevel 1[\s\S]*Quit/.test(
    installBeforePayload,
  ) &&
    installBeforePayload.indexOf("closeApplicationsDetect:") <
      installBeforePayload.indexOf("!insertmacro wails.webview2runtime"),
  "silent occupancy or detection failures abort before any installed payload can be modified",
);

ok(
  /closeApplicationsRunning:[\s\S]*?MessageBox MB_RETRYCANCEL[^\r\n]*IDRETRY closeApplicationsDetect\s*SetErrorLevel 1\s*Quit/.test(
    installBeforePayload,
  ) &&
    /closeApplicationsFailed:[\s\S]*?MessageBox MB_RETRYCANCEL[^\r\n]*IDRETRY closeApplicationsDetect\s*SetErrorLevel 1\s*Quit/.test(
      installBeforePayload,
    ),
  "Cancel exits the installer immediately instead of returning to a second Cancel action",
);

ok(
  /KWU_CLOSE_RUNNING_PROMPT "[^"]*应用菜单或系统托盘[^"]*退出[^"]*重试/.test(
    installerStrings,
  ) &&
    !/60 秒|选择“是”/.test(installerStrings),
  "running-app copy tells users to exit from the menu or tray without implying automatic closure",
);

const releaseSigningStart = projectNsi.indexOf('!if "$%KWU_WINDOWS_RELEASE_SIGNING%" == "1"');
const releaseSigningEnd = projectNsi.indexOf('\nName "${INFO_PRODUCTNAME}"', releaseSigningStart);
const releaseSigningBlock =
  releaseSigningStart >= 0 && releaseSigningEnd > releaseSigningStart
    ? projectNsi.slice(releaseSigningStart, releaseSigningEnd)
    : "";

ok(
  releaseSigningBlock.includes("desktop-windows-authenticode.ps1") &&
    releaseSigningBlock.indexOf("-Layer application") < projectNsi.indexOf("!insertmacro wails.files"),
  "release-only signing uses the shared helper before NSIS embeds the application",
);

ok(
  /!system '[^']*-Layer application[^']*' = 0/.test(releaseSigningBlock) &&
    /!uninstfinalize '[^']*-Layer uninstaller[^']*' = 0/.test(releaseSigningBlock) &&
    /!finalize '[^']*-Layer installer[^']*' = 0/.test(releaseSigningBlock),
  "application, uninstaller and installer signing hooks all fail closed",
);

ok(
  (releaseSigningBlock.match(/\$\{KWU_SIGN_HELPER\}/g) ?? []).length === 3 &&
    releaseSigningBlock.includes("$%KWU_WINDOWS_SIGN_EVIDENCE_PATH%"),
  "all compiler hooks share one Authenticode implementation and one evidence stream",
);

ok(
  /StrCmp "\$KwuUpdatePID" "" normalPayload updatePayload[\s\S]*updatePayload:[\s\S]*SetOverwrite try[\s\S]*ClearErrors[\s\S]*Goto installPayload[\s\S]*normalPayload:[\s\S]*SetOverwrite on[\s\S]*ClearErrors[\s\S]*installPayload:[\s\S]*!insertmacro wails\.files[\s\S]*IfErrors updateFailed[\s\S]*File \/r "\.\.\\python-runtime\\\$\{ARCH\}\\\*\.\*"[\s\S]*IfErrors updateFailed[\s\S]*publishPythonRuntime:[\s\S]*Rename[\s\S]*IfErrors updateFailed[\s\S]*SetOverwrite on[\s\S]*payloadDone:/.test(
    projectNsi,
  ),
  "normal installs and updates share one payload while updates retain overwrite failure handling",
);

ok(
  /IfFileExists "\$INSTDIR\\\$\{PRODUCT_EXECUTABLE\}" 0 updateFailed[\s\S]*IfFileExists "\$INSTDIR\\uninstall\.exe" 0 updateFailed[\s\S]*IfFileExists "\$INSTDIR\\runtimes\\python\\3\.12\.10\\\$\{ARCH\}\\python\.exe" 0 updateFailed[\s\S]*payloadDone:[\s\S]*IfFileExists "\$SMPROGRAMS\\\$\{INFO_PRODUCTNAME\}\.lnk" 0 postCommitFailed[\s\S]*IfFileExists "\$DESKTOP\\\$\{INFO_PRODUCTNAME\}\.lnk" 0 postCommitFailed[\s\S]*ReadRegStr \$0 HKCU "\$\{UNINST_KEY\}" "InstallLocation"[\s\S]*newInstallCommitted:[\s\S]*Delete "\$KwuLegacyInstallDir\\kwu-desktop\.exe"/.test(
    projectNsi,
  ) && /postCommitFailed:[\s\S]*updateFailed:/.test(projectNsi),
  "new executable, Python runtime, uninstaller, shortcuts and registry commit before legacy cleanup",
);

ok(
  /lstrcmpiW\(w "\$KwuLegacyInstallDir", w "\$INSTDIR"\)[\s\S]*StrCmp \$1 0 legacyRegistryCleanup[\s\S]*Delete "\$KwuLegacyInstallDir\\uninstall\.exe"/.test(
    projectNsi,
  ) &&
    /RMDir "\$KwuLegacyInstallDir"/.test(projectNsi) &&
    !/RMDir \/r "\$KwuLegacyInstallDir"/.test(projectNsi),
  "same-directory migration preserves the new uninstaller and legacy directories are removed only when empty",
);

ok(
  versionInfo.fixed?.file_version === "{{.Info.ProductVersion}}" &&
    versionInfo.fixed?.product_version === "{{.Info.ProductVersion}}",
  "portable executable embeds matching file and product versions",
);

ok(
  versionInfo.info?.["0000"]?.FileVersion === "{{.Info.ProductVersion}}" &&
    versionInfo.info?.["0000"]?.ProductName === "{{.Info.ProductName}}" &&
    versionInfo.info?.["0000"]?.FileDescription === "{{.Info.ProductName}}",
  "portable executable exposes the Unicode product name in Windows properties",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
