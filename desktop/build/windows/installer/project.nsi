Unicode true

####
## KWU per-user NSIS installer.
##
## This file is COMMITTED and customized (Wails leaves an existing project.nsi
## untouched and only regenerates wails_tools.nsh). The customizations vs.
## Wails' default template:
##
##   1. REQUEST_EXECUTION_LEVEL "user" + InstallDir under $LOCALAPPDATA - install
##      without administrator rights. This is what lets the auto-updater re-run a
##      freshly downloaded installer silently (`/S`) with no UAC prompt.
##   2. Uninstall registry under HKCU (not HKLM). Wails' wails.writeUninstaller /
##      wails.deleteUninstaller macros hard-code HKLM, which a non-admin install
##      cannot write - so we inline HKCU versions below instead.
##   3. New Clouvia installs remember their selected directory through the ASCII
##      uninstall identity, so v1.0.3 and later can update silently in place.
##      A recognized pre-v1.0.3 identity instead becomes a one-time interactive
##      migration: .onInit saves the old directory for conservative cleanup,
##      resets the new destination to %LOCALAPPDATA%\Programs\云智企办, and lets
##      the user confirm or Browse to a different location.
##
## Everything else mirrors Wails' generated default. Defines below override the
## ProjectInfo values that wails_tools.nsh would otherwise populate.
####

## Install per-user (no admin). Must be defined BEFORE including wails_tools.nsh,
## which only sets the "admin" default when REQUEST_EXECUTION_LEVEL is undefined.
!define REQUEST_EXECUTION_LEVEL "user"

# Wails' -o flag controls the compiled file but does not update
# ProjectData.OutputFilename before NSIS renders wails_tools.nsh. Define the
# installed payload explicitly so the source binary, installer payload, updater,
# shortcuts and uninstall metadata all agree on the Windows-only runtime name.
!define PRODUCT_EXECUTABLE "clouvia-desktop.exe"

# Use one ASCII uninstall identity so future releases are independent of the
# source-file and registry encoding used for the Unicode display name.
!define UNINST_KEY_NAME "ClouviaCloudOffice"
!define KWU_LEGACY_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\快务AI快务AI"
!define KWU_MOJIBAKE_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\蹇姟AI蹇姟AI"
!define SUIYUAN_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\esengine燧元"
!define REASONIX_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\ReasonixReasonix"

####
## Include the wails tools (auto-generated; provides INFO_* defines and the
## wails.* macros used below).
####
!include "wails_tools.nsh"
!include "FileFunc.nsh"
!include "Win\RestartManager.nsh"
!include /CHARSET=UTF8 "strings_zh.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Run ${INFO_PRODUCTNAME}"
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## Authenticode is release-only. desktop-build.sh performs the first preflight
## and supplies the evidence path; every compiler hook still invokes the same
## helper so identity, timestamp and post-sign verification fail closed.
!if "$%KWU_WINDOWS_RELEASE_SIGNING%" == "1"
    !define KWU_SIGN_HELPER "..\..\..\..\scripts\desktop-windows-authenticode.ps1"
    !ifdef ARG_WAILS_AMD64_BINARY
        !define KWU_WAILS_BINARY "${ARG_WAILS_AMD64_BINARY}"
    !else
        !define KWU_WAILS_BINARY "${ARG_WAILS_ARM64_BINARY}"
    !endif
    !system 'pwsh.exe -NoLogo -NoProfile -File "${KWU_SIGN_HELPER}" -Action Sign -Path "${KWU_WAILS_BINARY}" -Layer application -Architecture "${ARCH}" -EvidencePath "$%KWU_WINDOWS_SIGN_EVIDENCE_PATH%"' = 0
    !uninstfinalize 'pwsh.exe -NoLogo -NoProfile -File "${KWU_SIGN_HELPER}" -Action Sign -Path "%1" -Layer uninstaller -Architecture "${ARCH}" -EvidencePath "$%KWU_WINDOWS_SIGN_EVIDENCE_PATH%"' = 0
    !finalize 'pwsh.exe -NoLogo -NoProfile -File "${KWU_SIGN_HELPER}" -Action Sign -Path "%1" -Layer installer -Architecture "${ARCH}" -EvidencePath "$%KWU_WINDOWS_SIGN_EVIDENCE_PATH%"' = 0
