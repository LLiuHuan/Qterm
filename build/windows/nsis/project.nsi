Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
## 
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "qterm" # Default "qterm"
## !define INFO_COMPANYNAME    "Qterm" # Default "Qterm"
## !define INFO_PRODUCTNAME    "Qterm" # Default "Qterm"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) 2026, Qterm" # Default "(c) 2026, Qterm"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
####
## Include the wails tools
####
!include "wails_tools.nsh"

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
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

## 跨安装范围升级的清理。
##
## v0.0.8 及更早的安装包是机器级（Program Files + HKLM 卸载项），从本版起改成按用户安装
## （%LOCALAPPDATA%\Programs + HKCU 卸载项）。两套安装互不覆盖：直接装新版会同时存在
## 两份程序、两个卸载项、两个快捷方式，用户点哪个都可能不是刚更新的那个。
##
## 旧安装目录在 Program Files 下，删除需要管理员权限，而按用户安装的安装包以普通权限运行，
## 所以这里用 ExecShell "runas" 拉起旧版的卸载程序（会弹 UAC）。静默安装时直接执行，
## 交互安装时先问一句——用户可能刻意想保留旧版本。
##
## 注意这里**不用**注册表里的 UninstallString：它的值本身带引号（`"$INSTDIR\uninstall.exe"`），
## 再拼进命令行会变成双重引号而启动失败。旧版安装路径是固定公式，直接拼路径更可靠。
Function RemoveLegacyMachineInstall
    !if "${WAILS_INSTALL_SCOPE}" == "user"
    SetRegView 64
    ReadRegStr $0 HKLM "${UNINST_KEY}" "UninstallString"
    StrCmp $0 "" legacy_done
    StrCpy $1 "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\uninstall.exe"
    IfFileExists "$1" 0 legacy_done
    DetailPrint "检测到旧版机器级安装：$1"
    IfSilent legacy_exec
    MessageBox MB_OKCANCEL|MB_ICONQUESTION "检测到旧版本以机器级方式安装（Program Files）。\n\n如果不先卸载它，会同时存在两份程序。现在卸载旧版本吗？" IDOK legacy_exec
    Goto legacy_done
    legacy_exec:
    ExecShell "runas" "$1" "/S"
    ; ExecShell 是异步的、也不会告诉我们 UAC 是否被取消，所以只能轮询卸载项是否消失：
    ; 每 500ms 看一次，最多等 20 秒（旧卸载程序要删 Program Files 下的目录，慢盘上需要时间）。
    StrCpy $2 0
    legacy_wait:
    ReadRegStr $3 HKLM "${UNINST_KEY}" "UninstallString"
    StrCmp $3 "" legacy_done
    IntOp $2 $2 + 1
    IntCmp $2 40 legacy_timeout legacy_sleep legacy_sleep
    legacy_sleep:
    Sleep 500
    Goto legacy_wait
    legacy_timeout:
    IfSilent legacy_done
    MessageBox MB_OK|MB_ICONEXCLAMATION "旧版本没有卸载完成（可能取消了 UAC 授权，或卸载失败）。\n\n新版会继续安装，但系统里可能同时存在两份程序，请稍后手动卸载旧版本。"
    legacy_done:
    !endif
FunctionEnd

Section
    Call RemoveLegacyMachineInstall

    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    
    !insertmacro wails.files

    ; RDP helper（可选）：静态链接的单文件，构建过才会被拷到本目录。
    ; /nonfatal 让文件缺失时安装继续，应用侧会提示"未安装 RDP helper"。
    File /nonfatal "qterm-freerdp-helper.exe"

    ; 命令行客户端（可选）：与主程序同目录，Go 侧按"可执行文件同目录 qterm-cli.exe"
    ; 查找。缺失时同样跳过 —— 用户仍可在设置页里看到二进制缺失的提示。
    File /nonfatal "qterm-cli.exe"

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    
    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall" 
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