!endif

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!define KWU_DEFAULT_INSTALLDIR "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation" # Reuse the Clouvia install path for v1.0.3+ updates.
InstallDir "${KWU_DEFAULT_INSTALLDIR}" # Per-user install location (no admin rights required).
ShowInstDetails show # This will always show the installation details.

####
## Per-user uninstaller registry (HKCU). Replaces wails.writeUninstaller /
## wails.deleteUninstaller, which write HKLM and would fail without admin rights.
####
!macro kwu.writeUninstallRegistry
    WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
    # Persist the resolved Clouvia path so v1.0.3+ updates can reuse it through
    # InstallDirRegKey and silently reinstall in place.
    WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"

    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"
!macroend

!macro kwu.deleteUninstaller
    Delete "$INSTDIR\uninstall.exe"
    DeleteRegKey HKCU "${UNINST_KEY}"
!macroend

!macro kwu.deleteInstalledExecutable
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 done
    DetailPrint "Stopping ${PRODUCT_EXECUTABLE} before uninstall..."
    nsExec::Exec '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T /F'
    Pop $0
    Sleep 1000
    ClearErrors
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    IfErrors 0 done
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" fail done
fail:
    Abort "Unable to remove $INSTDIR\${PRODUCT_EXECUTABLE}. Close ${INFO_PRODUCTNAME} and run uninstall again."
done:
!macroend

Var KwuUpdatePID
Var KwuLegacyUninstallKey
Var KwuLegacyInstallDir
Var KwuOldShortcutName
Var KwuRestartManagerSession
Var KwuRestartManagerStatus
Var KwuRestartManagerRegistered
Var KwuRecoveryResume
Var KwuBackupRoot
Var KwuBackupMissingDeletes
Var KwuOwnerHandle
Var KwuOwnerStatus
!define KWU_TRANSACTION_OWNER_NAME "Global\ClouviaCloudOfficeInstallerTransactionOwnerV1"

; The machine-wide named event is an atomic process-lifetime claim shared by
; installer and uninstaller instances, including instances in other sessions.
; It is never waited or signaled; the kernel removes it with the final handle.
!macro kwu.defineTransactionOwnerFunctions PREFIX
Function ${PREFIX}KwuAcquireTransactionOwner
    StrCpy $KwuOwnerStatus 0
    StrCmp $KwuOwnerHandle "" ownerAcquireCreate ownerAcquireSucceeded

ownerAcquireCreate:
    System::Call 'kernel32::CreateEventW(p 0, i 1, i 0, w "${KWU_TRANSACTION_OWNER_NAME}") p .r0 ?e'
    Pop $1
    StrCmp $0 0 ownerAcquireDone
    IntCmp $1 183 ownerAcquireAlreadyExists ownerAcquireStore ownerAcquireStore

ownerAcquireAlreadyExists:
    System::Call 'kernel32::CloseHandle(p r0) i .r2'
    Goto ownerAcquireDone

ownerAcquireStore:
    StrCpy $KwuOwnerHandle $0

ownerAcquireSucceeded:
    StrCpy $KwuOwnerStatus 1

ownerAcquireDone:
FunctionEnd

Function ${PREFIX}KwuReleaseTransactionOwner
    StrCpy $KwuOwnerStatus 0
    StrCmp $KwuOwnerHandle "" ownerReleaseDone
    System::Call 'kernel32::CloseHandle(p $KwuOwnerHandle) i .r0'
    StrCmp $0 0 ownerReleaseDone
    StrCpy $KwuOwnerHandle ""
    StrCpy $KwuOwnerStatus 1

ownerReleaseDone:
FunctionEnd
!macroend

; Sets KwuRestartManagerStatus to:
;   0 - no trusted installed application/runtime file is in use
;   1 - at least one trusted installed application/runtime file is in use
;   2 - Restart Manager could not inspect the files
; A session remains open only for status 1 so the caller can close it before
; displaying the manual-exit prompt.
Function KwuDetectRunningInstalledApplications
   Call KwuEndRestartManagerSession
   StrCpy $KwuRestartManagerStatus 2
   StrCpy $KwuRestartManagerRegistered 0

   !insertmacro RestartManager_StartSession $KwuRestartManagerSession
   StrCmp "$KwuRestartManagerSession" "" detectRunningDone

   IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 detectRunningPythonExecutable
   !insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\${PRODUCT_EXECUTABLE}"
   StrCmp $0 0 0 detectRunningFailure
   StrCpy $KwuRestartManagerRegistered 1

detectRunningPythonExecutable:
   IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python.exe" 0 detectRunningPythonDLL
   !insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python.exe"
   StrCmp $0 0 0 detectRunningFailure
   StrCpy $KwuRestartManagerRegistered 1

detectRunningPythonDLL:
   IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python312.dll" 0 detectRunningLegacy
   !insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python312.dll"
   StrCmp $0 0 0 detectRunningFailure
   StrCpy $KwuRestartManagerRegistered 1

detectRunningLegacy:
   ; The legacy path is populated only after a matching uninstall registry
   ; record and uninstaller have authenticated it in .onInit.
   StrCmp "$KwuLegacyInstallDir" "" detectRunningRegistered
   IfFileExists "$KwuLegacyInstallDir\kwu-desktop.exe" 0 detectRunningRegistered
   !insertmacro RestartManager_RegisterFile "$KwuRestartManagerSession" "$KwuLegacyInstallDir\kwu-desktop.exe"
   StrCmp $0 0 0 detectRunningFailure
   StrCpy $KwuRestartManagerRegistered 1

detectRunningRegistered:
   StrCmp $KwuRestartManagerRegistered 0 detectRunningNone
   StrCpy $1 0
   StrCpy $2 0
   System::Call 'RSTRTMGR::RmGetList(i $KwuRestartManagerSession, *i .r1, *i 0, p0, *i .r2) i .r0'
   StrCmp $0 234 detectRunningFound
   StrCmp $0 0 detectRunningNone detectRunningFailure

detectRunningFound:
   StrCpy $KwuRestartManagerStatus 1
   Goto detectRunningDone

detectRunningNone:
   StrCpy $KwuRestartManagerStatus 0

detectRunningFailure:
   Call KwuEndRestartManagerSession

detectRunningDone:
FunctionEnd

Function KwuEndRestartManagerSession
   StrCmp "$KwuRestartManagerSession" "" endRestartManagerDone
   !insertmacro RestartManager_EndSession "$KwuRestartManagerSession"
   StrCpy $KwuRestartManagerSession ""
endRestartManagerDone:
FunctionEnd

!insertmacro kwu.defineTransactionOwnerFunctions ""
!insertmacro kwu.defineTransactionOwnerFunctions "un."

Function .onInit
   !insertmacro wails.checkArchitecture

   ; An in-app update launches this installer before the running application
   ; exits. Wait for that exact process so silent extraction never races the
   ; executable shutdown. A missing handle means the process has already exited.
   ${GetParameters} $R0
   ClearErrors
   ${GetOptions} "$R0" "/UPDATEPID=" $KwuUpdatePID
   IfErrors updateWaitDone
   StrCmp "$KwuUpdatePID" "" updateWaitDone

   System::Call 'kernel32::OpenProcess(i 0x00100000, i 0, i $KwuUpdatePID) p .r1'
   StrCmp $R1 0 updateWaitDone
   System::Call 'kernel32::WaitForSingleObject(p r1, i 60000) i .r2'
   System::Call 'kernel32::CloseHandle(p r1)'
   IntCmp $R2 0 updateWaitDone updateWaitFailed updateWaitFailed

updateWaitFailed:
   SetErrorLevel 1
   MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
   Abort

updateWaitDone:

   ; Old updaters pass their current directory through /D=. Treat it as a
   ; legacy source only when an installed uninstaller exists there and one of
   ; the known old registry records independently points at the same directory.
   ; This excludes unsupported portable updates and arbitrary /D= installs.
   IfFileExists "$INSTDIR\uninstall.exe" 0 unsupportedInstalledUpdate
   Goto checkUpdateKwu

unsupportedInstalledUpdate:
   ; v1.0.2 portable builds also supplied /UPDATEPID, but portable-to-installed
   ; conversion is intentionally outside the supported v1.0.3 migration.
   StrCmp "$KwuUpdatePID" "" done
   SetErrorLevel 1
   MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
   Abort

checkUpdateKwu:
   ReadRegStr $0 HKCU "${KWU_LEGACY_UNINST_KEY}" "InstallLocation"
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyKwuDetected
   ReadRegStr $0 HKCU "${KWU_LEGACY_UNINST_KEY}" "DisplayIcon"
   ${GetParent} "$0" $0
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyKwuDetected checkUpdateMojibake

legacyKwuDetected:
   StrCpy $KwuLegacyUninstallKey "${KWU_LEGACY_UNINST_KEY}"
   StrCpy $KwuOldShortcutName "快务AI"
   Goto legacyDetected

checkUpdateMojibake:
   ReadRegStr $0 HKCU "${KWU_MOJIBAKE_UNINST_KEY}" "InstallLocation"
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyMojibakeDetected
   ReadRegStr $0 HKCU "${KWU_MOJIBAKE_UNINST_KEY}" "DisplayIcon"
   ${GetParent} "$0" $0
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyMojibakeDetected checkUpdateSuiYuan

legacyMojibakeDetected:
   StrCpy $KwuLegacyUninstallKey "${KWU_MOJIBAKE_UNINST_KEY}"
   StrCpy $KwuOldShortcutName "快务AI"
   Goto legacyDetected

checkUpdateSuiYuan:
   ReadRegStr $0 HKCU "${SUIYUAN_UNINST_KEY}" "InstallLocation"
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacySuiYuanDetected
   ReadRegStr $0 HKCU "${SUIYUAN_UNINST_KEY}" "DisplayIcon"
   ${GetParent} "$0" $0
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacySuiYuanDetected checkUpdateReasonix

legacySuiYuanDetected:
   StrCpy $KwuLegacyUninstallKey "${SUIYUAN_UNINST_KEY}"
   StrCpy $KwuOldShortcutName "燧元"
   Goto legacyDetected

checkUpdateReasonix:
   ReadRegStr $0 HKCU "${REASONIX_UNINST_KEY}" "InstallLocation"
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyReasonixDetected
   ReadRegStr $0 HKCU "${REASONIX_UNINST_KEY}" "DisplayIcon"
   ${GetParent} "$0" $0
   System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
   StrCmp $1 0 legacyReasonixDetected done

legacyReasonixDetected:
   StrCpy $KwuLegacyUninstallKey "${REASONIX_UNINST_KEY}"
   StrCpy $KwuOldShortcutName "Reasonix"
   Goto legacyDetected

legacyDetected:
   ; Keep the old directory only as a cleanup source. Brand migration always
   ; returns to the normal visible directory page and its canonical default,
   ; even when v1.0.2 supplied /S and /D= for an in-place update.
   StrCpy $KwuLegacyInstallDir "$INSTDIR"
   StrCpy $INSTDIR "${KWU_DEFAULT_INSTALLDIR}"
   SetSilent normal
done:
FunctionEnd

Section
    !insertmacro wails.setShellContext

    ; Fail fast before WebView setup or any installer-owned path can mutate.
    Call KwuAcquireTransactionOwner
    StrCmp $KwuOwnerStatus 1 transactionOwnerAcquired
    SetErrorLevel 1
    MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
    Abort

transactionOwnerAcquired:

closeApplicationsDetect:
    Call KwuDetectRunningInstalledApplications
    StrCmp $KwuRestartManagerStatus 0 closeApplicationsDone
    StrCmp $KwuRestartManagerStatus 1 closeApplicationsRunning closeApplicationsFailed

closeApplicationsRunning:
    Call KwuEndRestartManagerSession
    IfSilent closeApplicationsRunningSilentAbort
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION|MB_DEFBUTTON1 "${KWU_CLOSE_RUNNING_PROMPT}" IDRETRY closeApplicationsDetect
    SetErrorLevel 1
    Quit

closeApplicationsFailed:
    Call KwuEndRestartManagerSession
    IfSilent closeApplicationsDetectionSilentAbort
    MessageBox MB_RETRYCANCEL|MB_ICONSTOP|MB_DEFBUTTON1 "${KWU_CLOSE_DETECTION_FAILURE_MESSAGE}" IDRETRY closeApplicationsDetect
    SetErrorLevel 1
    Quit

closeApplicationsRunningSilentAbort:
    DetailPrint "${KWU_CLOSE_RUNNING_SILENT_MESSAGE}"
    SetErrorLevel 1
    Quit

closeApplicationsDetectionSilentAbort:
    DetailPrint "${KWU_CLOSE_DETECTION_FAILURE_MESSAGE}"
    SetErrorLevel 1
    Quit

closeApplicationsDone:
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    StrCmp "$KwuUpdatePID" "" normalPayload updatePayload

updatePayload:
    ; Avoid the stock Abort/Retry/Ignore prompt during a silent update. A failed
    ; overwrite is surfaced by our concise updater error below instead.
    SetOverwrite try
    ClearErrors
    Goto installPayload

normalPayload:
    SetOverwrite on
    ClearErrors

installPayload:
    ; A committed directory is a durable commit marker: the new payload is live,
    ; so a cleanup retry must never roll it back.
    IfFileExists "$INSTDIR\.kwu-update-backup.committed" 0 recoverInterruptedActive
    RMDir /r "$INSTDIR\.kwu-update-backup.committed"
    IfFileExists "$INSTDIR\.kwu-update-backup.committed" transactionRecoveryFailed

recoverInterruptedActive:
    StrCpy $KwuRecoveryResume 1
    IfFileExists "$INSTDIR\.kwu-update-backup.active" 0 recoverInterruptedStaging
    IfFileExists "$INSTDIR\.kwu-update-backup.staging" transactionRecoveryFailed
    StrCpy $KwuBackupRoot "$INSTDIR\.kwu-update-backup.active"
    StrCpy $KwuBackupMissingDeletes 1
    Goto restoreBackupApplication

recoverInterruptedStaging:
    IfFileExists "$INSTDIR\.kwu-update-backup.staging" 0 stageApplicationPayload
    StrCpy $KwuBackupRoot "$INSTDIR\.kwu-update-backup.staging"
    StrCpy $KwuBackupMissingDeletes 0
    Goto restoreBackupApplication

stageApplicationPayload:
    ; Stage every version-coupled payload on the install volume before touching
    ; the live application/runtime pair.
    RMDir /r "$INSTDIR\.kwu-update-staging"
    IfFileExists "$INSTDIR\.kwu-update-staging" updateFailed
    SetOutPath "$INSTDIR\.kwu-update-staging"
    ClearErrors
    !insertmacro wails.files
    IfErrors updateFailed
    WriteUninstaller "$INSTDIR\.kwu-update-staging\uninstall.exe"
    IfErrors updateFailed
    IfFileExists "$INSTDIR\.kwu-update-staging\${PRODUCT_EXECUTABLE}" 0 updateFailed
    IfFileExists "$INSTDIR\.kwu-update-staging\uninstall.exe" 0 updateFailed

    RMDir /r "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging"
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging" updateFailed
    SetOutPath "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging"
    ClearErrors
    File /r "..\python-runtime\${ARCH}\*.*"
    IfErrors updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging\python.exe" 0 updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging\LICENSE.txt" 0 updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging\.kwu-runtime.json" 0 updateFailed

    SetOutPath "$INSTDIR"

    ; Collect the complete old payload under one directory. Publishing cannot
    ; begin until the directory itself atomically moves from staging to active.
    ClearErrors
    CreateDirectory "$INSTDIR\.kwu-update-backup.staging"
    IfErrors updateFailed
    ClearErrors
    FileOpen $0 "$INSTDIR\.kwu-update-backup.staging\.kwu-state" w
    IfErrors updateFailed
    FileClose $0

    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 backupUninstaller
    ClearErrors
    Rename "$INSTDIR\${PRODUCT_EXECUTABLE}" "$INSTDIR\.kwu-update-backup.staging\${PRODUCT_EXECUTABLE}"
    IfErrors updateFailed

backupUninstaller:
    IfFileExists "$INSTDIR\uninstall.exe" 0 backupPythonRuntime
    ClearErrors
    Rename "$INSTDIR\uninstall.exe" "$INSTDIR\.kwu-update-backup.staging\uninstall.exe"
    IfErrors updateFailed

backupPythonRuntime:
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}" 0 activatePayloadBackup
    ClearErrors
    Rename "$INSTDIR\runtimes\python\3.12.10\${ARCH}" "$INSTDIR\.kwu-update-backup.staging\python-runtime"
    IfErrors updateFailed

activatePayloadBackup:
    ClearErrors
    Rename "$INSTDIR\.kwu-update-backup.staging" "$INSTDIR\.kwu-update-backup.active"
    IfErrors updateFailed

publishPythonRuntime:
    ClearErrors
    Rename "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging" "$INSTDIR\runtimes\python\3.12.10\${ARCH}"
    IfErrors updateFailed

    ClearErrors
    Rename "$INSTDIR\.kwu-update-staging\uninstall.exe" "$INSTDIR\uninstall.exe"
    IfErrors updateFailed

    ClearErrors
    Rename "$INSTDIR\.kwu-update-staging\${PRODUCT_EXECUTABLE}" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    IfErrors updateFailed
    RMDir "$INSTDIR\.kwu-update-staging"
    IfFileExists "$INSTDIR\.kwu-update-staging" updateFailed

    ; The version-coupled payload transaction is now complete. Validate it before
    ; deleting rollback state; later shortcut/registry failures keep this coherent
    ; new payload instead of mixing versions.
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 updateFailed
    IfFileExists "$INSTDIR\uninstall.exe" 0 updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python.exe" 0 updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\LICENSE.txt" 0 updateFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\.kwu-runtime.json" 0 updateFailed

    ; This directory rename is the commit point. Once committed exists, cleanup
    ; failure is post-commit and a later installer only retries its removal.
    ClearErrors
    Rename "$INSTDIR\.kwu-update-backup.active" "$INSTDIR\.kwu-update-backup.committed"
    IfErrors updateFailed
    RMDir /r "$INSTDIR\.kwu-update-backup.committed"
    IfFileExists "$INSTDIR\.kwu-update-backup.committed" postCommitFailed
    SetOverwrite on

payloadDone:
    ClearErrors
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro kwu.writeUninstallRegistry
    IfErrors postCommitFailed

    ; Treat creation as committed only after every new application artifact and
    ; the ASCII uninstall identity can be observed. Until this point the legacy
    ; installation is untouched.
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 postCommitFailed
    IfFileExists "$INSTDIR\uninstall.exe" 0 postCommitFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python.exe" 0 postCommitFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\LICENSE.txt" 0 postCommitFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\.kwu-runtime.json" 0 postCommitFailed
    IfFileExists "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" 0 postCommitFailed
    IfFileExists "$DESKTOP\${INFO_PRODUCTNAME}.lnk" 0 postCommitFailed
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    System::Call 'kernel32::lstrcmpiW(w "$0", w "$INSTDIR") i .r1'
    StrCmp $1 0 newInstallCommitted postCommitFailed

newInstallCommitted:
    ; Remove only the known v1.0.0-v1.0.2 executable after the new installation
    ; commits. A locked executable leaves the complete old installation and its
    ; registry record intact. User data is outside both directories and is never
    ; deleted.
    RMDir /r "$INSTDIR\runtimes\node"
    RMDir /r "$INSTDIR\runtimes\feishu"
    StrCmp "$KwuLegacyInstallDir" "" legacyCleanupDone
    ClearErrors
    Delete "$KwuLegacyInstallDir\kwu-desktop.exe"
    IfFileExists "$KwuLegacyInstallDir\kwu-desktop.exe" legacyCleanupPreserved

    Delete "$SMPROGRAMS\$KwuOldShortcutName.lnk"
    Delete "$DESKTOP\$KwuOldShortcutName.lnk"

    ; The user may Browse back to the old directory. In that case uninstall.exe
    ; is the newly written Clouvia uninstaller and must not be removed.
    System::Call 'kernel32::lstrcmpiW(w "$KwuLegacyInstallDir", w "$INSTDIR") i .r1'
    StrCmp $1 0 legacyRegistryCleanup
    Delete "$KwuLegacyInstallDir\uninstall.exe"

legacyRegistryCleanup:
    DeleteRegKey HKCU "$KwuLegacyUninstallKey"
    RMDir "$KwuLegacyInstallDir"
    Goto legacyCleanupDone

legacyCleanupPreserved:
    DetailPrint "The previous installation is still in use and was preserved at $KwuLegacyInstallDir."

legacyCleanupDone:

    ; Only the in-app updater relaunches automatically. Ordinary /S installs
    ; preserve standard silent-installer semantics and leave the app closed.
    StrCmp "$KwuUpdatePID" "" sectionDone
    ; Interactive brand migration uses the standard Finish-page launch control.
    ; Future same-identity silent updates continue to relaunch here.
    StrCmp "$KwuLegacyInstallDir" "" 0 sectionDone
    ClearErrors
    Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
    IfErrors postCommitFailed sectionDone

postCommitFailed:
    ; The version-coupled app/runtime/uninstaller payload already committed.
    ; Report metadata or relaunch failures without restoring only part of it.
    SetOverwrite on
    SetErrorLevel 1
    MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
    Abort

updateFailed:
    ; Before the commit marker, recover the complete active backup. A collection
    ; failure only restores components already moved into backup.staging.
    SetOutPath "$INSTDIR"
    StrCpy $KwuRecoveryResume 0
    IfFileExists "$INSTDIR\.kwu-update-backup.active" 0 updateFailedCheckStagingBackup
    IfFileExists "$INSTDIR\.kwu-update-backup.staging" transactionRecoveryFailed
    StrCpy $KwuBackupRoot "$INSTDIR\.kwu-update-backup.active"
    StrCpy $KwuBackupMissingDeletes 1
    Goto restoreBackupApplication

updateFailedCheckStagingBackup:
    IfFileExists "$INSTDIR\.kwu-update-backup.staging" 0 updateFailedCleanupStage
    StrCpy $KwuBackupRoot "$INSTDIR\.kwu-update-backup.staging"
    StrCpy $KwuBackupMissingDeletes 0
    Goto restoreBackupApplication

restoreBackupApplication:
    IfFileExists "$KwuBackupRoot\${PRODUCT_EXECUTABLE}" restoreBackupApplicationPresent restoreBackupApplicationMissing

restoreBackupApplicationPresent:
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" transactionRecoveryFailed
    ClearErrors
    CopyFiles /SILENT "$KwuBackupRoot\${PRODUCT_EXECUTABLE}" "$INSTDIR"
    IfErrors transactionRecoveryFailed
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" restoreBackupUninstaller transactionRecoveryFailed

restoreBackupApplicationMissing:
    StrCmp $KwuBackupMissingDeletes 1 0 restoreBackupUninstaller
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" transactionRecoveryFailed

restoreBackupUninstaller:
    IfFileExists "$KwuBackupRoot\uninstall.exe" restoreBackupUninstallerPresent restoreBackupUninstallerMissing

restoreBackupUninstallerPresent:
    Delete "$INSTDIR\uninstall.exe"
    IfFileExists "$INSTDIR\uninstall.exe" transactionRecoveryFailed
    ClearErrors
    CopyFiles /SILENT "$KwuBackupRoot\uninstall.exe" "$INSTDIR"
    IfErrors transactionRecoveryFailed
    IfFileExists "$INSTDIR\uninstall.exe" restoreBackupPythonRuntime transactionRecoveryFailed

restoreBackupUninstallerMissing:
    StrCmp $KwuBackupMissingDeletes 1 0 restoreBackupPythonRuntime
    Delete "$INSTDIR\uninstall.exe"
    IfFileExists "$INSTDIR\uninstall.exe" transactionRecoveryFailed

restoreBackupPythonRuntime:
    IfFileExists "$KwuBackupRoot\python-runtime" restoreBackupPythonRuntimePresent restoreBackupPythonRuntimeMissing

restoreBackupPythonRuntimePresent:
    RMDir /r "$INSTDIR\runtimes\python\3.12.10\${ARCH}"
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}" transactionRecoveryFailed
    ClearErrors
    CreateDirectory "$INSTDIR\runtimes\python\3.12.10\${ARCH}"
    IfErrors transactionRecoveryFailed
    ClearErrors
    CopyFiles /SILENT "$KwuBackupRoot\python-runtime\*.*" "$INSTDIR\runtimes\python\3.12.10\${ARCH}"
    IfErrors transactionRecoveryFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\python.exe" 0 transactionRecoveryFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\LICENSE.txt" 0 transactionRecoveryFailed
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}\.kwu-runtime.json" restoreBackupState transactionRecoveryFailed

restoreBackupPythonRuntimeMissing:
    StrCmp $KwuBackupMissingDeletes 1 0 restoreBackupState
    RMDir /r "$INSTDIR\runtimes\python\3.12.10\${ARCH}"
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\${ARCH}" transactionRecoveryFailed

restoreBackupState:
    RMDir /r "$KwuBackupRoot"
    IfFileExists "$KwuBackupRoot" transactionRecoveryFailed
    StrCmp $KwuRecoveryResume 0 updateFailedCleanupStage
    StrCmp $KwuBackupMissingDeletes 1 recoverInterruptedStaging stageApplicationPayload

updateFailedCleanupStage:
    RMDir /r "$INSTDIR\.kwu-update-staging"
    IfFileExists "$INSTDIR\.kwu-update-staging" transactionRecoveryFailed
    RMDir /r "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging"
    IfFileExists "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging" transactionRecoveryFailed
    SetOverwrite on
    SetErrorLevel 1
    MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
    Abort

transactionRecoveryFailed:
    ; Stop immediately. Continuing after a recovery or state-cleanup failure
    ; could overwrite the only durable copy of a version-coupled payload.
    SetOverwrite on
    SetErrorLevel 1
    MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
    Abort

sectionDone:
    ; Ownership release is the final shared transaction mutation. Verify that
    ; the published claim still names this exact process instance before retire.
    Call KwuReleaseTransactionOwner
    StrCmp $KwuOwnerStatus 1 transactionOwnerReleased
    SetErrorLevel 1
    MessageBox MB_OK|MB_ICONSTOP "${KWU_UPDATE_FAILURE_MESSAGE}"
    Abort

transactionOwnerReleased:
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    Call un.KwuAcquireTransactionOwner
    StrCmp $KwuOwnerStatus 1 uninstallTransactionOwnerAcquired
    SetErrorLevel 1
    Abort "Another installer or uninstaller owns $INSTDIR, or its owner claim cannot be verified."

uninstallTransactionOwnerAcquired:

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    ; Precision uninstall: delete main application files
    !insertmacro kwu.deleteInstalledExecutable

    ; Remove only installer-owned transaction state. These directories can
    ; remain after a fail-closed recovery or committed-backup cleanup failure.
    RMDir /r "$INSTDIR\.kwu-update-staging"
    RMDir /r "$INSTDIR\.kwu-update-backup.staging"
    RMDir /r "$INSTDIR\.kwu-update-backup.active"
    RMDir /r "$INSTDIR\.kwu-update-backup.committed"
    RMDir /r "$INSTDIR\runtimes\python\3.12.10\.${ARCH}.staging"

    RMDir /r "$INSTDIR\runtimes\node"
    RMDir /r "$INSTDIR\runtimes\feishu"
    RMDir /r "$INSTDIR\runtimes\python"

    IfFileExists "$INSTDIR\.kwu-update-staging" uninstallTransactionCleanupFailed
    IfFileExists "$INSTDIR\.kwu-update-backup.staging" uninstallTransactionCleanupFailed
    IfFileExists "$INSTDIR\.kwu-update-backup.active" uninstallTransactionCleanupFailed
    IfFileExists "$INSTDIR\.kwu-update-backup.committed" uninstallTransactionCleanupFailed
    IfFileExists "$INSTDIR\runtimes\python" uninstallTransactionCleanupFailed
    Goto uninstallTransactionCleanupDone

uninstallTransactionCleanupFailed:
    Abort "Unable to remove installer transaction state under $INSTDIR. Close ${INFO_PRODUCTNAME} and run uninstall again."

uninstallTransactionCleanupDone:

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro kwu.deleteUninstaller

    ; Only remove the installation directory if it is empty to prevent data loss
    RMDir $INSTDIR

    ; The product-scoped claim is outside $INSTDIR, so it survives removal of
    ; an empty install root and remains the final shared transaction mutation.
    Call un.KwuReleaseTransactionOwner
    StrCmp $KwuOwnerStatus 1 uninstallTransactionOwnerReleased
    SetErrorLevel 1
    Abort "Unable to verify and release installer transaction ownership for $INSTDIR."

uninstallTransactionOwnerReleased:
SectionEnd
